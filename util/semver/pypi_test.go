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

// PyPI-specific tests.

import (
	"strings"
	"testing"
)

var pypiVersionParseTests = []versionParseTest{
	// In builds, "", numbers can start with zero.
	v("1.2.3+build.01", "", "1.2.3", "+build.01"),
	v("1dev", "", "1", "dev0"),

	// Pre-release and build metadata containing hyphens.
	v("1.2.3+a-b", "", "1.2.3", "+a-b"),

	// PyPI allows one leading 'v'.
	v("v1.2.3", "", "1.2.3"),
	v("vvvvvvvvvvvv1.2.3", "invalid version `vvvvvvvvvvvv1.2.3`", ""),

	// Leading zeros.
	v("1.2.3.beta1", "", "1.2.3", "beta1"),
	v("010", "", "10"),
	v("1.0.0.0", "", "1.0.0.0"),

	// Many numbers.
	v("6.5.4.3.2.1.beta1", "", "6.5.4.3.2.1", "beta1"),

	// Now some errors.
	v("1.rabbit", "invalid text in version string in `1.rabbit`", ""),
	v("1.2.", "empty component in `1.2.`", ""),
	v("1.0.0+", "invalid text in version string in `1.0.0+`", ""),
	v("1.0.0-", "invalid text in version string in `1.0.0-`", ""),
	v("1.0.0-.", "invalid text in version string in `1.0.0-.`", ""),
	v("1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`", ""),
}

func TestPyPIVersionParse(t *testing.T) {
	testVersionParse(t, PyPI, pypiVersionParseTests)
}

// Tests for wildcards.
var pypiWildcardVersionParseTests = []versionParseTest{
	v("1.*", "", "1.*"),
	v("1.2.*", "", "1.2.*"),
	v("1.*-alpha", "", "1.*", "alpha"),
	v("1.*.3-alpha", "", "1.*", "alpha"),
	v("1.*.3-alpha.1", "", "1.*", "alpha", "1"),
}

func TestPyPIWildcardVersionParse(t *testing.T) {
	testVersionParse(t, PyPI, pypiWildcardVersionParseTests)
}

var pypiCanonTests = []canonTest{
	{"1", "1.0.0", ""},
	{"1.0", "1.0.0", ""},
	{"1.0.0", "1.0.0", ""},
	{"1.0.0+beta.2", "1.0.0+beta.2", ""}, // A local, not a build.
	{"1.0.0-alpha", "1.0.0a0", ""},
	{"1.0.0-alpha.1", "1.0.0a1", ""},
	{"1.0.0-alpha.1+beta.2", "1.0.0a1+beta.2", ""},
	{"3!1.0.0", "3!1.0.0", ""}, // Epoch
	{"1.0.0.dev.4", "1.0.0.dev4", ""},
	{"1.0.0alpha2+local3", "1.0.0a2+local3", ""},
}

func TestPyPICanon(t *testing.T) {
	testVersionCanon(t, PyPI, pypiCanonTests)
}

var pypiConstraintErrorTests = []constraintErrorTest{
	{"☃", "invalid `☃` in `☃`"},
	{"==1.rabbit", "invalid text in version string in `1.rabbit`"},
	{"==1..7", "empty component in `1..7`"},
	{"==1.2.", "empty component in `1.2.`"},
	{"==1.0. 0", "empty component in `1.0.`"},
	{"==1.0.0+", "invalid text in version string in `1.0.0+`"},
	{"==1.0.0-", "invalid text in version string in `1.0.0-`"},
	{"==1.0.0-.", "invalid text in version string in `1.0.0-.`"},
	{"==1.0.0-alpha..", "empty component in `1.0.0-alpha..`"},
	{"==1.0.0-alpha..x", "empty component in `1.0.0-alpha..x`"},
	{"==1.0.0-alpha.☃", "invalid `☃` in `==1.0.0-alpha.☃`"},
	{"1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`"},
	{"==*", "illegal wildcard as first component in `*`"},
	{"1", "missing operator in `1`"},
	{"==1.2.3-nonsense", "invalid text in version string in `1.2.3-nonsense`"},
	{"==apples", "invalid version `apples`"},

	// Bad constraints.
	{"==1.0 ||| ==2.0", "invalid `|` in `==1.0 ||| ==2.0`"},

	// Grammatical constructs valid only in some Systems.
	{"==1.0.0 - ==2.0.0", "invalid `-` in `==1.0.0 - ==2.0.0`"},

	// Fixed bugs - triggered panic in opVersionToSpan.
	{"~=x ", "no numbers in version `x`"},
}

func TestPyPIConstraintError(t *testing.T) {
	testConstraintError(t, PyPI, pypiConstraintErrorTests)
}

func TestPyPISets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Simple confidence checks of basic operators.
		{"", "{[0.0.0:∞.∞.∞]}"}, // Empty constraint matches anything.
		{"==1.2.3.4.5", "{1.2.3.4.5}"},
		{">1.2.3.4.5", "{(1.2.3.4.5:∞.∞.∞.∞.∞]}"},
		{">=1.2.3.4.5", "{[1.2.3.4.5:∞.∞.∞.∞.∞]}"},
		{"<1.2.3.4.5", "{[0.0.0.dev0:1.2.3.4.5)}"},
		{"<=1.2.3.4.5", "{[0.0.0.dev0:1.2.3.4.5]}"},
		{"~=1.2", "{[1.2.0:1.∞.∞]}"},
		{"~=1.2.3", "{[1.2.3:1.2.∞]}"},
		{"~=1.2.3.4", "{[1.2.3.4:1.2.3.∞]}"},
		{"~=1.2.3.4.5", "{[1.2.3.4.5:1.2.3.4.∞]}"},
		{"!=1.2.3", "{[0.0.0:1.2.3),(1.2.3:∞.∞.∞]}"},
		{"!=1.2.*", "{[0.0.0:1.2.0),(1.2.∞:∞.∞.∞]}"},

		// Variant and peculiar versions. Use == and check canonicalization.
		{"==00001", "{1.0.0}"},
		{"==00010", "{10.0.0}"},
		{"==1.0.0rc1", "{1.0.0rc1}"},
		{"==1.0.0RC1", "{1.0.0rc1}"},
		{"==1.0.0-C.1", "{1.0.0rc1}"},
		{"==1!1.0.0rc.1", "{1!1.0.0rc1}"},
		{"==1.0.0dev", "{1.0.0.dev0}"},
		{"==1.0.0Dev2", "{1.0.0.dev2}"},
		{"==1.0.0post", "{1.0.0.post0}"},
		{"==1.0.0r2", "{1.0.0.post2}"},
		{"==1.0.0rev3", "{1.0.0.post3}"},
	}
	for _, test := range tests {
		if !sameSet(PyPI, test.con, test.ref) {
			c, _ := PyPI.ParseConstraint(test.con)
			t.Errorf("PyPI set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

var pypiCompareTests = []compareTest{
	// Ordering of "pre-release versions".
	{"1.0.0-alpha", "1.0.0-alpha.1", -1},

	// Zero-prefixed elements are not numbers, except in NPM, PyPI and RubyGems.
	{"1.0.0-rc.12", "1.0.0-rc.011", 1},
	{"0.0.1-alpha.006", "0.0.1-alpha.6", 0},

	// In PyPI hyphens are just separators.
	{"1.0.0-alpha", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha-1", 0},

	// Numeric/alphabetic PyPI comparison is unusual.
	{"1.0.0-1", "1.0.0-alpha", 1},

	// PyPI uses + for locals and sorts them after.
	{"6.0.0+build1", "6.0.0+build1", 0},
	{"6.0.0+build1", "6.0.0+build2", -1},
	{"6.0.0+build2", "6.0.0+build1", 1},
	{"6.0.0", "6.0.0+build1", -1},
	{"6.0.0+build2", "6.0.0-a", 1},
	// . separated local sections are compared individually.
	{"6.0.0+build1", "6.0.0+build.1", 1},
	{"6.0.0+build.1", "6.0.0+build.2", -1},

	// Shortened versions imply zeros.
	{"1", "1.0", 0},
	{"1", "1.0.0", 0},
	{"1.0", "1.0.0", 0},
}

func TestPyPICompare(t *testing.T) {
	testCompare(t, PyPI, pypiCompareTests)
}

// TestPyPICompareSequential uses an ordered list and does a full cross-check.
// The list comes from the PEP440 document, with a few additions.
func TestPyPICompareSequential(t *testing.T) {
	// Sorted in increasing precedence order, based on semver.org 2.0.0 example.
	tests := []string{
		"1.0.dev456",
		"1.0a1",
		"1.0a2.dev456",
		"1.0a12.dev456",
		"1.0a12",
		"1.0b1.dev456",
		"1.0b2",
		"1.0b2.post345.dev456",
		"1.0b2.post345",
		"1.0b3", // Addition.
		"1.0rc", // Addition.
		"1.0rc1.dev456",
		"1.0rc1",
		"1.0rc.2", // Addition.
		"1.0",
		"1.0+abc.5",
		"1.0+abc.7",
		"1.0+5",
		"1.0.post456.dev34",
		"1.0.post456",
		"1.0.post567",
		"1.1.dev1",
	}
	testCompareSequential(t, PyPI, tests)
}

// Test among other things that matching works with more than 3 numbers.
var pypiMatchTests = []matchTest{
	{P, "==1.2.3.4", m("1.2.3.4")},
	{P, ">=2.3.3", m("2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "==2.3.4", m("2.3.4")},
	{P, "==2.3.4-rc.1", m("2.3.4-rc.1")},
	{P, "==2.3.4.post2", m("2.3.4.post2")},
	{P, "==2.3.4+local3", m("2.3.4+local3")},
	{P, "==2.3.4.dev1", m("2.3.4.dev1")}, // Only way to match the .dev is to be explicit.
	{P, ">2.3.4.post1", m("2.3.4.post2 2.3.4.5")},
	{P, ">2.3.4.post2", m("2.3.4.5")},
	{P, ">2.3.4.dev1", m("2.3.4 2.3.4.5")}, // Does not include post or pre for some reason.
	{P, ">2.3.4-rc.1", m("2.3.4 2.3.4.5")}, // Does not include post.
	{P, ">=2.3.4-rc.1", m("2.3.4 2.3.4-rc.1 2.3.4.post2 2.3.4.5")},
	{P, ">2.3.3.dev1", m("2.3.4 2.3.4.post2 2.3.4.5")},                        // Still omits pre
	{P, ">=2.3.4.dev1", m("2.3.4 2.3.4.dev1 2.3.4-rc.1 2.3.4.post2 2.3.4.5")}, // Includes pre
	{P, ">=2.3.4", m("2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "==1.2.3", m("1.2.3")},
	{P, "==1.2", m("1.2 1.2.0")},
	{P, "==1.*", m("1 1.2 1.2.0 1.2.3 1.2.3.4")},
	{P, "==2.3.*", m("2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "", m("1 1.2 1.2.0 1.2.3 1.2.3.4 2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "!=1.2.3", m("1 1.2 1.2.0 1.2.3.4 2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "!=1.2.*", m("1 2.3.4 2.3.4.post2 2.3.4.5")},
	{P, "!=1.*", m("2.3.4 2.3.4.post2 2.3.4.5")},
}

func TestPyPIMatch(t *testing.T) {
	testMatch(t, false, pypiMatchTests, pypiTestVersions)
}

var pypiTestVersions = strings.Fields(`
1
1.2
1.2.0
1.2.3
1.2.3.4
2.3.4
2.3.4.dev1
2.3.4-rc.1
2.3.4.post2
2.3.4+local3
2.3.4.5`)

func TestPyPIPostReleaseMatch(t *testing.T) {
	testMatch(t, false, []matchTest{
		{P, ">1", m("1.1 1.1.post1")},
		{P, ">1.0", m("1.1 1.1.post1")},
		{P, ">1.0.0", m("1.1 1.1.post1")},
		{P, ">1.0.post1", m("1.0.post2 1.1 1.1.post1")},
		{P, ">1.0,<=1.1", m("1.1")},
		{P, "<=1.1", m("1.0 1.0.post1 1.0.post2 1.1")},
		{P, "<1.1", m("1.0 1.0.post1 1.0.post2")},
		{P, ">1.0.post1,<1.1", m("1.0.post2")},
		{P, "==1.*", m("1.0 1.0.post1 1.0.post2 1.1 1.1.post1")},
		{P, "==1.0.post1", m("1.0.post1")},
		{P, "~=1.1", m("1.1 1.1.post1")},
	}, []string{
		"1.0",
		"1.0.post1",
		"1.0.post2",
		"1.1",
		"1.1.post1",
	})
}
