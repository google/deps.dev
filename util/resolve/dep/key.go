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

package dep

//go:generate go run stringer -type AttrKey -output stringer.go

// AttrKey represents an attribute key that may be applied to a Type.
//
// Its specific values are an implementation detail of this package;
// only use the named constants in client code.
type AttrKey int8

// The negative AttrKey values below are stored in a compact form
// and have special handling in type.go.

const (
	// Use a 5 bit mask for special attributes.
	maskLen = 5

	// Dev indicates the dependency is required to develop a package.
	// Its value is ignored; its presence is the indicator.
	Dev AttrKey = -0x01

	// Opt indicates the dependency is optional; it is not
	// necessary but may provide additional functionality.
	// Its value is ignored; its presence is the indicator.
	Opt AttrKey = -0x02

	// Test indicates the dependency is required to build a package's tests.
	// Its value is ignored; its presence is the indicator.
	Test AttrKey = -0x04

	// -0x08 and -0x10 are reserved for future use.

	// The previous AttrKey are represented compactly in the encoded form.
	// Below here are AttrKey whose values are serialized.

	// XTest indicates the dependency is from a Go XTest.
	XTest AttrKey = 1

	// Framework indicates the dependency belongs to a NuGet target framework.
	Framework AttrKey = 2

	// Scope indicates the scope of a dependency.
	// Its value should be one of these:
	// - For Maven dependencies: provided, runtime, system, import
	// - For Cargo dependencies: build
	// - For NPM dependencies: peer, bundle
	// Maven scopes 'compile' and 'test' are not valid here. They are
	// modeled as regular dependency and test dependency. NPM scope 'bundle'
	// means the dependency was declared as such, not necessarily that it
	// was found inside the tarball.
	Scope AttrKey = 3

	// Attribute keys for Maven dependencies.
	//
	// MavenClassifier and MavenArtifactType are part of Maven dependency key.
	// They are both free text defined by Maven package maintainers.
	//
	// MavenDependencyOrigin indicates the origin of a Maven dependency.
	// Its value should be one of these: import, management, parent.
	//
	// MavenExclusions holds the list of exclusions for the given dependency.
	// Each item of the list is separated by | (a pipe) and is of the form
	// groupID:artifactID where groupID and/or artifactID may be a * (wildcard).
	MavenClassifier       AttrKey = 4
	MavenArtifactType     AttrKey = 5
	MavenDependencyOrigin AttrKey = 6
	MavenExclusions       AttrKey = 9

	// EnabledDependencies represents which optional dependencies
	// are enabled by this dependent in its dependency.
	//
	// In Cargo, this is a comma-separated list of features/optional
	// dependencies that are activated by this dependency.
	EnabledDependencies AttrKey = 7

	// KnownAs is the name under which this dependency is referenced
	// by the package.
	KnownAs AttrKey = 8

	// Environment holds conditions used to filter dependencies according to
	// local context.
	//
	// In PyPI this holds a PEP 508 environment marker.
	Environment AttrKey = 10

	// Selector is used in the context of resolved graphs and flags whether
	// the dependency is the selector of the concrete version.
	// In the case of NPM, this is set for all the edges that are not marked
	// as dedup in the install tree.
	// For Maven, this is set for all the edges that would appear in the
	// dependency tree.
	Selector AttrKey = 11
)
