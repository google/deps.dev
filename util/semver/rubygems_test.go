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

// RubyGems-specific tests.

import (
	"strings"
	"testing"
)

var rubyGemsVersionParseTests = []versionParseTest{
	v("1", "", "1"),
	v("1.2", "", "1.2"),
	v("1.2-alpha", "", "1.2", "alpha"),
	v("1.2.3--", "", "1.2.3", "-"),

	v("v1.2.3", "invalid version `v1.2.3`", ""),
	v("vvvvvvvvvvvv1.2.3", "invalid version `vvvvvvvvvvvv1.2.3`", ""),

	v("1.2.3.beta1", "", "1.2.3", "beta1"),
	v("2.0.0b5", "", "2.0.0", "b5"),
	v("010", "", "10"),
	v("1.0.0.0", "", "1.0.0.0"),
	v("1.rabbit", "", "1", "rabbit"),
	v("0.0.01", "", "0.0.1"),

	// Many numbers.
	v("6.5.4.3.2.1.beta1", "", "6.5.4.3.2.1", "beta1"),

	// Periods instead of - for versions...
	v("6.5.beta1", "", "6.5", "beta1"),

	// ...even with zeros.
	v("6.5.0beta1", "", "6.5.0", "beta1"),

	// Now some errors.

	// All should fail....
	v("1.2.", "empty pre-release metadata in `1.2.`", ""),
	v("1.0.0 ", "invalid character ' ' in `1.0.0 `", ""),
	v("1.0.0+", "invalid text in version string in `1.0.0+`", ""),
	v("1.0.0-", "empty pre-release metadata in `1.0.0-`", ""),
	v("1.0.0-.", "empty component in `1.0.0-.`", ""),
	v("1.*x.0-alpha.0", "empty pre-release metadata in `1.*x.0-alpha.0`", ""),
}

func TestRubyGemsVersionParse(t *testing.T) {
	testVersionParse(t, RubyGems, rubyGemsVersionParseTests)
}

var rubyGemsCanonTests = []canonTest{
	{"1", "1.0.0", ""},
	{"1.0", "1.0.0", ""},
	{"1.0.0", "1.0.0", ""},

	// Disallows "+", converts "-" to "", splits at boundary between alpha and num.
	{"1.0.0-a-b", "1.0.0-a.pre.b", ""},
	{"6.0.0.beta1", "6.0.0-beta.1", ""},
	{"6.0.0.beta1.gamma2", "6.0.0-beta.1.gamma.2", ""},
	// Many numbers.

	{"6.5.4.3.2.1-beta1", "6.5.4.3.2.1-beta.1", ""},

	// Was bug caused by misparsing.
	{"0.0.1a1.dev01573066581", "0.0.1-a.1.dev.01573066581", ""},
}

func TestRubyGemsCanon(t *testing.T) {
	testVersionCanon(t, RubyGems, rubyGemsCanonTests)
}

var rubyGemsConstraintErrorTests = []constraintErrorTest{
	{"☃", "invalid `☃` in `☃`"},
	{"1..7", "empty component in `1..7`"},
	{"1.2.", "empty pre-release metadata in `1.2.`"},
	{"1.0. 0", "empty pre-release metadata in `1.0.`"},
	{"1.0.0+", "invalid text `+` in `1.0.0+`"},
	{"1.0.0-.", "empty component in `1.0.0-.`"},
	{"1.0.0-alpha..", "empty component in `1.0.0-alpha..`"},
	{"1.0.0-alpha..x", "empty component in `1.0.0-alpha..x`"},
	{"1.0.0-alpha.☃", "invalid text `☃` in `1.0.0-alpha.☃`"},
	{"1.*x.0-alpha.0", "empty pre-release metadata in `1.*x.0-alpha.0`"},

	// Bad constraints.
	{",", "unexpected comma in `,`"},
	{"1.0 ||| 2.0", "invalid text `|` in `1.0 ||| 2.0`"},
	{",,", "unexpected comma in `,,`"},
	{", ,", "unexpected comma in `, ,`"},
	{"1.0,", "missing item after comma in `1.0,`"},

	// Grammatical constructs valid only in some Systems.
	{"1.0.0 || 2.0.0", "invalid text `|` in `1.0.0 || 2.0.0`"},
	{"1.0.0 - 2.0.0", "invalid text `-` in `1.0.0 - 2.0.0`"},
	{">=1.0.0 <2.0.0", "missing comma in RubyGems in `>=1.0.0 <2.0.0`"},

	// Fixed bugs.
	{"~>2 || beta", "invalid `|` in `~>2 || beta`"},
}

func TestRubyGemsConstraintError(t *testing.T) {
	testConstraintError(t, RubyGems, rubyGemsConstraintErrorTests)
}

func TestRubyGemsSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Simple confidence checks of basic operators.
		{"1.2.3.4.5", "{1.2.3.4.5}"},
		{"=1.2.3.4.5", "{1.2.3.4.5}"},
		{">1.2.3.4.5", "{(1.2.3.4.5:∞.∞.∞.∞.∞]}"},
		{">=1.2.3.4.5", "{[1.2.3.4.5:∞.∞.∞.∞.∞]}"},
		{"<1.2.3.4.5", "{[0.0.0-a:1.2.3.4.5)}"},
		{"<=1.2.3.4.5", "{[0.0.0-a:1.2.3.4.5]}"},
		{"~>1", "{[1.0.0:1.∞.∞]}"},
		{"~>1.2", "{[1.2.0:1.∞.∞]}"},
		{"~>1.2.3", "{[1.2.3:1.2.∞]}"},
		{"~>1.2.3.4", "{[1.2.3.4:1.2.3.∞]}"},
		{"~>1.2.3.4.5", "{[1.2.3.4.5:1.2.3.4.∞]}"},
		{"!=1.2.3", "{[0.0.0:1.2.3),(1.2.3:∞.∞.∞]}"},
	}
	for _, test := range tests {
		if !sameSet(RubyGems, test.con, test.ref) {
			c, _ := RubyGems.ParseConstraint(test.con)
			t.Errorf("RubyGems set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

// Borrowed example tests from
// https://github.com/rubygems/rubygems/blob/3.4/test/rubygems/test_gem_version.rb
var rubyGemsCompareTests = []compareTest{
	{"1.0", "1.0.0", 0},
	{"1.0", "1.0.a", 1},
	{"1.8.2", "0.0.0", 1},
	{"1.8.2", "1.8.2.a", 1},
	{"1.8.2.b", "1.8.2.a", 1},
	{"1.8.2.a", "1.8.2", -1},
	{"1.8.2.a10", "1.8.2.a9", 1},
	{"0.beta.1", "0.0.beta.1", 0},
	{"0.0.beta", "0.0.beta.1", -1},
	{"0.0.beta", "0.beta.1", -1},
	{"5.a", "5.0.0.rc2", -1},
	{"5.x", "5.0.0.rc2", 1},

	// Tests added by this package.
	{"1.2.3", "1.2.3.a", 1},
	{"1.2.3", "1.2.3.4.a", -1},
	{"5.1.0", "6.0.0.beta1", -1},
	{"6.0.0.beta1", "6.0.0.beta2", -1},
	{"6.0.0.beta2", "6.0.0", -1},

	// Numeric preleases compare as numbers, ignore leading zeros.
	{"1.0.0-rc.12", "1.0.0-rc.012", 0},
	{"1.0.0-rc.12", "1.0.0-rc.011", 1},
	{"0.0.1-alpha.006", "0.0.1-alpha.6", 0},

	// Numeric dominates alphabetic.
	{"1.0.0-1", "1.0.0.alpha", 1},
	{"1.0.0-alpha.1", "1.0.0.alpha.beta", 1},

	// Numeric is compared numerically.
	{"1.0.0-alpha.5", "1.0.0-alpha.10", -1},

	// A minus becomes an element ".pre."; very subtle consequences.
	{"1.0.0-alpha.5", "1.0.0.alpha.10", 1},

	// Many numbers.
	{"6.5.4.3.2", "6.5.4.3", 1},
	{"6.5.4.3.2", "6.5.4.3.1", 1},
	{"6.5.4.3.2", "6.5.4.3.2", 0},
	{"6.5.4.3.2", "6.5.4.3.3", -1},
	{"6.5.4.3.2", "6.5.4.3.3.1", -1},
}

func TestRubyGemsCompare(t *testing.T) {
	testCompare(t, RubyGems, rubyGemsCompareTests)
}

// TestRubyGemsCompareSequential uses an ordered list and does a full cross-check.
// The list comes from Semver.org version 2.0.0, although in RubyGems the ordering
// is slightly different (first two cases swapped due to different rules about prereleases.)
func TestRubyGemsCompareSequential(t *testing.T) {
	// Sorted in increasing precedence order, based on semver.org 2.0.0 example.
	tests := []string{
		"1.0.0-alpha.beta",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
		"1.1.0",
		"1.1.1-alpha",
		"1.1.1-beta",
		"1.1.1",
		"1.1.2",
		"1.2.0",
		"2.0.0",
	}
	testCompareSequential(t, RubyGems, tests)
}

var (
	rubyGemsUniverse = rubyGemsAll("")
)

// rubyGemsAll is a helper that turns all test versions, even those
// with prereleases, into an existence map.
func rubyGemsAll(prefix string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range rubyGemsTestVersions {
		if strings.HasPrefix(a, prefix) {
			m[a] = true
		}
	}
	return m
}

// Test that matching works with more than 3 numbers.
var rubyGemsMatchTests = []matchTest{
	// Empty.
	{R, "", rubyGemsUniverse}, // Special case: matches all versions without prerelease tags.
	{R, " ", rubyGemsUniverse},

	// Single versions match only themselves.
	{R, "1.2.3.4.5", m("1.2.3.4.5")},
	{R, "1.2.3.4", m("1.2.3.4")},
	{R, "1.2.3", m("1.2.3")},
	{R, "1.2", m("1.2")},
	{R, "1", m("1")},

	// Single versions with operands.
	{R, ">1.2.3", m("1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1 2.3.4.5.6 2.3.4.5.6-pre 2.3.4.5.7")},
	{R, ">=1.2.3", m("1.2.3 1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1 2.3.4.5.6 2.3.4.5.6-pre 2.3.4.5.7")},
	{R, ">=1.2.3.5.0", m("1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1 2.3.4.5.6 2.3.4.5.6-pre 2.3.4.5.7")},
	{R, "<1.2.2", m("1 1.2")},
	{R, ">1", m("1.2 1.2.3 1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1 2.3.4.5.6 2.3.4.5.6-pre 2.3.4.5.7")},

	// Twiddle-wakka, tilde-wakka, bacon-eater.
	{R, "~>1.2", m("1.2 1.2.3 1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1")},
	{R, "~>1.2.3", m("1.2.3 1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1")},
	{R, "~>1.2.3.4", m("1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1")},
	{R, "~>1.2.3.4.5", m("1.2.3.4.5 1.2.3.4.6")},

	// Commas.
	{R, ">=1.0.0, <2.0.0", rubyGemsAll("1")},
	{R, "2.3.4.5.6 , >2.0.0", m("2.3.4.5.6")},
	{R, ">=2.0.0, <2.3.4.5.7", m("2.3.4.5.6 2.3.4.5.6-pre")},

	// !=
	{R, ">1.2.3, !=2.3.4.5.6", m("1.2.3.4 1.2.3.4.5 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1 1.2.4.0.1 1.2.5.0.1 2.3.4.5.6-pre 2.3.4.5.7")},
	{R, "~>1.2.3.4, !=1.2.3.4.5", m("1.2.3.4 1.2.3.4.6 1.2.3.5.0 1.2.3.5.1")},

	// Partial match with prerelease.
	{R, ">=2.3.4.5.6-pre, <3.0.0", m("2.3.4.5.6-pre 2.3.4.5.6 2.3.4.5.7")},
	{R, ">2.3.4.5.6-p, <2.3.4.5.6-q", m("2.3.4.5.6-pre")},
}

func TestRubyGemsMatch(t *testing.T) {
	testMatch(t, false, rubyGemsMatchTests, rubyGemsTestVersions)
}

var rubyGemsTestVersions = strings.Fields(`
1
1.2
1.2.3
1.2.3.4
1.2.3.4.5
1.2.3.4.6
1.2.3.5.0
1.2.3.5.1
1.2.4.0.1
1.2.5.0.1
2.3.4.5.6
2.3.4.5.6-pre
2.3.4.5.7`)
