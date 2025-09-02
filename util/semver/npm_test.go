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

// NPM-specific tests.

import (
	"testing"
)

var defaultVersionVersionParseTests = []versionParseTest{
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
	v("1.2.", "empty component in `1.2.`", ""),
	v("1.0.0 ", "invalid character ' ' in `1.0.0 `", ""),
	v("1.0.0+", "empty build metadata in `1.0.0+`", ""),
	v("1.0.0-", "empty pre-release metadata in `1.0.0-`", ""),
	v("1.0.0-.", "empty component in `1.0.0-.`", ""),
	v("1.*x.0-alpha.0", "invalid text in version string in `1.*x.0-alpha.0`", ""),
}

var npmVersionParseTests = []versionParseTest{
	// NPM allows leading 'v', lots of leading 'v's.
	v("v1.2.3", "", "1.2.3"),
	v("vvvvvvvvvvvv1.2.3", "", "1.2.3"),
	v("v*", "", "*"),
}

func TestNPMVersionParse(t *testing.T) {
	testVersionParse(t, NPM, defaultVersionVersionParseTests)
	testVersionParse(t, NPM, npmVersionParseTests)
}

func TestDefaultVersionParse(t *testing.T) {
	testVersionParse(t, DefaultSystem, defaultVersionVersionParseTests)
}

var npmCanonTests = []canonTest{
	{"1", "1.0.0", ""},
	{"1.0", "1.0.0", ""},
	{"1.0.0", "1.0.0", ""},
	{"1.0.0+beta.2", "1.0.0+beta.2", "1.0.0"},
	{"1.0.0-alpha", "1.0.0-alpha", ""},
	{"1.0.0-alpha.1", "1.0.0-alpha.1", ""},
	{"1.0.0-alpha.1+beta.2", "1.0.0-alpha.1+beta.2", "1.0.0-alpha.1"},

	// Pre-release and build metadata containing hyphens.
	{"1.0.0-a-b", "1.0.0-a-b", ""},
	{"1.0.0+a-b", "1.0.0+a-b", "1.0.0"},

	// Wildcards.
	{"*", "*", ""},
	{"1.*", "1.*", ""},
	{"1.0.*", "1.0.*", ""},
	{"1.*-alpha", "1.*", ""},
	{"1.*.0-alpha", "1.*", ""},
	{"1.*.0-alpha.1", "1.*", ""},
	{"1.0.x+build.01", "1.0.*", "1.0.*"},
	{"1.0.X+build.01", "1.0.*", "1.0.*"},

	{"v6.0.0-beta1", "6.0.0-beta1", ""},
	{"vv6.0.0-beta1+build2", "6.0.0-beta1+build2", "6.0.0-beta1"},
	{"vvv6.x.0-beta1", "6.*", ""},
}

func TestNPMCanon(t *testing.T) {
	testVersionCanon(t, NPM, npmCanonTests)
}

var npmConstraintErrorTests = []constraintErrorTest{
	{"☃", "invalid `☃` in `☃`"},
	{"1.rabbit", "non-numeric version in `1.rabbit`"},
	{"1..7", "empty component in `1..7`"},
	{"1.0.0.0", "more than 3 numbers present in `1.0.0.0`"},
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
	{"1.2-pre", "prerelease requires 3 numbers: `1.2-pre`"}, // R zero-fill.
	{"0.3.0 - 0", "impossible constraint: max greater than min in `0.3.0 - 0`"},
	{"||", "unexpected or in `||`"},
	{"1.0 ||| 2.0", "invalid `|` in `1.0 ||| 2.0`"},
	{"|| ||", "unexpected or in `|| ||`"},
	{"1.0,", "invalid text `,` in `1.0,`"},
	{"1.0||", "missing item after or in `1.0||`"},
	{"^1.2.3.4.5", "more than 3 numbers present in `1.2.3.4.5`"},
	{"1.0.0 - 4.2.0 3.0", "unexpected version in `1.0.0 - 4.2.0 3.0`"},
	{"3.0 1.0.0 - 4.2.0", "unexpected range after version in `3.0 1.0.0 - 4.2.0`"},

	// Fixed bugs.
	{"^2 || beta", "invalid version `beta`"},
}

func TestNPMConstraintError(t *testing.T) {
	testConstraintError(t, NPM, npmConstraintErrorTests)
}

func TestNPMSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Simple confidence checks of basic operators.
		{"1.2.3", "{1.2.3}"},
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
		{"~1.2.3", "{[1.2.3:1.2.∞]}"},
		{"1.2.*", "{[1.2.0:1.2.∞]}"},
		{"1.*", "{[1.0.0:1.∞.∞]}"},
		{"*", "{[0.0.0-0:∞.∞.∞]}"},
		// Compound constructs: and, or, range.
		{">=1.2.3 <=2.3.4", "{[1.2.3:2.3.4]}"},
		{"1.2.3 || 2.3.4", "{1.2.3,2.3.4}"},
		{"1.2.3 - 2.3.4", "{[1.2.3:2.3.4]}"},
		{">=1.2.3 <4.5.0 || 9.2.3 - 9.4.5", "{[1.2.3:4.5.0),[9.2.3:9.4.5]}"},
		{"1 - 2 || 3.8.1", "{[1.0.0:2.∞.∞],3.8.1}"},
	}
	for _, test := range tests {
		if !sameSet(NPM, test.con, test.ref) {
			c, _ := NPM.ParseConstraint(test.con)
			t.Errorf("NPM set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

var npmCompareTests = []compareTest{
	// Ordering of "pre-release versions".
	// Example from the semver.org Version 2.0.0 spec.
	{"1.0.0-alpha", "1.0.0-alpha.1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},

	// Zero-prefixed elements are numbers in NPM, PyPI and RubyGems.
	{"1.0.0-rc.12", "1.0.0-rc.011", 1},
	{"0.0.1-alpha.006", "0.0.1-alpha.6", 0},
	{"2001.1001.0000-dev-harmony-fb", "2001.1001.0-dev-harmony-fb", 0},

	// Hyphens are part of the identifier, not separators.
	{"1.0.0-alpha", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha-1", -1},
	{"1.0.0-alpha-1", "1.0.0-alpha-1.1", -1},

	// Numeric is below alphabetic.
	{"1.0.0-1", "1.0.0-alpha", -1},
	{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
	{"v6.0.0-beta2", "6.0.0", -1},
	{"1.0.0-1", "v1.0.0-alpha", -1},
	{"v1.0.0-alpha.1", "1.0.0-alpha.beta", -1},

	// Numeric is compared numerically.
	{"1.0.0-alpha.5", "v1.0.0-alpha.10", -1},

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

	// NPM allows but ignores v.
	{"5.1.0", "v6.0.0-beta1", -1},
	{"6.0.0-beta1", "v6.0.0-beta2", -1},
}

func TestNPMCompare(t *testing.T) {
	testCompare(t, NPM, npmCompareTests)
}

func TestCalculateMinVersion(t *testing.T) {
	tests := []struct {
		c    string
		want string
	}{
		{"*", "0.0.0"},
		{"", "0.0.0"},
		{"1.0.0", "1.0.0"},
		{">=1.0.0", "1.0.0"},
		{">1.0.0", "1.0.1"},
		{"<1.0.0", "0.0.0"},
		{"<=1.0.0", "0.0.0"},
		{">1.0.0 <2.0.0", "1.0.1"},
		{">=1.0.0 >=2.0.0 <3.0.0", "2.0.0"},
		{">=1.0.0 <2.0.0 <3.0.0", "1.0.0"},
		{">=2.0.0-alpha", "2.0.0-alpha"},
		{">2.0.0-alpha", "2.0.0-alpha.0"},
		{">2.0.0-alpha.0", "2.0.0-alpha.0.0"},
		{">2.0.0-alpha.-2", "2.0.0-alpha.-2.0"},
		{">2.0.0--", "2.0.0--.0"},
		{">=2.0.0-alpha.0+build.1", "2.0.0-alpha.0+build.1"},
		{">2.0.0-alpha.0+build.1", "2.0.0-alpha.0.0"},
		{"1.0.0 - 2.0.0", "1.0.0"},
		{"1.1 - 2.0.0", "1.1.0"},
		{"3.x", "3.0.0"},
		{"3.1.x", "3.1.0"},
		{"~1.2.3", "1.2.3"},
		{"~1.2", "1.2.0"},
		{"~1", "1.0.0"},
		{"^1.2.3", "1.2.3"},
		{"^0.2.3", "0.2.3"},
		{"^0.0.3", "0.0.3"},
		{"1.0.0 || 2.0.0", "1.0.0"},
		{">1.0.0 || 2.0.0", "1.0.1"},
		{">=3.0.0 || 2.0.0", "2.0.0"},
		{">=3.0.0 || <2.0.0", "0.0.0"},
		{">=3.0.0 <4.0.0 || >5.0.0", "3.0.0"},
		// This test is purposely complex, to make sure that we correctly sort and overlap ranges.
		// The final value of 0.0.5 comes from the `>0.0.4` clause.
		{">5.0.0 || <2.0.0 >0.4.0 <1.0.0 >0.5.0 <3.0.0 >0.3.0 || <9.0.0 >0.0.3 <8.0.0 >0.0.2 <7.0.0 >0.0.4 || <9.0.0 >1.0.0 <8.0.0 >2.0.0 <7.0.0 >3.0.0", "0.0.5"},
	}
	for _, test := range tests {
		t.Run(test.c, func(t *testing.T) {
			c := parseConstraint(t, NPM, test.c)
			m, err := c.CalculateMinVersion()
			if err != nil {
				t.Fatal(err)
			}
			wantV := parseVersion(t, NPM, test.want)
			if !m.equal(wantV) {
				t.Errorf("calculateMinVersion(%q) = %q; want %q", test.c, m.String(), wantV.String())
			}
		})
	}
}
