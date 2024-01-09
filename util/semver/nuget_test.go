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

import (
	"strings"
	"testing"
)

// NuGet-specific tests.

var nugetVersionParseTests = []versionParseTest{
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

	// Four numbers OK.
	v("1.2.3.4", "", "1.2.3.4"),
	// Five not.
	v("1.2.3.4.5", "more than 4 numbers present in `1.2.3.4.5`", ""),

	// Leading zeros OK.
	v("0.0.01", "", "0.0.1"),
	v("010", "", "10"),

	// Wildcards.
	v("*", "", "*"),
	v("*-*", "", "*", "*"),
	v("1.*", "", "1.*"),
	v("1.0.0-*", "", "1.0.0", "*"),
	v("1.0.0*-*", "", "1.0.0", "*"),
	v("1.0.0*-abc*", "", "1.0.0", "abc*"),

	// Now some errors.
	v("1.2.3.beta1", "invalid text in version string in `1.2.3.beta1`", ""),
	v("2.0.0b5", "invalid text in version string in `2.0.0b5`", ""),
	v("1.rabbit", "non-numeric version in `1.rabbit`", ""),
	v("v1.2.3", "invalid version `v1.2.3`", ""),
	v("vvvvvvvvvvvv1.2.3", "invalid version `vvvvvvvvvvvv1.2.3`", ""),
	v("1.2.", "empty component in `1.2.`", ""),
	v("1.0.0 ", "invalid character ' ' in `1.0.0 `", ""),
	v("1.0.0+", "empty build metadata in `1.0.0+`", ""),
	v("1.0.0-", "empty pre-release metadata in `1.0.0-`", ""),
	v("1.0.0-.", "empty component in `1.0.0-.`", ""),
	v("1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`", ""),
	v("1.*9*", "invalid text in version string in `1.*9*`", ""),
	v("1.*.1", "wildcard in middle of version in `1.*.1`", ""),
	v("1.*.*", "version has multiple wildcards in `1.*.*`", ""),
	v("1.0.0*-alpha", "missing asterisk at end of prerelease in `1.0.0*-alpha`", ""),
	v("1.0.0-a*a*", "invalid text in version string in `1.0.0-a*a*`", ""),
}

func TestNuGetVersionParse(t *testing.T) {
	testVersionParse(t, NuGet, nugetVersionParseTests)
}

var nugetCanonTests = []canonTest{
	{"1", "1.0.0", ""},
	{"1.0", "1.0.0", ""},
	{"1.0.0", "1.0.0", ""},
	{"1.00.0", "1.0.0", ""}, // Leading zeros OK.
	{"1.001.0", "1.1.0", ""},
	{"1.001.0.0", "1.1.0", ""},   // A fourth zero is allowed but removed.
	{"1.001.0.1", "1.1.0.1", ""}, // A fourth non-zero is kept.
	{"1.0.0+beta.2", "1.0.0", "1.0.0"},
	{"1.0.0-alpha", "1.0.0-alpha", ""},
	{"1.0.0-alpha.1", "1.0.0-alpha.1", ""},
	{"1.0.0-AlPhA.1", "1.0.0-alpha.1", "1.0.0-alpha.1"}, // The version should be lowercased.

	// Pre-release, build metadata, and build metadata containing hyphens.
	{"1.0.0-a-b", "1.0.0-a-b", ""},
	{"1.0.0+a-b", "1.0.0", "1.0.0"},
	{"1.0.0-alpha.1+beta.2", "1.0.0-alpha.1", "1.0.0-alpha.1"},

	// Wildcards.
	{"*", "*", ""},
	{"1.*", "1.*", ""},
	{"1.0.*", "1.0.*", ""},
	{"1.*-alpha", "1.*", ""},
}

func TestNuGetCanon(t *testing.T) {
	testVersionCanon(t, NuGet, nugetCanonTests)
}

var nugetConstraintErrorTests = []constraintErrorTest{
	{"", "invalid empty constraint"},
	{"1.0.0 , 2.0.0", "cannot have more than one range in `1.0.0 , 2.0.0`"},
	{"[1.0,2.0],[3.0,4.0]", "cannot have more than one range in `[1.0,2.0],[3.0,4.0]`"},
	{"1.0.0  2.0.0", "unexpected version in `1.0.0  2.0.0`"},
	{"[", "expected comma or closing bracket in `[`"},
	{"()", "hard requirement must be closed on both ends in `()`"},
	{")", "unexpected rbracket in `)`"},
	{"(1.0)", "hard requirement must be closed on both ends in `(1.0)`"},
	{"[1.0]]2.0]", "unexpected rbracket in `[1.0]]2.0]`"},
	{"[1.0][2.0]", "unexpected lbracket in `[1.0][2.0]`"},
}

func TestNuGetConstraintError(t *testing.T) {
	testConstraintError(t, NuGet, nugetConstraintErrorTests)
}

func TestNuGetSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Examples from https://maven.apache.org/pom.html's section,
		// which are Maven but have the same form as Nuget but sometimes
		// different results because of the meaning of a single version and
		// wilcards are accepted. NuGet docs are at
		// https://docs.microsoft.com/en-us/nuget/concepts/package-versioning

		// Floating part tests which don't include prereleases.
		{"*", "{[0.0.0-0:∞.∞.∞)}"},
		{"1", "{[1.0.0:∞.∞.∞]}"},
		{"1.*", "{[1.0.0:∞.∞.∞)}"},
		{"1.1.*", "{[1.1.0:∞.∞.∞)}"},
		{"1.0.0*", "{[1.0.0:∞.∞.∞]}"},

		// Floating part tests which do include prereleases.
		{"*-*", "{[0.0.0-0:∞.∞.∞)}"},
		{"1.*-*", "{[1.0.0-0:∞.∞.∞)}"},
		{"1.*-ab*", "{[1.0.0-ab:∞.∞.∞)}"},
		{"1.0.0*-ab*", "{[1.0.0-ab:∞.∞.∞)}"},
		{"1.2.0-*", "{[1.2.0-0:∞.∞.∞)}"},
		{"1.0-abac*", "{[1.0.0-abac:∞.∞.∞)}"},
		{"1.0.0-a.a*", "{[1.0.0-a.a:∞.∞.∞)}"},

		{"[1.0]", "{1.0.0}"},
		{"(,1.0]", "{[0.0.0:1.0.0]}"},
		{"[1.0,2.0)", "{[1.0.0:2.0.0)}"},
		{"[1.2,1.3]", "{[1.2.0:1.3.0]}"},
		{"[1.5,)", "{[1.5.0:∞.∞.∞]}"},
	}
	for _, test := range tests {
		if !sameSet(NuGet, test.con, test.ref) {
			c, _ := NuGet.ParseConstraint(test.con)
			t.Errorf("NuGet set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

var nugetCompareTests = []compareTest{
	// Ordering of "pre-release versions".
	// Example from the semver.org Version 2.0.0 spec.
	{"1.0.0-alpha", "1.0.0-alpha.1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
	// pre releases are case insensitive
	{"1.0.0-alpha", "1.0.0-ALPHA", 0},
	{"1.0.0-alpha", "1.0.0-ALPHA.1", -1},

	{"1.0.0-I", "1.0.0-i", 0},
	{"1.0.0-I", "1.0.0-iI", -1},

	// Zero-prefixed elements are not numbers.
	{"1.0.0-rc.12", "1.0.0-rc.011", -1},
	{"0.0.1-alpha.006", "0.0.1-alpha.6", 1},

	// Hyphens are part of the identifier, not separators.
	{"1.0.0-alpha", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha-1", "1.0.0-alpha-1.1", -1},

	// Numeric is below alphabetic.
	{"1.0.0-1", "1.0.0-alpha", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},

	// Build tags are ignored.
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

func TestNuGetCompare(t *testing.T) {
	testCompare(t, NuGet, nugetCompareTests)
}

// nuGetAll is a helper that turns all non-prerelease NuGet test versions with the prefix into an existence map.
func nuGetAll(prefix string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range nuGetTestVersions {
		if strings.HasPrefix(a, prefix) && !strings.Contains(a, "-") {
			m[a] = true
		}
	}
	return m
}

// Test among other things that matching works with more than 3 numbers.
var nuGetMatchTests = []matchTest{
	// Bare versions match >= themselves and include prereleases.
	{U, "1.2.3.4", m("1.2.3.4 2.3.4-pre 2.3.4.5")},
	{U, "1.2.3", m("1.2.3 1.2.3.4 2.3.4-pre 2.3.4.5")},
	{U, "1.2", m("1.2 1.2.0 1.2.3 1.2.3.4 2.3.4-pre 2.3.4.5")},
	{U, "1.*", m("1 1.2 1.2.0 1.2.3 1.2.3.4 2.3.4.5")},
	{U, "2.0.0-*", m("2.3.4-pre 2.3.4.5")},

	// Single versions match only themselves.
	{U, "[1.2.3.4]", m("1.2.3.4")},
	{U, "[1.2.3]", m("1.2.3")},   // Does not match 1.2.3.4.
	{U, "[1.2]", m("1.2 1.2.0")}, // Does not match 1.2.3, etc. but does match 1.2.0.
	{U, "[1]", m("1")},
}

func TestNuGetMatch(t *testing.T) {
	testMatch(t, false, nuGetMatchTests, nuGetTestVersions)
}

var nuGetTestVersions = strings.Fields(`
1
1.2
1.2.0
1.2.3
1.2.3.4
2.3.4-pre
2.3.4.5`)
