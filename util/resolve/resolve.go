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

/*
Package resolve performs dependency resolution.

The Client interface describes how to access available package versions and
their dependencies. Implementers of the Resolver interface use a Client to
find a satisfactory set of packages and versions, and produce a Graph which
describes those versions and their relationship to one another.
*/
package resolve

import (
	"context"
	"fmt"
	"sort"

	apipb "deps.dev/api/v3"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/semver"
)

//go:generate stringer -type System,VersionType

// System nominates a packaging system, such as Maven, npm, etc.
type System byte

const (
	UnknownSystem = System(apipb.System_SYSTEM_UNSPECIFIED)
	NPM           = System(apipb.System_NPM)
	Maven         = System(apipb.System_MAVEN)
	PyPI          = System(apipb.System_PYPI)
)

// Semver returns the corresponding semver.System.
func (s System) Semver() semver.System {
	switch s {
	case NPM:
		return semver.NPM
	case Maven:
		return semver.Maven
	case PyPI:
		return semver.PyPI
	}
	return semver.DefaultSystem
}

// PackageKey uniquely identifies a package.
type PackageKey struct {
	System
	Name string
}

func (k PackageKey) String() string {
	return k.System.String() + ":" + k.Name
}

// Compare reports whether pk1 is less than, equal to or greater than pk2,
// returning a -1, 0 or 1. respectively.
// It compares System, PackageType and then Name.
func (pk1 PackageKey) Compare(pk2 PackageKey) int {
	if pk1.System < pk2.System {
		return -1
	}
	if pk1.System > pk2.System {
		return 1
	}
	if pk1.Name < pk2.Name {
		return -1
	}
	if pk1.Name > pk2.Name {
		return 1
	}
	return 0
}

// VersionKey uniquely identifies a version of a package.
type VersionKey struct {
	PackageKey
	VersionType
	Version string
}

func (k VersionKey) String() string {
	return fmt.Sprintf("%s[%s:%s]", k.PackageKey, k.VersionType, k.Version)
}

// Compare reports whether vk1 is less than, equal to or greater than vk2,
// returning -1, 0 or 1 respectively.
// It compares PackageKey, VersionType and then Version.
func (vk1 VersionKey) Compare(vk2 VersionKey) int {
	if c := vk1.PackageKey.Compare(vk2.PackageKey); c != 0 {
		return c
	}
	if vk1.VersionType < vk2.VersionType {
		return -1
	}
	if vk1.VersionType > vk2.VersionType {
		return 1
	}
	if vk1.Version < vk2.Version {
		return -1
	}
	if vk1.Version > vk2.Version {
		return 1
	}
	return 0
}

// Less reports whether vk1 sorts before vk2,
// sorting by PackageKey, VersionType, and then lexicographically by Version.
func (vk1 VersionKey) Less(vk2 VersionKey) bool { return vk1.Compare(vk2) < 0 }

// VersionType indicates the type of a version.
type VersionType byte

const (
	UnknownVersionType VersionType = iota

	// Concrete versions nominate a specific version.
	Concrete

	// Requirement versions describe dependencies; they are a reference to a
	// set of acceptable concrete versions. Their version strings are
	// whatever their ecosystem uses to refer to a particular version or set
	// of versions in dependencies.
	Requirement
)

// SortVersionKeys sorts the given slice of VersionKeys
// in the order specified by the VersionKey.Less method.
func SortVersionKeys(ks []VersionKey) {
	sort.Sort(versionKeys(ks))
}

type versionKeys []VersionKey

func (s versionKeys) Len() int           { return len(s) }
func (s versionKeys) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s versionKeys) Less(i, j int) bool { return s[i].Less(s[j]) }

// RequirementVersion represents a direct dependency.
type RequirementVersion struct {
	VersionKey // The requirement version.
	Type       dep.Type
}

func (d RequirementVersion) String() string {
	s := d.VersionKey.String()
	if !d.Type.IsRegular() {
		s = d.Type.String() + "|" + s
	}
	return s
}

// Resolver describes a dependency resolver.
type Resolver interface {
	Resolve(context.Context, VersionKey) (*Graph, error)
}
