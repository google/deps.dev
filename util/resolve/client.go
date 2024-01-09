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
	"context"
	"errors"
	"fmt"

	"deps.dev/util/resolve/version"
)

// Version combines a VersionKey with the version's attributes.
type Version struct {
	VersionKey
	version.AttrSet
}

func (v Version) String() string {
	return fmt.Sprintf("{%v %v}", v.VersionKey, v.AttrSet)
}

// Equal reports whether the two versions are equivalent.
func (v Version) Equal(w Version) bool {
	if v.VersionKey == w.VersionKey {
		return true
	}
	return v.AttrSet.Equal(w.AttrSet)
}

// Client defines an interface to fetch the data needed for dependency
// resolutions.
type Client interface {
	// Version finds a particular version, providing access to its
	// attributes.
	Version(context.Context, VersionKey) (Version, error)
	// Versions returns all the known versions of a package.
	Versions(context.Context, PackageKey) ([]Version, error)
	// Requirements returns the direct dependencies of the provided version.
	Requirements(context.Context, VersionKey) ([]RequirementVersion, error)
	// MatchingVersions return the set of concrete versions that match the
	// provided requirement version. The versions are returned in a
	// system-specific order, expected by the relevant resolver.
	MatchingVersions(context.Context, VersionKey) ([]Version, error)
}

// ErrNotFound is returned by Clients to indicate the requested data could not
// be located.
var ErrNotFound = errors.New("not found")

type LocalClient struct {
	// PackageVersions holds all the Concrete versions of every package.
	PackageVersions map[PackageKey][]Version
	// imports holds the direct dependencies of every concrete version.
	imports map[VersionKey][]RequirementVersion
}

// NewLocalClient creates a new, empty, LocalClient.
func NewLocalClient() *LocalClient {
	return &LocalClient{
		PackageVersions: make(map[PackageKey][]Version),
		imports:         make(map[VersionKey][]RequirementVersion),
	}
}

// AddVersion adds a version to the client along with its direct dependencies.
// Any existing version will be replaced. Also ensures all packages in the
// dependencies have an entry in the PackageVersions map, although it may be
// empty.
func (lc *LocalClient) AddVersion(v Version, deps []RequirementVersion) {
	if v.HasAttr(version.Deleted) {
		return
	}

	versions := lc.PackageVersions[v.PackageKey]
	// If an equivalent version already exists, replace it to use the new
	// attributes.
	existed := false
	for i, w := range versions {
		if w.VersionKey == v.VersionKey {
			existed = true
			versions[i] = w
		}
	}
	// Otherwise insert and sort.
	if !existed {
		versions = append(versions, v)
		SortVersions(versions)
	}
	lc.PackageVersions[v.PackageKey] = versions

	SortDependencies(deps)
	lc.imports[v.VersionKey] = deps

	// Ensure dependency packages exist, even though we might
	// not have versions for them.
	for _, d := range deps {
		if _, ok := lc.PackageVersions[d.PackageKey]; !ok {
			lc.PackageVersions[d.PackageKey] = []Version{}
		}
	}
}

// Version implements Client, finding a Version by key.
func (lc *LocalClient) Version(ctx context.Context, vk VersionKey) (Version, error) {
	for _, v := range lc.PackageVersions[vk.PackageKey] {
		if v.VersionKey == vk {
			return v, nil
		}
	}
	return Version{}, fmt.Errorf("version %v: %w", vk, ErrNotFound)
}

// Versions implements Client, returning all of the known Concrete versions for
// the given package.
func (lc *LocalClient) Versions(ctx context.Context, pk PackageKey) ([]Version, error) {
	if vs, ok := lc.PackageVersions[pk]; ok {
		return vs, nil
	}
	return nil, fmt.Errorf("package %v: %w", pk, ErrNotFound)
}

// Requirements implements Client, returning the direct dependencies of a version.
func (lc *LocalClient) Requirements(ctx context.Context, vk VersionKey) ([]RequirementVersion, error) {
	if deps, ok := lc.imports[vk]; ok {
		return deps, nil
	}
	return nil, fmt.Errorf("version %v: %w", vk, ErrNotFound)
}

// MatchingVersions implements Client, returning all of the known Concrete
// versions that satisfy the provided requirement.
func (lc *LocalClient) MatchingVersions(ctx context.Context, vk VersionKey) ([]Version, error) {
	vs, ok := lc.PackageVersions[vk.PackageKey]
	if !ok {
		return nil, fmt.Errorf("version: %v: %w", vk, ErrNotFound)
	}
	ms := MatchRequirement(vk, vs)
	return ms, nil
}
