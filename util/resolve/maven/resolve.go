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
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"
	"time"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	versionpkg "deps.dev/util/resolve/version"
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

// TODO: a user may set the default registry outside pom.xml, so we should
// allow injecting the registry configuration.
func (r *resolver) Resolve(ctx context.Context, vk resolve.VersionKey) (*resolve.Graph, error) {
	start := time.Now()
	// Resolve first in full-visibility mode. If only one registry is required,
	// this is the result.
	g, hasMulti, err := r.resolve(ctx, vk, false)
	if !hasMulti {
		return g, err
	}
	// Resolve allowing multiple registries.
	gm, _, err := r.resolve(ctx, vk, true)
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
func (r *resolver) resolve(ctx context.Context, vk resolve.VersionKey, multi bool) (g *resolve.Graph, hasMulti bool, err error) {
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

	fetchRepos, depRepos := parseRegistries(ver.AttrSet)
	if len(depRepos) > 0 {
		v.repositories = append([]string(nil), depRepos...)
		v.repositories = append(v.repositories, fetchRepos...)
	} else {
		v.repositories = fetchRepos
	}
	todo := []version{v}

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
			// We skip test and optional dependencies, as for a consumer, none
			// would be included (the optional would be indirect).
			// https://maven.apache.org/guides/introduction/introduction-to-optional-and-excludes-dependencies.html#how-do-optional-dependencies-work
			opt = providedImports
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
			if v, ok := mgt[r.packageKeyForDependency(d.RequirementVersion)]; ok && !first {
				d.Version = v.Version
			}
			matches, err := r.client.MatchingVersions(ctx, d.VersionKey)
			if err != nil {
				return nil, false, err
			}

			// Look if this is already resolved.
			matched := false
			c := versionKey{
				packageKey: r.packageKeyForDependency(d.RequirementVersion),
			}
			for _, m := range matches {
				c.VersionKey = m.VersionKey
				if _, ok := concreteVersions[c]; !ok {
					continue
				}
				if err := g.AddEdge(concreteVersions[cur.versionKey], concreteVersions[c], d.Version, d.Type); err != nil {
					return nil, false, err
				}
				matched = true
				break
			}
			if matched {
				continue
			}

			// Remember the versions that we can't access.
			reachables := matches[:0]
			cloned := false
			for _, m := range matches {
				if hasMulti && !multi {
					// No need to check more, we already know that multiple
					// registries are required.
					break
				}
				registries, _ := parseRegistries(m.AttrSet)
				// TODO: revisit the logic here once we support injecting
				// the registry configuration.
				// Attributes having no registries means the package only
				// available in the default registry.
				keep := len(registries) == 0
				for _, reg := range registries {
					if reg == "" {
						// This is on the default registry, keep it.
						keep = true
						break
					}
					if u, err := url.Parse(reg); err == nil {
						if u.Host == "repo.maven.apache.org" && strings.Trim(u.Path, "/") == "maven2" {
							// This is on Maven Central, keep it
							keep = true
							break
						}
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
				if multi {
					if keep {
						reachables = append(reachables, m)
					} else if !cloned {
						// Clone to avoid altering matches.
						reachables = append([]resolve.Version(nil), reachables...)
						cloned = true
					}
				}
				if !keep {
					hasMulti = true
				}
			}

			if multi {
				matches = reachables
			}
			if len(matches) == 0 {
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

			match := matches[len(matches)-1]
			// Prefer a direct match if it exists: this honors the preference
			// of soft requirements.
			rqt := d.VersionKey
			// Because the version strings are the same, just
			// check whether the corresponding concrete version
			// exists.
			rqt.VersionType = resolve.Concrete
			// Double check the version actually exists, and
			// hasn't been deleted by ensuring it is one of
			// the matching versions.
			for _, mv := range matches {
				if rqt == mv.VersionKey {
					match = mv
					break
				}
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
			if d.exclusions != nil {
				mergeExclusions(d.exclusions, cur.exclusions)
				n.exclusions = d.exclusions
			}
			_, registries := parseRegistries(match.AttrSet)
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

func parseRegistries(a versionpkg.AttrSet) (fetch []string, dep []string) {
	r, ok := a.GetAttr(versionpkg.Registries)
	if !ok {
		return nil, nil
	}
	for _, rr := range strings.Split(r, "|") {
		rr := strings.TrimSpace(rr)
		if reg, ok := strings.CutPrefix(rr, "dep:"); ok {
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
