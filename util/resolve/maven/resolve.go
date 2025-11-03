// Copyright 2024 Google LLC
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
Package maven implements a resolver for Maven dependencies, based on Maven
version 3.6.3.
*/
package maven

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	versionpkg "deps.dev/util/resolve/version"
	"deps.dev/util/semver"
)

const (
	debug = false
)

// resolver implements resolve.Resolver for Maven.
type resolver struct {
	client resolve.Client
}

// NewResolver creates a Maven Resolver connected to the given client.
func NewResolver(client resolve.Client) resolve.Resolver {
	return &resolver{
		client: client,
	}
}

// version represents a concrete version to resolve that adds transitive
// exclusions to a concrete coordinate.
type version struct {
	// versionKey holds the version key.
	versionKey
	// includesDependencies indicates whether the artifact contains its
	// dependencies as part of its type. If set, do not resolve transitively.
	// war, ear, and rar include their dependencies.
	// https://maven.apache.org/ref/3.6.3/maven-core/artifact-handlers.html
	includesDependencies bool
	// exclusions holds the packages to exclude from the transitive
	// dependencies.
	exclusions map[string]bool
	// repositories holds the repositories where to look for dependencies.
	repositories []string
}

// dependency represents a Maven dependency that adds exclusions to a resolve
// dependency.
type dependency struct {
	resolve.RequirementVersion
	// exclusions holds the packages to exclude from the transitive
	// dependencies.
	exclusions map[string]bool
}

// packageKey represents a unique key for the resolver. In Maven, only
// one version of a given packageKey can be installed.
type packageKey struct {
	resolve.PackageKey
	// classifier holds the classifier of the artifact.
	classifier string
	// typ holds the type of the artifact.
	typ string
}

// versionKey represents a unique key for the resolver. In Maven, only
// one version of a given packageKey can be installed.
type versionKey struct {
	packageKey
	resolve.VersionKey
}

var errIncompatible = errors.New("incompatible requirements")

// TODO: a user may set the default registry outside pom.xml, so we should
// allow injecting the registry configuration.
func (r *resolver) Resolve(ctx context.Context, vk resolve.VersionKey) (*resolve.Graph, error) {
	start := time.Now()
	const maxRetries = 100
	// requirements holds all requirements that we encounter during the
	// resolution.
	// This is used for packages that appear with several and different
	// requirements in the dependency graph. Maven allows only one concrete
	// version per package: the effective requirement is the intersection of
	// all requirements for a given package.
	requirements := make(map[packageKey][]resolve.VersionKey)
	// Resolve first in full-visibility mode. If only one registry is required,
	// this is the result.
	g, hasMulti, err := r.resolve(ctx, vk, requirements, false)
	// Set a limit on how many times to retry the resolution.
	for i := 0; i < maxRetries && errors.Is(err, errIncompatible); i++ {
		// Check the context at each iteration.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// The requirements map has been mutated with the additional
		// requirements, retry the resolution with the new set to see if
		// this will yield a compatible version for all (or if more
		// incompatible requirements will be discovered).
		g, hasMulti, err = r.resolve(ctx, vk, requirements, false)
	}
	if !hasMulti {
		return g, err
	}

	// Resolve allowing multiple registries.
	gm, _, err := r.resolve(ctx, vk, requirements, true)
	if err != nil {
		return nil, err
	}
	// Reset duration for comparison.
	g.Duration, gm.Duration = 0, 0
	if equal, err := eq(g, gm); err != nil {
		return nil, err
	} else if !equal {
		// TODO: record a warning instead of an error.
		e := "multi-registry resolution differ: missing repository configuration"
		if g.Error == "" {
			g.Error = e
		} else {
			g.Error += "; " + e
		}
	}
	g.Duration = time.Since(start)
	return g, nil
}

// resolve resolves the given version specified by the version key.
// If multi is true, the resolved graph may include versions available on
// any Maven repository.
// If multi is false, the results are limited by the repositories defined in
// each respective version's pom.xml.
// In all cases, resolve returns whether some matching versions are in
// multiple repositories.
func (r *resolver) resolve(ctx context.Context, vk resolve.VersionKey, requirements map[packageKey][]resolve.VersionKey, multi bool) (g *resolve.Graph, hasMulti bool, err error) {
	if vk.System != resolve.Maven {
		return nil, false, fmt.Errorf("expected %s system, got %s", resolve.Maven, vk.System)
	}
	if vk.VersionType != resolve.Concrete {
		return nil, false, fmt.Errorf("expected %s version, got %s", resolve.Concrete, vk.VersionType)
	}

	start := time.Now()
	ver, err := r.client.Version(ctx, vk)
	if err != nil {
		return
	}

	g = &resolve.Graph{}
	g.AddNode(vk)

	v := version{
		versionKey: versionKey{
			packageKey: r.packageKeyForDependency(resolve.RequirementVersion{VersionKey: vk}),
			VersionKey: vk,
		},
	}

	defaultRegistry, fetchRepos, depRepos := parseRegistries(ver.AttrSet)
	if len(depRepos) > 0 {
		v.repositories = append([]string(nil), depRepos...)
		v.repositories = append(v.repositories, fetchRepos...)
	} else {
		v.repositories = fetchRepos
	}
	todo := []version{v}

	resolvedPackages := map[packageKey]bool{todo[0].packageKey: true}
	concreteVersions := map[versionKey]resolve.NodeID{todo[0].versionKey: 0}
	// nodes ensure that there is only one resolve node per version key,
	// regardless of the dependency type that yields to that resolution.
	nodes := map[resolve.VersionKey]resolve.NodeID{vk: 0}
	mgt, err := r.dependencyManagement(ctx, ver.VersionKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot get dependency management: %w", err)
	}

	for first := true; len(todo) > 0; first = false {
		var cur version
		// This is a BFS, Maven takes the "nearest" definition.
		// https://maven.apache.org/guides/introduction/introduction-to-dependency-mechanism.html#transitive-dependencies
		cur, todo = todo[0], todo[1:]

		if debug {
			log.Printf("cur: %s", cur.VersionKey)
		}
		if cur.includesDependencies {
			continue
		}

		var opt importsOpt
		if first {
			// TODO: make the allowed types of imports configurable
			opt = testImports | optImports | providedImports
		}
		imps, err := r.imports(ctx, cur.VersionKey, opt)
		if err == resolve.ErrNotFound && !first {
			// If the concrete version ver can't be found, it's only
			// a fatal error in the first instance; otherwise proceed.
			continue
		} else if err != nil {
			return nil, false, err
		}

		for _, d := range imps {
			if debug {
				log.Printf("dep: %s %s", d.VersionKey, d.Type)
			}

			if isExcluded, err := r.isExcluded(cur.exclusions, d.VersionKey); err != nil {
				return nil, false, err
			} else if isExcluded {
				if debug {
					log.Printf("dep excluded: %s %s", d.VersionKey, d.Type)
				}
				continue
			}

			c := versionKey{
				packageKey: r.packageKeyForDependency(d.RequirementVersion),
			}
			if v, ok := mgt[c.packageKey]; ok && !first {
				d.Version = v.Version
			}
			if reqs := requirements[c.packageKey]; !slices.Contains(reqs, d.VersionKey) {
				// Append the requirement if it is not seen before
				requirements[c.packageKey] = append(reqs, d.VersionKey)
			}

			match, err := r.findMatch(ctx, requirements[c.packageKey])
			if errors.Is(err, errNoMatch) {
				reqs := make([]string, len(requirements[c.packageKey]))
				for i, req := range requirements[c.packageKey] {
					reqs[i] = req.Version
				}
				slices.Sort(reqs)
				g.AddError(concreteVersions[cur.versionKey], d.VersionKey, fmt.Sprintf("could not find a version that satisfies requirements %s for package %s", reqs, d.Name))
				continue
			} else if err != nil {
				return nil, false, err
			}

			// Look if this is already resolved.
			c.VersionKey = match.VersionKey
			if _, ok := concreteVersions[c]; ok {
				if err := g.AddEdge(concreteVersions[cur.versionKey], concreteVersions[c], d.Version, d.Type); err != nil {
					return nil, false, err
				}
				continue
			}
			if ok := resolvedPackages[c.packageKey]; ok {
				// Not matched but already resolved, which indicates this is an
				// incompatible requirement
				reqs, ok2 := requirements[c.packageKey]
				if !ok2 {
					reqs = []resolve.VersionKey{}
				}
				// TODO: check requirement duplicates?
				requirements[c.packageKey] = append(reqs, d.VersionKey)
				return nil, false, errIncompatible
			}

			// Check if this is a version that we can't access.
			reachable := false
			if !hasMulti || multi {
				// Only need to check if we don't already know if multiple
				// registries are required.
				_, registries, _ := parseRegistries(match.AttrSet)
				// Attributes having no registries means the package only
				// available in the default registry.
				keep := len(registries) == 0
				for _, reg := range registries {
					if reg == "" {
						// This is on the default registry, keep it.
						keep = true
						break
					}

					if defaultRegistry == "" {
						// If default registry is not set, assume it's Maven Central.
						if u, err := url.Parse(reg); err == nil {
							if u.Host == "repo.maven.apache.org" && strings.Trim(u.Path, "/") == "maven2" {
								// This is on Maven Central, keep it
								keep = true
								break
							}
						}
					} else if reg == defaultRegistry {
						keep = true
						break
					}

					for _, rep := range cur.repositories {
						// It can be reached, keep it.
						if reg == rep {
							keep = true
							break
						}
					}
					if keep {
						break
					}
				}
				if multi && keep {
					reachable = true
				}
				if !keep {
					hasMulti = true
				}
			}

			if multi && !reachable {
				// TODO: in the case of provided, we should use a similar
				// mechanism to npm bundles with derived packages.
				// In the meantime, just skip the error as this is most
				// probably a false positive.
				if s, _ := d.Type.GetAttr(dep.Scope); s == "provided" {
					continue
				}
				vk := d.VersionKey
				g.AddError(concreteVersions[cur.versionKey], vk, fmt.Sprintf("could not find a version that satisfies requirement %s for package %s", vk.Version, vk.Name))
				continue
			}

			if id, ok := nodes[match.VersionKey]; ok {
				// The version key is already in the graph, just add an edge.
				if err := g.AddEdge(concreteVersions[cur.versionKey], id, d.Version, d.Type); err != nil {
					return nil, false, err
				}
				continue
			}

			matchID := g.AddNode(match.VersionKey)
			nodes[match.VersionKey] = matchID
			dt := d.Type.Clone()
			dt.AddAttr(dep.Selector, "")
			if err := g.AddEdge(concreteVersions[cur.versionKey], matchID, d.Version, dt); err != nil {
				return nil, false, err
			}
			n := version{
				versionKey: versionKey{
					packageKey: r.packageKeyForDependency(d.RequirementVersion),
					VersionKey: match.VersionKey,
				},
				exclusions:   cur.exclusions,
				repositories: cur.repositories,
			}
			if t, ok := d.Type.GetAttr(dep.MavenArtifactType); ok {
				n.includesDependencies = t == "ear" || t == "war" || t == "rar"
			}
			concreteVersions[n.versionKey] = matchID
			resolvedPackages[n.packageKey] = true
			if d.exclusions != nil {
				mergeExclusions(d.exclusions, cur.exclusions)
				n.exclusions = d.exclusions
			}
			_, _, registries := parseRegistries(match.AttrSet)
			// Add the list of declared repositories in the version to the list
			// of reachable repositories.
			if len(registries) > 0 {
				n.repositories = append([]string(nil), cur.repositories...)
				n.repositories = append(n.repositories, registries...)
			}
			todo = append(todo, n)
		}
	}
	g.Duration = time.Since(start)
	return g, hasMulti, nil
}

type importsOpt byte

const (
	testImports importsOpt = 1 << iota
	optImports
	providedImports
)

var errNoMatch = errors.New("no version satisfies all requirements")

// findMatch returns the preferred matching versions for the given requirements.
// Requirements should be given in the order encountered during resolution.
// Returns errNoMatch if no versions satisfy the constraints.
//
// Maven seems to choose the a version based on the order it's encountered,
// skipping versions that don't satisfy all of the hard requirements.
// e.g. {requirements in order} -> selected version:
// {1.0, 2.0} -> 1.0
// {1.0, [2.0,3.0]} -> 3.0
// {1.0, 2.0, [2.0,3.0]} -> 2.0
func (r *resolver) findMatch(ctx context.Context, requirements []resolve.VersionKey) (resolve.Version, error) {
	// This sanity check is probably not necessary.
	if len(requirements) == 0 {
		return resolve.Version{}, errors.New("no requirements provided")
	}
	pk := requirements[0].PackageKey
	for _, req := range requirements[1:] {
		if req.PackageKey != pk {
			return resolve.Version{}, fmt.Errorf("requirement package key mismatch: %s != %s", req.PackageKey, pk)
		}
	}

	var (
		softVersions    []resolve.VersionKey // The soft versions, in order.
		hardConstraints []*semver.Constraint // All hard requirement constraints.
		hardIdx         = -1                 // The index of the first hard requirement encountered.
		versions        []resolve.Version    // The cached result of client.Versions()
	)
	// We only want to call client.Versions() if we actually see a
	// hard requirement. Maven only checks this file for hard requirements -
	// soft requirements are downloaded directly.

	// Iterate through to find hard constraints and preference order.
	for i, req := range requirements {
		constraint, err := semver.Maven.ParseConstraint(req.Version)
		if err != nil {
			return resolve.Version{}, fmt.Errorf("failed parsing version constraint '%s': %w", req, err)
		}

		if constraint.IsSimple() { // Soft requirement
			req.VersionType = resolve.Concrete
			softVersions = append(softVersions, req)
			continue
		}

		// Hard requirement
		if hardIdx == -1 {
			// First hard requirement we've encountered.
			hardIdx = i
			// Grab the list of available versions, in descending order.
			versions, err = r.client.Versions(ctx, req.PackageKey)
			if err != nil {
				return resolve.Version{}, err
			}
			resolve.SortVersions(versions)
			slices.Reverse(versions)
		}
		// Maven errors if the hard requirement does not match at least one version
		// in the metadata files. Imitate that behavior here.
		if !slices.ContainsFunc(versions, func(v resolve.Version) bool { return constraint.Match(v.Version) }) {
			return resolve.Version{}, fmt.Errorf("found no versions matching the constraint %s", req.Version)
		}
		hardConstraints = append(hardConstraints, constraint)
	}

	// Find the first preferred version that satisfies all constraints.
	matchesAll := func(ver string) bool {
		return !slices.ContainsFunc(hardConstraints, func(c *semver.Constraint) bool { return !c.Match(ver) })
	}
	for i, vk := range softVersions {
		if i == hardIdx {
			// This is the point where a hard requirement would be preferred.
			// Use a match in the listed versions.
			if idx := slices.IndexFunc(versions, func(v resolve.Version) bool { return matchesAll(v.Version) }); idx != -1 {
				return versions[idx], nil
			}
			// No match is not an error - there may be an unlisted soft requirement
			// after this that satisfies the constraints.
		}

		if matchesAll(vk.Version) {
			// The soft requirement satisfies hard constraints.
			// Only now do we check if the soft requirement actually exists.
			return r.client.Version(ctx, vk)
		}
	}

	if len(softVersions) == hardIdx {
		// Hard requirement was at end of list - find a match.
		if idx := slices.IndexFunc(versions, func(v resolve.Version) bool { return matchesAll(v.Version) }); idx != -1 {
			return versions[idx], nil
		}
	}

	return resolve.Version{}, errNoMatch
}

func (r *resolver) imports(ctx context.Context, ver resolve.VersionKey, opt importsOpt) (deps []dependency, err error) {
	imps, err := r.client.Requirements(ctx, ver)
	if err != nil {
		return nil, fmt.Errorf("cannot get imports for %s: %w", ver, err)
	}
	for _, imp := range imps {
		if opt&testImports == 0 && imp.Type.HasAttr(dep.Test) {
			continue
		}
		if opt&optImports == 0 && imp.Type.HasAttr(dep.Opt) {
			continue
		}
		if imp.Type.HasAttr(dep.MavenDependencyOrigin) {
			continue
		}
		if opt&providedImports == 0 {
			if scope, ok := imp.Type.GetAttr(dep.Scope); ok && scope == "provided" {
				continue
			}
		}
		d := dependency{
			RequirementVersion: imp,
		}
		d.Type = imp.Type.Clone()
		if s, ok := imp.Type.GetAttr(dep.MavenExclusions); ok {
			d.exclusions = parseExclusions(s)
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// parseExclusions splits the given exclusion string and returns a map of the
// fields. The exclusions are received as a comma- or pipe-separated string.
func parseExclusions(s string) map[string]bool {
	if s == "" {
		return nil
	}
	// sep returns whether the given rune is a separator.
	sep := func(r rune) bool {
		return r == '|' || r == ','
	}
	excl := make(map[string]bool)
	for _, e := range strings.FieldsFunc(s, sep) {
		excl[e] = true
	}
	return excl
}

func (r *resolver) dependencyManagement(ctx context.Context, vk resolve.VersionKey) (mgt map[packageKey]resolve.VersionKey, err error) {
	imps, err := r.client.Requirements(ctx, vk)
	if err != nil {
		return nil, fmt.Errorf("imports for %s: %w", vk, err)
	}
	for _, imp := range imps {
		if origin, ok := imp.Type.GetAttr(dep.MavenDependencyOrigin); !ok || origin != "management" {
			continue
		}
		if debug {
			log.Printf("mgt: %s %s", imp.VersionKey, imp.Type)
		}
		if mgt == nil {
			mgt = make(map[packageKey]resolve.VersionKey)
		}
		mgt[r.packageKeyForDependency(imp)] = imp.VersionKey
	}
	return mgt, nil
}

func (r *resolver) packageKeyForDependency(d resolve.RequirementVersion) packageKey {
	c := packageKey{
		PackageKey: d.PackageKey,
	}
	if classifier, ok := d.Type.GetAttr(dep.MavenClassifier); ok {
		c.classifier = classifier
	}
	if typ, ok := d.Type.GetAttr(dep.MavenArtifactType); ok {
		if typ != "jar" {
			c.typ = typ
		}
	}
	return c
}

func mergeExclusions(exclusions, other map[string]bool) {
	for k, v := range other {
		exclusions[k] = v
	}
}

// isExcluded specifies if the version is excluded.
// The check occurs in key space.
func (r *resolver) isExcluded(excl map[string]bool, v resolve.VersionKey) (bool, error) {
	if excl == nil {
		return false, nil
	}
	// All exclusion.
	if excl["*:*"] {
		return true, nil
	}
	// Direct exclusion.
	if excl[v.Name] {
		return true, nil
	}
	// Wildcard exclusion.
	fields := strings.Split(v.Name, ":")
	if len(fields) != 2 {
		return false, fmt.Errorf("invalid name, except 1 unique colon, got %s", v.Name)
	}
	return excl[fields[0]+":*"] || excl["*:"+fields[1]], nil
}

func parseRegistries(a versionpkg.AttrSet) (defaultRegistry string, fetch []string, dep []string) {
	r, ok := a.GetAttr(versionpkg.Registries)
	if !ok {
		return "", nil, nil
	}
	for _, rr := range strings.Split(r, "|") {
		rr := strings.TrimSpace(rr)
		if reg, ok := strings.CutPrefix(rr, "default:"); ok {
			defaultRegistry = reg
		} else if reg, ok := strings.CutPrefix(rr, "dep:"); ok {
			dep = append(dep, reg)
		} else {
			fetch = append(fetch, rr)
		}
	}
	return
}

// eq returns whether the two given graphs are equal.
func eq(g1, g2 *resolve.Graph) (bool, error) {
	if err := g1.Canon(); err != nil {
		return false, fmt.Errorf("canon: %w", err)
	}
	if err := g2.Canon(); err != nil {
		return false, fmt.Errorf("canon multi: %w", err)
	}
	return reflect.DeepEqual(g1, g2), nil
}
