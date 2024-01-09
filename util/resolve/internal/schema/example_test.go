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

package schema_test

import (
	"fmt"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/internal/schema"
)

// This schema declares three packages, each with versions that have
// imports.
func Example_schema() {
	const text = `

# Package alice has two versions, each with one dependency.
alice
	1.0.0
		# This version depends on bob
		# with the requirement ">=1.0.0".
		bob@>=1.0.0
	2.0.0
		bob@^1.0.0

# Package bob has one version that has two dependencies.
bob
	1.0.0
		# This version depends on bob/pkg
		# with the requirement "1.0.0"...
		bob/pkg@1.0.0
		# ... and optionally on cat with the
		# requirement "latest"
		Opt|cat@latest

# Package bob/pkg has one version that has one dependency.
bob/pkg
	1.0.0
		# This version depends on cat
		# with the requirement "main".
		cat@main

# Package cat has one concrete version.
# It has no dependencies.
cat
	# This version line declares a concrete version, just like
	# those in the other packages above.
	c0d3f4c3
`
	s, err := schema.New(text, resolve.NPM)
	if err != nil {
		panic(err)
	}

	fmt.Println(s.Package("alice").Version("2.0.0", resolve.Concrete).VersionKey)
	fmt.Println(s.Package("bob").Version("1.0.0", resolve.Concrete).VersionKey)
	fmt.Println(s.Package("bob").Version("1.0.0", resolve.Concrete).Requirements)
	fmt.Println(s.Package("bob/pkg").Version("1.0.0", resolve.Concrete).VersionKey)
	fmt.Println(s.Package("cat").Version("c0d3f4c3", resolve.Concrete).VersionKey)
	// Output:
	// NPM:alice[Concrete:2.0.0]
	// NPM:bob[Concrete:1.0.0]
	// [NPM:bob/pkg[Requirement:1.0.0] opt|NPM:cat[Requirement:latest]]
	// NPM:bob/pkg[Concrete:1.0.0]
	// NPM:cat[Concrete:c0d3f4c3]
}
