// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package pypi implements pip's dependency resolution algorithm.

It is largely translated from the resolvelib library, which is vendored by pip,
and can be found here:
https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/

Throughout, pip's Candidate class is replaced with resolve.Version and pip's
Requirement class with resolve.Dependency. Where feasible a resolve.Package is
used where pip would use the package name as a string.
*/
package pypi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/pypi/internal/lru"
	"deps.dev/util/semver"
)

const debug = false

// debugf prints a message if debug is true. If any of the arguments are
// resolve.Package or resolve.Version they are expanded to PackageKeys or
// VersionKeys.
func debugf(rc resolve.Client, pattern string, args ...any) {
	if !debug {
		return
	}
	for i, arg := range args {
		switch v := arg.(type) {
		case resolve.Version:
			args[i] = v.VersionKey
		case resolve.PackageKey:
			args[i] = v.Name
		case []resolve.Version:
			vks := make([]resolve.VersionKey, len(v))
			for i, v := range v {
				vks[i] = v.VersionKey
			}
			args[i] = vks
		case []resolve.RequirementVersion:
			items := make([]string, len(v))
			for i, d := range v {
				items[i] = fmt.Sprintf("(%v %v)", d.VersionKey, d.Type)
			}
			args[i] = items
		}
	}
	fmt.Printf(pattern, args...)
}

type resolver struct {
	client               resolve.Client
	markerCache          *lru.Cache[string, marker]
	constraintCache      *lru.Cache[resolve.VersionKey, *semver.Constraint]
	prereleaseMatchCache *lru.Cache[resolve.VersionKey, []resolve.Version]
}

func NewResolver(rc resolve.Client) resolve.Resolver {
	return &resolver{
		client:               rc,
		markerCache:          lru.New[string, marker](10000),
		constraintCache:      lru.New[resolve.VersionKey, *semver.Constraint](10000),
		prereleaseMatchCache: lru.New[resolve.VersionKey, []resolve.Version](10000),
	}
}

func (r *resolver) Close() error {
	return nil
}

func (r *resolver) Resolve(ctx context.Context, vk resolve.VersionKey) (*resolve.Graph, error) {
	t0 := time.Now()

	if vt := vk.VersionType; vt != resolve.Concrete {
		return nil, fmt.Errorf("version type %v is not %v", vt, resolve.Concrete)
	}

	p := &provider{
		rc:                   r.client,
		markerCache:          r.markerCache,
		constraintCache:      r.constraintCache,
		prereleaseMatchCache: r.prereleaseMatchCache,
		rootPackage:          vk.PackageKey,
		rootVersion:          vk,
	}

	// The root of a pip resolution is just the current working directory
	// and it expects the resolution to begin with the list of direct
	// dependencies. The first step for us is therefore to collect them.
	// TODO: allow for some extras (such as test and doc)
	v := vk
	deps, err := p.getDependencies(ctx, v, nil)
	if err != nil {
		return nil, err
	}
	// Store the order of the direct dependencies in the provider. In pip
	// this is called "user_requested" and is built from the commnand line
	// arguments or requirements.txt file.
	direct := make(map[resolve.PackageKey]int, len(deps))
	for i, d := range deps {
		direct[p.identify(d.VersionKey)] = i
	}
	p.userRequested = direct

	// Pip stops after 200k iterations, so we will limit ourselves in the same
	// way.
	// https://github.com/pypa/pip/blob/main/src/pip/_internal/resolution/resolvelib/resolver.py#L95
	const maxRounds = 200000
	res := resolution{p: p}
	state, err := res.resolve(ctx, deps, maxRounds)
	if err != nil {
		var riErr resolutionImpossibleError
		if errors.Is(err, errTooDeep) || errors.As(err, &riErr) {
			return &resolve.Graph{Error: err.Error()}, nil
		}
		return nil, err
	}

	g, err := buildGraph(r.client, vk, state)
	if err != nil {
		return nil, err
	}
	g.Duration = time.Since(t0)
	return g, nil
}

// buildGraph serves the same purpose as _build_result in pip: filtering any
// disconnected versions returned by the resolver and building a graph. The
// difference is that our graph is a lot more detailed as it is at the version
// level and contains dependency types and requirements, which leads to quite a
// different implementation. Filtering out disconnected components is quite
// important as the resolver does not always do a perfect job cleaning up
// dependencies when it is forced to downgrade versions.
func buildGraph(rc resolve.Client, root resolve.VersionKey, s *state) (*resolve.Graph, error) {
	connected := make(map[resolve.VersionKey]bool, s.mapping.Len())
	connected[root] = true

	g := &resolve.Graph{}
	rootPackage := root.PackageKey
	ids := map[resolve.PackageKey]resolve.NodeID{
		rootPackage: g.AddNode(root),
	}

	// Add all the nodes that can reach the root.
	s.mapping.Iterate(func(p resolve.PackageKey, v resolve.VersionKey) {
		if !hasRouteToRoot(rc, v, connected, s) {
			return
		}
		if _, ok := ids[p]; !ok {
			// If this is the root package showing up again due to a
			// loop, don't add a duplicate node.
			ids[p] = g.AddNode(v)
		}
	})

	// Add edges.
	for p, to := range ids {
		crit, ok := s.criteria.Get(p)
		if !ok {
			if p == rootPackage {
				// This means the root never had a criteria
				// created for it, which just means there is no
				// loop involving it and there are therefore no
				// edges to add from a dependency to the root.
				continue
			}
			// This would mean there was a package in mapping that
			// was not in criteria, which should never happen.
			return nil, fmt.Errorf("graph for %v: unexpected package %v", root, p)
		}
		for i, req := range crit.informationReqs {
			parent := crit.informationParents[i]
			var from resolve.NodeID
			if parent == (resolve.VersionKey{}) {
				from = ids[rootPackage]
			} else {
				f, ok := ids[parent.PackageKey]
				if !ok {
					// This means the parent is not connected to the
					// root for some reason. Skip it.
					continue
				}
				from = f
			}
			rvk := req.VersionKey
			if err := g.AddEdge(from, to, rvk.Version, req.Type); err != nil {
				return nil, err
			}
		}
	}

	return g, nil
}

func hasRouteToRoot(rc resolve.Client, v resolve.VersionKey, connected map[resolve.VersionKey]bool, s *state) bool {
	if c, ok := connected[v]; c {
		return true
	} else if ok {
		// It's been visited but not yet found to be connected, either
		// because it isn't, or we've just recursed back to the start of
		// a loop. In any case there's no additional paths to the route
		// through here.
		return false
	}
	// Insert a false for now, to mark this version as visited.
	connected[v] = false

	p := v.PackageKey
	crit, ok := s.criteria.Get(p)
	if !ok {
		// This should never happen, but if it does the version is
		// certainly not connected to the root.
		return false
	}
	for _, parent := range crit.informationParents {
		if connected[parent] {
			connected[v] = true
			return true
		}
		parentPackage := parent.PackageKey
		if pv, ok := s.mapping.Get(parentPackage); !ok || pv != parent {
			// The parent was never pinned or a different version
			// was pinned. Either way, there is definitely no path
			// to the root through here.
			continue
		}
		if hasRouteToRoot(rc, parent, connected, s) {
			connected[v] = true
			return true
		}
	}
	return false
}

// provider is a wrapper around the resolve client, giving it an API that
// matches the resolvelib.Provider abstract class in Python
// (https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/providers.py
// with a concrete implementation at
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_internal/resolution/resolvelib/provider.py).
// It fetches dependencies and does semver matching which makes it a fairly thin
// wrapper over the resolve.Client.
type provider struct {
	rc resolve.Client
	// userRequested holds the order of the direct dependencies, used when
	// prioritizing them.
	userRequested map[resolve.PackageKey]int
	// markerCache caches parsed environment markers.
	markerCache *lru.Cache[string, marker]
	// constraintCache caches parsed semver constraints.
	constraintCache *lru.Cache[resolve.VersionKey, *semver.Constraint]
	// prereleaseMatchCache caches matching versions that include
	// prereleases.
	prereleaseMatchCache *lru.Cache[resolve.VersionKey, []resolve.Version]

	rootVersion resolve.VersionKey
	rootPackage resolve.PackageKey
}

// identify provides a package level identifier used, for example, to see if
// two requirements conflict. In Python it is the package name as a string, we
// can use a resolve.Package.
func (p *provider) identify(v resolve.VersionKey) resolve.PackageKey {
	return v.PackageKey
}

// getPreference produces an orderable key used to pick the next criterion to
// attempt to address. As such, matching pip's behavior exactly is likely to be
// important to resolving correctly. The pip equivalent can be found at
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_internal/resolution/resolvelib/provider.py#L67
// and returns a tuple (delay_this, rating, order, identifier). delay_this is
// true iff the package is setuptools, rating is a number used to favor more
// explicit requirements, order is essentially whether the package is a direct
// dependency or not and name is just the package name, to break ties
// lexicographically.
// In Python the arguments are slightly massaged to contain the candidates
// and the requirements separately, here we don't bother to avoid creating
// large numbers of temporary objects.
// TODO: this is the only place we leave ID space. We should at
// least cache these keys, if not re-design this whole mechanism to avoid the
// need for all of these short-lived objects.
func (p *provider) getPreference(identifier resolve.PackageKey, mapping *versionMap, crits *criteria) preferenceKey {
	key := preferenceKey{
		name:              identifier.Name,
		restrictiveRating: 3,
	}
	// the "restrictive rating" is:
	// 0 if there is only one possible candidate. It is not clear exactly
	// when this happens, but it is probably the case when the requirement
	// specifies an exact URL or path. Right now we do not handle any of
	// these cases, so this is not possible.
	// 1 if any requirement is a version constraint containing the == or
	// === operators
	// 2 if any of the requirements involve any other version constraint.
	// 3 otherwise.
	crit, _ := crits.Get(identifier)
	for _, req := range crit.informationReqs {
		// We need to see the constraints, so we have to fetch the
		// version keys.
		vk := req.VersionKey
		if strings.Contains(vk.Version, "==") {
			key.restrictiveRating = 1
			break
		}
		if vk.Version != "" {
			key.restrictiveRating = 2
			break
		}
	}
	// Order is set if the package is "user-requested", otherwise it is set
	// to infinity (we will just use the maximum int for the same effect). A
	// user-requested package is one that is specified on the command line,
	// which for our purposes is a direct dependency.  The actual value is
	// the position of the dependency in the list of direct dependencies.
	key.order = math.MaxInt32
	if ur, ok := p.userRequested[identifier]; ok {
		key.order = ur
	}

	// delayThis is set if the package is called setuptools. This is a hack
	// because setuptools has very many versions and it is uncommon to
	// request a specific range. See
	// https://github.com/pypa/pip/blob/21.1.3/src/pip/_internal/resolution/resolvelib/provider.py#L124
	// for details.
	key.delayThis = strings.ToLower(key.name) == "setuptools"
	return key
}

// preferenceKey is a sort key for a criterion.
type preferenceKey struct {
	delayThis         bool
	restrictiveRating int
	order             int
	name              string
}

// Less compares preferenceKeys: first by delayThis (sorting false ahead of
// true), then by restrictiveRating, order and finally by name.
func (pk1 preferenceKey) Less(pk2 preferenceKey) bool {
	if pk1.delayThis != pk2.delayThis {
		return !pk1.delayThis
	}
	if pk1.restrictiveRating != pk2.restrictiveRating {
		return pk1.restrictiveRating < pk2.restrictiveRating
	}
	if pk1.order != pk2.order {
		return pk1.order < pk2.order
	}
	return pk1.name < pk2.name
}

// findMatches finds all versions that match the logical AND of the given
// requirements. They are expected to be returned in descending order of
// preference. In general for pip this appears to be semver order, although it
// can be changed with various flags. In pip the actual implementation is part
// of the Factory class, found here:
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_internal/resolution/resolvelib/factory.py#L347
// The signature of this function in Python takes the package name, a mapping
// of package names to lists of requirements and a mapping of package names to
// lists of incompatibilities. It only ever looks up the provided package name
// in these mappings, so to provide the flexibility the callers need we
// just take the lists.
// TODO: needs to match pip exactly to be accurate.
func (p *provider) findMatches(ctx context.Context, name resolve.PackageKey, reqs []resolve.RequirementVersion, incompatibilities map[resolve.VersionKey]bool) ([]resolve.VersionKey, error) {
	var matches []resolve.VersionKey
	if len(reqs) == 0 {
		return matches, nil
	}
	// Determine if we have to match with pre-releases or not.
	var (
		getMatches = p.matchingVersions
		anyPre     = false
	)
	// If there's only one requirement, the normal graph matching will
	// already include pre-releases if necessary.
	if len(reqs) > 1 {
		for _, req := range reqs {
			anyPre = anyPre || p.matchesPrerelease(req.VersionKey)
		}
	}
	if anyPre {
		debugf(p.rc, "using pre-release matching: %v\n", reqs)
		getMatches = p.matchingVersionsWithPrereleases
	}

	req := reqs[0]
	mvs, err := getMatches(ctx, req.VersionKey)
	if err != nil {
		return nil, err
	}
	// Copy the first set of matches to avoid any side effects if the client
	// has any internal caches. Filter out the incompatibilities at the same
	// time.
	for _, mv := range mvs {
		if !incompatibilities[mv] {
			matches = append(matches, mv)
		}
	}
	if len(matches) == 0 {
		return nil, requirementsConflictedError{
			p:            p,
			noCandidates: true,
			name:         req.PackageKey,
			reqs:         []resolve.RequirementVersion{req},
		}
	}
	for _, req := range reqs[1:] {
		mvs, err := getMatches(ctx, req.VersionKey)
		if err != nil {
			return nil, err
		}
		matches = intersect(matches, mvs)
	}
	return matches, nil
}

// matchingVersions wraps p.rc.MatchingVersions, unless the provided requirement
// is on the root package. For the root package, the root version is already
// assumed to be installed, so there can only be at most one possible match.
func (p *provider) matchingVersions(ctx context.Context, req resolve.VersionKey) ([]resolve.VersionKey, error) {
	mvs, err := p.rc.MatchingVersions(ctx, req)
	if err != nil {
		return nil, err
	}

	if req.PackageKey != p.rootPackage {
		return getVersionKeys(mvs), nil
	}
	for _, mv := range mvs {
		if mv.VersionKey == p.rootVersion {
			return []resolve.VersionKey{p.rootVersion}, nil
		}
	}
	return nil, nil
}

// matchingVersionsWithPrereleases is similar to matchingVersions except that
// it always returns all pre-release matches
// TODO: cache this, it's slow
func (p *provider) matchingVersionsWithPrereleases(ctx context.Context, req resolve.VersionKey) ([]resolve.VersionKey, error) {
	if p.matchesPrerelease(req) {
		// If the requirement by itself matches pre-releases, the
		// existing matching versions are fine (and cached by the
		// client).
		return p.matchingVersions(ctx, req)
	}

	mvs, ok := p.prereleaseMatchCache.Get(req)
	if ok {
		return getVersionKeys(mvs), nil
	}

	vs, err := p.rc.Versions(ctx, req.PackageKey)
	if err != nil {
		return nil, err
	}
	constraint, err := p.getConstraint(req)
	if err != nil {
		return nil, nil
	}

	debugf(p.rc, "filtering %v by %v\n", vs, constraint)

	mvs, err = filterSlice(vs, func(v resolve.Version) (bool, error) {
		if v.VersionType != resolve.Concrete {
			return false, nil
		}
		ver, err := semver.PyPI.Parse(v.Version)
		if err != nil {
			return false, nil
		}
		return constraint.MatchVersionPrerelease(ver), nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(mvs, func(i, j int) bool {
		iv, _ := semver.PyPI.Parse(mvs[i].Version)
		jv, _ := semver.PyPI.Parse(mvs[j].Version)
		if iv == nil || jv == nil {
			return mvs[i].Version < mvs[j].Version
		}
		return iv.Compare(jv) < 0
	})
	debugf(p.rc, "got %v\n", mvs)
	mvs = slices.Clone(mvs)
	p.prereleaseMatchCache.Add(req, mvs)
	return getVersionKeys(mvs), nil
}

func getVersionKeys(vs []resolve.Version) (vks []resolve.VersionKey) {
	for _, v := range vs {
		vks = append(vks, v.VersionKey)
	}
	return
}

// matchesPrerelease returns whether a requirement allows pre-release matches.
func (p *provider) matchesPrerelease(req resolve.VersionKey) bool {
	c, err := p.getConstraint(req)
	if err != nil {
		return false
	}
	return c.HasPrerelease()
}

func (p *provider) getConstraint(v resolve.VersionKey) (*semver.Constraint, error) {
	c, ok := p.constraintCache.Get(v)
	if ok {
		return c, nil
	}
	c, err := semver.PyPI.ParseConstraint(v.Version)
	if err != nil {
		return nil, err
	}
	p.constraintCache.Add(v, c)
	return c, nil
}

// intersect returns only the elements in both a and b. It may mutate a.
func intersect(a, b []resolve.VersionKey) []resolve.VersionKey {
	// TODO: O(n^2) and called a lot, do better.
	w := 0
	for _, av := range a {
		found := false
		for i, bv := range b {
			if av == bv {
				// a and b are sorted and should have no
				// duplicates, so we know that nothing left in a
				// can match anything below this point in b.
				b = b[i+1:]
				found = true
				break
			}
		}
		if found {
			a[w] = av
			w++
		}
	}
	return a[:w]
}

// filterSlice returns a slice containing all the elements in ts that match the
// provided predicate. The returned slice uses the same backing array, items
// that do not match are swapped to the end.
func filterSlice[T any](ts []T, pred func(T) (bool, error)) ([]T, error) {
	end := len(ts)
	for i := 0; i < end; {
		ok, err := pred(ts[i])
		if err != nil {
			return nil, err
		}
		if ok {
			i++
			continue
		}
		end--
		ts[i], ts[end] = ts[end], ts[i]
	}
	return ts[:end], nil
}

// getDependencies fetches the dependencies for a provided concrete version.
func (p *provider) getDependencies(ctx context.Context, v resolve.VersionKey, extras map[string]bool) ([]resolve.RequirementVersion, error) {
	deps, err := p.rc.Requirements(ctx, v)
	if err != nil {
		return nil, err
	}
	// Filter according to any environment markers. In pip this happens
	// earlier (and several layers further away from the resolver), see
	// https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/pkg_resources/__init__.py#L3026
	// for details.
	return filterSlice(deps, func(d resolve.RequirementVersion) (bool, error) {
		env, ok := d.Type.GetAttr(dep.Environment)
		if !ok {
			// No environment markers, always keep.
			return true, nil
		}
		// TODO(pfcm): cache evaluation results too?
		m, err := p.parseMarker(env)
		if err != nil {
			return false, err
		}
		return m.Eval(extras), nil
	})
}

func (p *provider) parseMarker(raw string) (marker, error) {
	cached, ok := p.markerCache.Get(raw)
	if ok {
		return cached, nil
	}
	m, err := parseMarker(raw)
	if err != nil {
		return nil, err
	}
	p.markerCache.Add(raw, m)
	return m, nil
}

// resolution manages the actual resolution. It maps directly to the
// resolvelib.Resolution object in pip, found here:
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/resolvers.py
type resolution struct {
	// states is a stack of states, with the current state at the end.
	states []*state
	// p is the provider that interacts with the graph.
	p *provider
}

// state gets the most recent state.
func (r *resolution) state() *state {
	if len(r.states) == 0 {
		return nil
	}
	return r.states[len(r.states)-1]
}

// pushNewState adds a new state to the stack which is a copy of the previous
// state. It will panic if there are no states.
func (r *resolution) pushNewState() {
	base := r.state()
	s := &state{
		mapping:  base.mapping.Clone(),
		criteria: base.criteria.Copy(),
	}

	r.states = append(r.states, s)
}

// mergeIntoCriterion attempts to insert the provided requirement into our
// collection of criteria. If we have never seen a requirement for this package
// then create a new criterion, otherwise try and merge the new requirement with
// those we have seen so far.
func (r *resolution) mergeIntoCriterion(ctx context.Context, req resolve.RequirementVersion, parent resolve.VersionKey) (resolve.PackageKey, criterion, error) {
	name := r.p.identify(req.VersionKey)
	crit, _ := r.state().criteria.Get(name)
	// Check that we haven't already added this exact req/parent pair.
	for i := range crit.informationReqs {
		oldReq := crit.informationReqs[i]
		if oldReq.Version == req.Version && oldReq.Type.Equal(req.Type) {
			if crit.informationParents[i] == parent {
				return name, crit, nil
			}
		}
	}
	reqs := append(crit.informationReqs, req)

	matches, err := r.p.findMatches(ctx, name, reqs, crit.incompatibilities)
	if err != nil {
		return resolve.PackageKey{}, criterion{}, err
	}
	if len(matches) == 0 {
		return resolve.PackageKey{}, criterion{}, requirementsConflicted(r.p, name, reqs)
	}
	// Make a copy, with the new set of matches and the new requirement.
	newCrit := crit.copy()
	newCrit.candidates = matches
	newCrit.informationReqs = reqs
	newCrit.informationParents = append(newCrit.informationParents, parent)
	newCrit.extras = unionExtras(crit.extras, req.Type)
	return name, newCrit, nil
}

// isCurrentPinSatisfying checks whether the provided criterion is satisfied by
// the current state.
func (r *resolution) isCurrentPinSatisfying(ctx context.Context, name resolve.PackageKey, crit criterion) bool {
	currentPin, ok := r.state().mapping.Get(name)
	if !ok {
		return false
	}
	// Pip checks every requirement explicitly against the current pin.
	// This is slow for us, because we have to fetch matching versions from
	// the graph and scan through them. Instead, as long as the criterion's
	// candidates are correct, it is sufficient to just check the current
	// pin is listed as a candidate.
	for _, c := range crit.candidates {
		if c == currentPin {
			return true
		}
	}
	return false
}

// getCriteriaToUpdate gathers criteria for the dependencies of the provided
// version. If the dependency is on a package we've never seen before a new
// criterion is created. Otherwise, we have to merge the new requirement into an
// existing criterion. In both cases the current state remains unchanged.
func (r *resolution) getCriteriaToUpdate(ctx context.Context, candidate resolve.VersionKey, extras map[string]bool) (map[resolve.PackageKey]criterion, error) {
	criteria := make(map[resolve.PackageKey]criterion)
	deps, err := r.p.getDependencies(ctx, candidate, extras)
	if err != nil {
		return nil, err
	}
	for _, d := range deps {
		debugf(r.p.rc, "\t->%v\n", d.Version)
		// Note that if there are multiple dependencies on the same
		// package, only the last one will make it into criteria. This
		// seems like a bug in pip: there was a test that was affected by it (in
		// https://github.com/pypa/pip/blob/20.3/tests/yaml/conflict_1.yml)
		// which was the only error test that didn't require exit code 1
		// and checked for a warning generated outside the resolver.
		// TODO: keep an eye on this, it may be fixed upstream.
		name, crit, err := r.mergeIntoCriterion(ctx, d, candidate)
		if err != nil {
			return nil, err
		}
		criteria[name] = crit
	}
	return criteria, nil
}

// getPreference provides a comparison key for a package, deferring
// to the provider's getPreference method.
func (r *resolution) getPreference(name resolve.PackageKey) preferenceKey {
	return r.p.getPreference(name, r.state().mapping, r.state().criteria)
}

// attemptToPinCriterion tries to find one of the provided criterion's candidate
// versions that works.
func (r *resolution) attemptToPinCriterion(ctx context.Context, name resolve.PackageKey) ([]requirementsConflictedError, error) {
	crit, _ := r.state().criteria.Get(name)
	var causes []requirementsConflictedError
	debugf(r.p.rc, "pinning %v (%d candidates, %d bad):\n", name, len(crit.candidates), len(crit.incompatibilities))
	// Try the candidates in descending order. They are stored in ascending
	// order, so iterate them backwards.
	for i := len(crit.candidates) - 1; i >= 0; i-- {
		candidate := crit.candidates[i]
		debugf(r.p.rc, "\t%v\n", candidate.Version)
		criteria, err := r.getCriteriaToUpdate(ctx, candidate, crit.extras)
		if err != nil {
			var rce requirementsConflictedError
			if errors.As(err, &rce) {
				debugf(r.p.rc, " doesn't work: %v\n", rce)
				// This candidate's dependencies cause a
				// conflict with some other requirements. Try
				// the next one.
				causes = append(causes, rce)
				continue
			}
			return nil, err
		}
		debugf(r.p.rc, " works\n")
		// This candidate should work. Python has a redundant check here
		// in case the provider is inconsistent, which is sensible
		// because their provider may make network calls here. In many
		// cases our resolve.Client does not, and we trust it anyway.
		s := r.state()
		// In Python they have to be careful to ensure this pair will be
		// the next popped off mapping. We do not, because our
		// versionMap updates the insertion order for every Set call.
		s.mapping.Set(name, candidate)
		// Add criteria for the dependencies.
		for n, c := range criteria {
			s.criteria.Put(n, c)
		}
		debugf(r.p.rc, "--------------------------------\n")
		return nil, nil
	}
	return causes, nil
}

// backtrack winds back the stack of states to attempt to find a point where it
// is possible to try something new. The doc comment at
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/resolvers.py#L242
// contains a reasonable description of how this behaves.
func (r *resolution) backtrack(ctx context.Context) (bool, error) {
	debugf(r.p.rc, "backtracking--------------------------")
	for len(r.states) >= 3 {
		// Always remove the state that triggered backtracking.
		r.states = r.states[:len(r.states)-1]
		// Now the state at the top has a pin which we know caused us
		// problems.
		broken := r.state()
		// We will re-create it without the problematic pin, so remove
		// it from the stack.
		r.states = r.states[:len(r.states)-1]
		// There must always be a candidate to pop, because the only
		// state that has no pins is the first on the stack and broken
		// must be the second or higher by the loop condition.
		name, candidate := broken.mapping.Pop()
		debugf(r.p.rc, "%v didn't work\n", candidate)

		debugf(r.p.rc, "current pins:\n")
		if debug {
			r.state().mapping.Iterate(func(_ resolve.PackageKey, v resolve.VersionKey) {
				debugf(r.p.rc, "\t%v\n", v)
			})
		}

		type incompatibility struct {
			name              resolve.PackageKey
			incompatibilities map[resolve.VersionKey]bool
		}
		var incompatibilitiesFromBroken []incompatibility
		for _, c := range *broken.criteria {
			incompatibilitiesFromBroken = append(incompatibilitiesFromBroken, incompatibility{
				name:              c.name,
				incompatibilities: c.crit.incompatibilities,
			})
		}
		// Add the newly discovered incompatibility.
		incompatibilitiesFromBroken = append(incompatibilitiesFromBroken, incompatibility{
			name:              name,
			incompatibilities: map[resolve.VersionKey]bool{candidate: true},
		})

		patchCriteria := func() (bool, error) {
			for _, inc := range incompatibilitiesFromBroken {
				if len(inc.incompatibilities) == 0 {
					continue
				}
				crit, ok := r.state().criteria.Get(inc.name)
				if !ok {
					continue
				}
				allIncompats := make(map[resolve.VersionKey]bool, len(inc.incompatibilities)+len(crit.incompatibilities))
				for inc := range inc.incompatibilities {
					allIncompats[inc] = true
				}
				for inc := range crit.incompatibilities {
					allIncompats[inc] = true
				}
				// Pip calls r.p.findMatches here, but all that
				// has changed is the incompatibilities, so
				// fetching all of the matches for every
				// requirement is wasteful. Instead just filter
				// them ourselves.
				// TODO: maybe we can do it in place on crit?
				var matches []resolve.VersionKey
				for _, c := range crit.candidates {
					if !allIncompats[c] {
						matches = append(matches, c)
					}
				}
				if len(matches) == 0 {
					return false, nil
				}
				newCrit := crit.copy()
				newCrit.incompatibilities = allIncompats
				newCrit.candidates = matches
				r.state().criteria.Put(inc.name, newCrit)
			}
			return true, nil
		}

		r.pushNewState()
		if ok, err := patchCriteria(); err != nil {
			return false, err
		} else if ok {
			debugf(r.p.rc, "--------------------------------backtracked\n")
			return true, nil
		}

		// This state does not work with the new incompatibility
		// information. Keep winding down the stack.
	}
	// Not enough states left, all options are exhausted.
	return false, nil
}

// resolve actually performs a resolution, running for at most maxRounds.
func (r *resolution) resolve(ctx context.Context, reqs []resolve.RequirementVersion, maxRounds int) (*state, error) {
	if len(r.states) != 0 {
		return nil, errors.New("already resolved")
	}

	// Initialize the state.
	r.states = []*state{{
		mapping:  newVersionMap(0),
		criteria: newCriteria(),
	}}
	state := r.state()
	// Build the initial criteria.
	for _, req := range reqs {
		name, crit, err := r.mergeIntoCriterion(ctx, req, r.p.rootVersion)
		if err == nil {
			state.criteria.Put(name, crit)
			continue
		}
		var rce requirementsConflictedError
		if errors.As(err, &rce) {
			return nil, resolutionImpossible(rce)
		}
		return nil, err
	}
	// Push a copy of the first state, so that there is always something to
	// backtrack to.
	r.pushNewState()

	var unsatisfiedCriterionNames []resolve.PackageKey
	for i := 0; i < maxRounds; i++ {
		// Check the context every 100 iterations.
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		state := r.state()
		// Collect unsatisfied criteria.
		unsatisfiedCriterionNames = unsatisfiedCriterionNames[:0]

		// Note that the order of this iteration doesn't need to match
		// Python, because we re-prioritize the results later.
		for _, c := range *state.criteria {
			if r.isCurrentPinSatisfying(ctx, c.name, c.crit) {
				continue
			}
			unsatisfiedCriterionNames = append(unsatisfiedCriterionNames, c.name)
		}
		// If there's nothing unsatisfied we're done.
		if len(unsatisfiedCriterionNames) == 0 {
			return state, nil
		}

		// Otherwise, find the most preferred one to try and
		// pin.
		minName := unsatisfiedCriterionNames[0]

		min := r.getPreference(minName)
		for _, name := range unsatisfiedCriterionNames[1:] {
			score := r.getPreference(name)
			if score.Less(min) {
				minName = name
				min = score
			}
		}
		debugf(r.p.rc, "min key: %v\n", min)
		if debug {
			crit, _ := r.state().criteria.Get(minName)
			printCriterion(r.p.rc, crit)
		}
		// Try and pin it.
		failureCauses, err := r.attemptToPinCriterion(ctx, minName)
		if err != nil {
			return nil, err
		}
		if len(failureCauses) != 0 {
			// Attempt to backtrack.
			debugf(r.p.rc, "------ about to backtrack because:\n")
			for _, f := range failureCauses {
				debugf(r.p.rc, "\t%v\n", f.Error())
			}
			if ok, err := r.backtrack(ctx); err != nil {
				return nil, err
			} else if !ok {
				// This isn't the most useful error message,
				// because it is only why the most recent attempt
				// to pin failed. However, it is what python
				// uses, so that is good enough.
				return nil, resolutionImpossible(failureCauses...)
			}
		} else {
			// The pin worked, keep rolling through. Backtracking
			// manipulates the state stack so we only need to push a new one
			// if we did not need to backtrack.
			r.pushNewState()
		}
	}
	return nil, errTooDeep
}

// state is the state of a resolution, holding the versions pinned so far and
// the requirements to be satisfied. It corresponds to the resolvelib.State
// named tuple defined here:
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/resolvers.py#L102
type state struct {
	// mapping holds the currently pinned versions. In Python it is an
	// OrderedDict, which provides both efficient access by key and the
	// ability to remove items in the order they were added which is used
	// during backtracking.
	mapping *versionMap
	// criteria holds criterion objects which capture requirements to be satisfied
	// and matching versions.
	criteria *criteria
}

// criterion represents possible resolution results of a package. This maps to
// the Criterion object in Python defined here:
// https://github.com/pypa/pip/blob/21.1.3/src/pip/_vendor/resolvelib/resolvers.py#L45
type criterion struct {
	// informationReqs and informationParents hold all of the requirements
	// that need to be satisfied alongside the package version that brought
	// that requirement. In Python this is a single array of tuples, but we need
	// the slice of requirements often enough that we store them as two parallel
	// slices.
	// These slices may be shared between criteria and should not be
	// modified.
	informationReqs    []resolve.RequirementVersion
	informationParents []resolve.VersionKey
	// extras holds the union of all of the extras requested by each
	// requirement in information.
	extras map[string]bool
	// incompatibilities holds concrete versions of this package known not
	// to work. This is populated during backtracking: when candidates are
	// discovered not to work they are moved from candidates to
	// incompatibilities. This means over the course of resolution
	// incompatibilities will grow as candidates shrinks.
	incompatibilities map[resolve.VersionKey]bool
	// candidates holds concrete versions that might work: the intersection
	// of the matching versions for all requirements in information, minus
	// any that have been shifted to incompatibilities. During the
	// resolution the number of candidates will always shrink as new
	// requirements are discovered or as part of backtracking when
	// candidates are found not to work.
	candidates []resolve.VersionKey
}

func printCriterion(rc resolve.Client, crit criterion) {
	fmt.Println("criterion-------------------------")
	fmt.Println("requirements:")
	for i, req := range crit.informationReqs {
		p := crit.informationParents[i]
		parent := "root"
		if p != (resolve.VersionKey{}) {
			parent = p.String()
		}
		fmt.Printf("\t%v %v  <- %v\n", req.VersionKey, req.Type, parent)
	}
	fmt.Println("extras:")
	for k := range crit.extras {
		fmt.Printf("\t%s\n", k)
	}
	fmt.Println("incompatibilities:")
	for v := range crit.incompatibilities {
		fmt.Printf("\t%v\n", v)
	}
	fmt.Printf("candidates:\n")
	for i, v := range crit.candidates {
		if i >= 10 {
			fmt.Println("...")
			break
		}
		fmt.Printf("\t%v\n", v)
	}
	fmt.Println("-------------------------------")
}

// copy makes a copy of a criterion. The candidates, informationReqs and
// informationParents slices will be reused.
func (c criterion) copy() criterion {
	extras := make(map[string]bool, len(c.extras))
	for k, v := range c.extras {
		extras[k] = v
	}
	incompatibilities := make(map[resolve.VersionKey]bool, len(c.incompatibilities))
	for k, v := range c.incompatibilities {
		incompatibilities[k] = v
	}
	return criterion{
		informationReqs:    c.informationReqs,
		informationParents: c.informationParents,
		extras:             extras,
		incompatibilities:  incompatibilities,
		candidates:         c.candidates,
	}
}

// unionExtras collects any requested extras from a dep.Type and inserts them
// into a copy of the provided map. It does not mutate its argument.
func unionExtras(extras map[string]bool, t dep.Type) map[string]bool {
	newExtras := make(map[string]bool, len(extras))
	for k, v := range extras {
		newExtras[k] = v
	}
	// Requested extras are stored under the EnabledDependencies key.
	es, ok := t.GetAttr(dep.EnabledDependencies)
	if !ok {
		return newExtras
	}
	for _, e := range strings.Split(es, ",") {
		newExtras[e] = true
	}
	return newExtras
}

type criteria []criterionPair

func newCriteria() *criteria {
	c := criteria([]criterionPair{})
	return &c
}

type criterionPair struct {
	name resolve.PackageKey
	crit criterion
}

func (c *criteria) Copy() *criteria {
	d := make(criteria, c.Len())
	copy(d, *c)
	return &d
}

func (c *criteria) Len() int {
	return len(*c)
}

func (c *criteria) Put(name resolve.PackageKey, crit criterion) {
	cs := *c
	i := sort.Search(len(cs), func(i int) bool {
		return cs[i].name.Compare(name) >= 0
	})
	if i < len(cs) {
		// is it already here?
		if cs[i].name == name {
			cs[i].crit = crit
		} else {
			// Insert at i.
			cs = append(cs[:i+1], cs[i:]...)
			cs[i] = criterionPair{name: name, crit: crit}
		}
	} else {
		// It goes on the end.
		cs = append(cs, criterionPair{name: name, crit: crit})
	}
	*c = cs
}

func (c criteria) Get(name resolve.PackageKey) (criterion, bool) {
	i := sort.Search(len(c), func(i int) bool {
		return c[i].name.Compare(name) >= 0
	})
	if i < len(c) && c[i].name == name {
		return c[i].crit, true
	}
	return criterion{}, false
}

// requirementsConflictedError is used to signal when sets of requirements can
// not be satisfied.
type requirementsConflictedError struct {
	noCandidates bool // set if there are no candidates for the version at all
	name         resolve.PackageKey
	reqs         []resolve.RequirementVersion
	p            *provider
}

func requirementsConflicted(p *provider, name resolve.PackageKey, reqs []resolve.RequirementVersion) requirementsConflictedError {
	return requirementsConflictedError{name: name, reqs: reqs, p: p}
}

func (r requirementsConflictedError) Error() string {
	// This error is used during the normal course of operation to indicate
	// backtracking is required, so avoid jumping back to key space until
	// actually printed.
	pkgname := r.name.Name
	var reqs []string
	for _, req := range r.reqs {
		reqs = append(reqs, req.Version)
	}
	if r.noCandidates {
		return fmt.Sprintf("no candidates at all for: %s %q", pkgname, strings.Join(reqs, ","))
	}

	return fmt.Sprintf("requirements conflict: %s: %q", pkgname, strings.Join(reqs, ","))
}

// resolutionImpossibleError is returned when the resolution con not complete
// due to requirements that are impossible to satisfy.
type resolutionImpossibleError struct {
	rces []requirementsConflictedError
}

func resolutionImpossible(rces ...requirementsConflictedError) resolutionImpossibleError {
	return resolutionImpossibleError{rces: rces}
}

func (rie resolutionImpossibleError) Error() string {
	var sb strings.Builder
	sb.WriteString("resolution impossible:\n")
	for _, rce := range rie.rces {
		sb.WriteString(rce.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

var errTooDeep = errors.New("resolution aborted after too many iterations")
