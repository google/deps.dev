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

package version

//go:generate stringer -type AttrKey -output stringer.go

// AttrKey represents an attribute key that may be applied to an AttrSet.
//
// Its specific values are an implementation detail of this package;
// only use the named constants in client code.
type AttrKey int8

// The negative AttrKey values below are stored in a compact form
// and have special handling in version.go. The positive AttrKey values
// are serialized as varints.

// As guidance, consider that information encoded in an AttrSet is stored in
// the graph and is accessible from resolvers, so only that data which is
// necessary for graph maintenance or accurate resolution should be present.

const (
	// Use a 4 bit mask for special attributes.
	maskLen = 4

	// Blocked indicates the version is blocked or disabled for resolution.
	// Its value is ignored; its presence is the indicator.
	//
	// In Cargo, this is equivalent to a version being "yanked".
	Blocked AttrKey = -0x01

	// Deleted indicates the version has been deleted upstream.
	// Its value is ignored; its presence is the indicator.
	Deleted AttrKey = -0x02

	// Error indicates that the version has an error when it has been fed to
	// the system (e.g. parsing, abstracting, building effective POM...)
	// This is not a resolution error, that would be stored in the resolved
	// graph, but an error during the ingestion step.
	// Its value is ignored; its presence is the indicator. The exact error
	// can be retrieved through the ingestion services (e.g. pacman, gopher...)
	Error AttrKey = -0x04

	// -0x08 is reserved for future use.

	// The previous AttrKey are represented compactly in the encoded form.
	// Below here are AttrKey whose values are serialized.

	// Redirect indicates the version has been moved to a different version or package.
	//
	// In Maven, this is equivalent to a relocation version.
	Redirect AttrKey = 1

	// Features represent clusters of optional dependencies which
	// are opted into as a set.
	//
	// In Cargo, this is a JSON map from the feature name to a list
	// of enabled dependencies/other features.
	Features AttrKey = 2

	// DerivedFrom names another package from which this version is derived.
	// The presence of this attribute implies that this version is not a primary
	// form but is rather a derivative form in some context.
	//
	// NPM uses this to link a bundled dependency to its original package.
	DerivedFrom AttrKey = 3

	// NativeLibrary specifies what native library this version links against.
	NativeLibrary AttrKey = 4

	// Registries specifies the registries where the version can be found and
	// the registries in which the dependencies can be fetched.
	// In Maven, this is a comma separated list of registry IDs, and dependency
	// registries are prefixed with "dep:".
	Registries AttrKey = 5

	// SupportedFrameworks specifies what dotnet target frameworks this
	// versions supports as a colon-separated list. Each element is the raw
	// string received from upstream.
	SupportedFrameworks AttrKey = 6

	// DependencyGroups specifies what dotnet target framework dependencies
	// the package specifies as a colon-separated list. It includes
	// dependency groups that don't have any dependencies for that
	// framework.
	DependencyGroups AttrKey = 7

	// Ident is a 16-byte UUID that uniquely identifies this immutable
	// version.
	Ident AttrKey = 8

	// Created is the time the version was created as reported upstream. The
	// value is represented as a unix timestamp in seconds encoded as
	// varint.
	Created AttrKey = 9

	// Tags is a comma separated list of other names this version is known
	// as, such as "latest" in npm.
	Tags AttrKey = 10
)
