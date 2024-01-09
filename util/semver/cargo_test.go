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

package semver

// Cargo-specific tests.

import (
	"testing"
)

var cargoVersionParseTests = []versionParseTest{
	v("1", "", "1"),
	v("1.2", "", "1.2"),
	v("1.2-alpha", "", "1.2", "alpha"),

	// In builds, "", numbers can start with zero.
	v("1.2.3+build.01", "", "1.2.3", "+build.01"),

	// Pre-release and build metadata containing hyphens.
	v("1.2.3-a-b", "", "1.2.3", "a-b"),
	v("1.2.3+a-b", "", "1.2.3", "+a-b"),
	v("1.2.3-a-b.c+d.e-f", "", "1.2.3", "a-b", "c", "+d.e-f"),

	// Pre-release and build metadata consisting of a single hyphen;
	// pathological but valid.
	v("1.2.3--", "", "1.2.3", "-"),
	v("1.2.3+-", "", "1.2.3", "+-"),
	v("1.2.3--+-", "", "1.2.3", "-", "+-"),

	// Now some errors.
	v("1.2.3.beta1", "invalid text in version string in `1.2.3.beta1`", ""),
	v("2.0.0b5", "invalid text in version string in `2.0.0b5`", ""),
	v("1.0.0.0", "more than 3 numbers present in `1.0.0.0`", ""),
	v("1.rabbit", "non-numeric version in `1.rabbit`", ""),
	v("v1.2.3", "invalid version `v1.2.3`", ""),
	v("vvvvvvvvvvvv1.2.3", "invalid version `vvvvvvvvvvvv1.2.3`", ""),
	v("1.2.", "empty component in `1.2.`", ""),
	v("1.0.0 ", "invalid character ' ' in `1.0.0 `", ""),
	v("1.0.0+", "empty build metadata in `1.0.0+`", ""),
	v("1.0.0-", "empty pre-release metadata in `1.0.0-`", ""),
	v("1.0.0-.", "empty component in `1.0.0-.`", ""),
	v("1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`", ""),
	v("1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`", ""),
}

func TestCargoVersionParse(t *testing.T) {
	testVersionParse(t, Cargo, cargoVersionParseTests)
}

var cargoCanonTests = []canonTest{
	{"1", "1.0.0", ""},
	{"1.0", "1.0.0", ""},
	{"1.0.0", "1.0.0", ""},
	{"1.0.0+beta.2", "1.0.0+beta.2", "1.0.0"},
	{"1.0.0-alpha", "1.0.0-alpha", ""},
	{"1.0.0-alpha.1", "1.0.0-alpha.1", ""},
	{"1.0.0-alpha.1+beta.2", "1.0.0-alpha.1+beta.2", "1.0.0-alpha.1"},

	// Pre-release and build metadata containing hyphens.
	{"1.0.0-a-b", "1.0.0-a-b", "1.0.0-a-b"},
	{"1.0.0+a-b", "1.0.0+a-b", "1.0.0"},

	// Wildcards.
	{"*", "*", ""},
	{"1.*", "1.*", ""},
	{"1.0.*", "1.0.*", ""},
	{"1.*-alpha", "1.*", ""},
	{"1.*.0-alpha", "1.*", ""},
	{"1.*.0-alpha.1", "1.*", ""},
	{"1.0.x+build.01", "1.0.*", ""},
	{"1.0.X+build.01", "1.0.*", ""},
}

func TestCargoCanon(t *testing.T) {
	testVersionCanon(t, Cargo, cargoCanonTests)
}

var cargoConstraintErrorTests = []constraintErrorTest{
	{"☃", "invalid `☃` in `☃`"},
	{"1.rabbit", "non-numeric version in `1.rabbit`"},
	{"1..7", "empty component in `1..7`"},
	{"1.0.0.0", "more than 3 numbers present in `1.0.0.0`"},
	{"010", "number has leading zero in `010`"},
	{"1.2.", "empty component in `1.2.`"},
	{"1.0. 0", "empty component in `1.0.`"},
	{"1.0.0+", "empty build metadata in `1.0.0+`"},
	{"1.0.0-", "empty pre-release metadata in `1.0.0-`"},
	{"1.0.0-.", "empty component in `1.0.0-.`"},
	{"1.0.0-alpha..", "empty component in `1.0.0-alpha..`"},
	{"1.0.0-alpha..x", "empty component in `1.0.0-alpha..x`"},
	{"1.0.0-alpha.☃", "invalid text `☃` in `1.0.0-alpha.☃`"},
	{"1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`"},

	// Bad constraints.
	// This is legal as a constraint in Cargo.
	// {"1.2-pre", "prerelease requires 3 numbers: `1.2-pre`"},
	{"0.3.0 - 0", "invalid text `-` in `0.3.0 - 0`"},
	{",", "unexpected comma in `,`"},
	{"1.0 ||| 2.0", "invalid text `|` in `1.0 ||| 2.0`"},
	{",,", "unexpected comma in `,,`"},
	{", ,", "unexpected comma in `, ,`"},
	{"1.0,", "missing item after comma in `1.0,`"},
	{"3.0 1.0.0", "and list not supported in Cargo in `3.0 1.0.0`"},

	// Grammatical constructs valid only in some Systems.
	{"1.0.0 || 2.0.0", "invalid text `|` in `1.0.0 || 2.0.0`"},
	{"1.0.0 - 2.0.0", "invalid text `-` in `1.0.0 - 2.0.0`"},

	// Fixed bugs.
	{"^2 || beta", "invalid `|` in `^2 || beta`"},
}

func TestCargoConstraintError(t *testing.T) {
	testConstraintError(t, Cargo, cargoConstraintErrorTests)
}

func TestCargoSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Simple confidence checks of basic operators.
		{"1.2.3", "{[1.2.3:1.∞.∞]}"}, // Unadorned version is ^version.
		{"=1.2.3", "{1.2.3}"},
		{">1.2.3", "{[1.2.4:∞.∞.∞]}"},
		{">=1.2.3", "{[1.2.3:∞.∞.∞]}"},
		{"<1.2.3", "{[0.0.0-0:1.2.3)}"},
		{"<=1.2.3", "{[0.0.0-0:1.2.3]}"},
		{"^1", "{[1.0.0:1.∞.∞]}"},
		{"^1.2", "{[1.2.0:1.∞.∞]}"},
		{"^1.2.3", "{[1.2.3:1.∞.∞]}"},
		{"~1", "{[1.0.0:1.∞.∞]}"},
		{"~1.2", "{[1.2.0:1.2.∞]}"},
		{"~1.2-pre", "{[1.2.0-pre:1.2.∞-pre]}"},
		{"~1.2.3", "{[1.2.3:1.2.∞]}"},
		{"1.2.*", "{[1.2.0:1.2.∞]}"},
		{"1.*", "{[1.0.0:1.∞.∞]}"},
		{"*", "{[0.0.0-0:∞.∞.∞]}"},
		// Compound constructs: comma is and.
		{">=1.2.3,<=2.3.4", "{[1.2.3:2.3.4]}"},
	}
	for _, test := range tests {
		if !sameSet(Cargo, test.con, test.ref) {
			c, _ := Cargo.ParseConstraint(test.con)
			t.Errorf("Cargo set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

var cargoCompareTests = []compareTest{
	// Ordering of "pre-release versions".
	// Example from the semver.org Version 2.0.0 spec.
	{"1.0.0-alpha", "1.0.0-alpha.1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},

	// Zero-prefixed elements are not numbers, except in NPM, PyPI and RubyGems.
	{"1.0.0-rc.12", "1.0.0-rc.011", -1},
	{"0.0.1-alpha.006", "0.0.1-alpha.6", 1},

	// Hyphens are part of the identifier, not separators, but in RubyGems
	// hyphens are treated specially (they become ".pre.") and in PyPI they
	// are just separators.
	{"1.0.0-alpha", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha-1", "1.0.0-alpha-1.1", -1},

	// Numeric is below alphabetic except in RubyGems. PyPI is unusual.
	{"1.0.0-1", "1.0.0-alpha", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},

	// Build tags are ignored and Maven and RubyGems don't have them.
	// PyPI has something (locals) with the same syntax and sorts them after.
	{"6.0.0+build1", "6.0.0+build1", 0},
	{"6.0.0+build1", "6.0.0+build2", 0},
	{"6.0.0+build2", "6.0.0+build1", 0},
	{"6.0.0", "6.0.0+build1", 0},
	{"6.0.0+build2", "6.0.0-aaa", 1},
	{"6.0.0+build2", "6.0.0-zzz", 1},

	// Shortened versions imply zeros.
	{"1", "1.0", 0},
	{"1", "1.0.0", 0},
	{"1.0", "1.0.0", 0},
}

func TestCargoCompare(t *testing.T) {
	testCompare(t, Cargo, cargoCompareTests)
}
