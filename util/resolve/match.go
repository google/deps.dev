// Copyright 2023 Google LLC
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

package resolve

import (
	"sort"
	"strings"

	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/version"
	"deps.dev/util/semver"
)

// SortVersions sorts a set of version in ascending order, by semver.
func SortVersions(vs []Version) {
	if len(vs) == 0 {
		return
	}
	if vs[0].System == NPM {
		sortNPMVersions(vs)
		return
	}
	sys := vs[0].System.Semver()
	vers := make(map[VersionKey]*semver.Version)
	for _, v := range vs {
		ver, err := sys.Parse(v.Version)
		if err != nil {
			continue
		}
		vers[v.VersionKey] = ver
	}
	sort.Slice(vs, func(i, j int) bool {
		vi, vj := vers[vs[i].VersionKey], vers[vs[j].VersionKey]
		if vi == nil || vj == nil {
			// Does this make any sense at all?
			return vs[i].Version < vs[j].Version
		}
		return vi.Compare(vj) < 0
	})
}

func sortNPMVersions(vs []Version) {
	vers := make(map[VersionKey]*semver.Version)
	for _, v := range vs {
		ver, err := semver.NPM.Parse(v.Version)
		if err != nil {
			continue
		}
		vers[v.VersionKey] = ver
	}
	sort.Slice(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		av, bv := vers[a.VersionKey], vers[b.VersionKey]
		if (av != nil) != (bv != nil) {
			return av != nil
		} else if av != nil {
			if c := av.Compare(bv); c != 0 {
				return c < 0
			}
		}
		// Otherwise order lexicographically.
		return a.VersionKey.Version < b.VersionKey.Version
	})

	var (
		allPrerelease      = true
		latestIdx          = -1
		latestIsPrerelease = false
	)
	// Find the "latest" if present.
	for i, v := range vs {
		if sv := vers[v.VersionKey]; sv != nil {
			allPrerelease = allPrerelease && sv.IsPrerelease()
		} else {
			allPrerelease = false
		}
		if tags, _ := v.GetAttr(version.Tags); strings.Contains(tags, "latest") {
			latestIdx = i
			latestIsPrerelease = vers[v.VersionKey] != nil && vers[v.VersionKey].IsPrerelease()
		}
	}

	// If there was a "latest" tag, move it to the end unless it points
	// to a pre-release and there are matching non pre-release versions.
	if latestIdx >= 0 && !(latestIsPrerelease && !allPrerelease) {
		latest := vs[latestIdx]
		copy(vs[latestIdx:], vs[latestIdx+1:])
		vs[len(vs)-1] = latest
	}
}

// SortDependencies sorts a set of dependencies in a system-specific order for
// resolution. For many systems this is no order, as it can be important to the
// resolution that they are processed in the order they were retrieved from the
// metadata.
func SortDependencies(deps []RequirementVersion) {
	if len(deps) == 0 {
		return
	}
	switch deps[0].System {
	case NPM:
		sortNPMDependencies(deps)
	}
}

func sortNPMDependencies(deps []RequirementVersion) {
	dev := dep.NewType(dep.Dev)
	// Lowercase lexicographic ordering on the package name.
	// In case of matching lowercase, lower is considered less than upper
	// ("a" < "A", that is the contrary of what go does, but is logic for
	// NPM as uppercase is considered deprecated in names, so it favors lower).
	sort.Slice(deps, func(i, j int) bool {
		a, b := deps[i], deps[j]
		// Sort dev alone at the end.
		if devA, devB := a.Type.Equal(dev), b.Type.Equal(dev); devA != devB {
			return devB
		}

		na, nb := a.Name, b.Name
		if n, ok := a.Type.GetAttr(dep.KnownAs); ok {
			na = n
		}
		if n, ok := b.Type.GetAttr(dep.KnownAs); ok {
			nb = n
		}

		la, lb := strings.ToLower(na), strings.ToLower(nb)
		if la != lb {
			return la < lb
		}
		return na > nb
	})
}

// MatchRequirement returns the items from the given list of Concrete versions
// that match the given Requirement version, with appropriate system-specific
// logic. The list can be in any order, which may be modified, and the returned
// versions will be in a system-specific order expected by the relevant
// resolver.
func MatchRequirement(req VersionKey, versions []Version) []Version {
	switch req.System {
	case NPM:
		return matchNPMRequirement(req, versions)
	default:
		return matchRequirement(req, versions)
	}
}

// matchNPMRequirement matches npm requirements.
func matchNPMRequirement(req VersionKey, vers []Version) []Version {
	sortNPMVersions(vers)
	constraint, err := req.System.Semver().ParseConstraint(req.Version)
	if err != nil {
		// Look for an exact string match, either on the version string
		// or in the tags.
		for _, v := range vers {
			if req.Version == v.Version {
				return []Version{v}
			}
			tags, _ := v.GetAttr(version.Tags)
			for _, tag := range strings.Split(tags, ",") {
				if req.Version == tag {
					return []Version{v}
				}
			}
		}
		return nil
	}
	matches := make([]Version, 0, len(vers))
	for _, v := range vers {
		if constraint.Match(v.Version) {
			matches = append(matches, v)
		}
	}
	return matches
}

// matchRequirement is a default implementation of MatchRequirement, appropriate
// for many systems.
func matchRequirement(req VersionKey, versions []Version) []Version {
	constraint, err := req.System.Semver().ParseConstraint(req.Version)
	if err != nil {
		// Fall back to string matching.
		constraint = nil
	}
	matches := make([]Version, 0, len(versions))
	for _, v2 := range versions {
		// If v is a semver constraint match using semver; otherwise
		// just string match.
		if constraint != nil {
			if !constraint.Match(v2.Version) {
				continue
			}
		} else if req.Version != v2.Version {
			continue
		}
		// TODO: use the attributes properly
		matches = append(matches, v2)
	}
	return matches
}
