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
	"testing"
)

// Maven-specific tests.
// Maven 3.9.0 (or perhaps 3.8.7) changed the version parsing behaviour
// We're not targeting that yet.

var mavenCanonTests = []canonTest{
	{"1", "1", ""}, // Trailing zeros, null strings are trimmed.
	{"1.0", "1", ""},
	{"1.0.0", "1", ""},
	{"1.0.0-alpha", "1-alpha", ""},

	{"1.0.0.a1", "1.0.0.alpha-1", ""},
	{"1.0.0-a1", "1-alpha-1", ""},
	{"1.0.0-a.1", "1-a.1", ""},
	{"1.0.0-a-1", "1-a-1", ""},

	// Pre-release and build metadata containing hyphens.
	{"1.0.0-a-b", "1-a-b", ""},

	{"1.2.3", "1.2.3", ""},
	{"1.0.0", "1", ""},
	{"1.0", "1", ""},
	{"1.ga", "1", ""},
	{"1.final", "1", ""},
	{"1.", "1", ""},
	{"1.0.0-foo-0.0", "1-foo", ""},
	{"1.0.0-0.0.0", "1", ""},
	{"1-1.foo-bar1baz-.1", "1-1.foo-bar-1-baz-0.1", ""},

	// These change in Maven 3.9.0
	{"1.2.alpha.alpha", "1.2.alpha.alpha", ""},
	{"1.2.alpha.alpha.alpha", "1.2.alpha.alpha.alpha", ""},
	{"1.2.g.g", "1.2.g.g", ""},
	{"1.2.g.g.g.g", "1.2.g.g.g.g", ""},
	{"3.0.0.alpha.3.1.pre", "3.0.0.alpha.3.1.pre", ""},
	{"3.0.0.alpha.4.0", "3.0.0.alpha.4", ""},
	{"1.0.0.adf1.adf2", "1.0.0.adf-1.adf-2", ""},

	// Other odd examples.
	{"1....0-3", "1-3", ""},
	{"1.*-a", "1.*-a", ""},
	{"0.*_*.0", "0.*_*", ""},

	// Underscore is a "character" in Maven.
	{"1.2_3", "1.2-_-3", ""},
	{"1.2.abc_3", "1.2.abc_-3", ""},
	{"1.2.abc_def", "1.2.abc_def", ""},

	// Strings and numbers can be interleaved.
	{"RELEASE120", "release-120", ""},
	{"1.1.3-alpha.3.0+2022-05-16T16-21-58-758705Z", "1.1.3-alpha.3-+-2022-05-16-t-16-21-58-758705-z", ""},
	// But the special string abbreviations still apply when followed by a
	// number (with no separator).
	{"a", "a", ""},
	{"a0", "alpha", ""},
	{"a1", "alpha-1", ""},
	{"a-1", "a-1", ""},
}

func TestMavenCanon(t *testing.T) {
	testVersionCanon(t, Maven, mavenCanonTests)
}

var mavenConstraintErrorTests = []constraintErrorTest{
	{"[", "expected comma or closing bracket in `[`"},
	{"()", "hard requirement must be closed on both ends in `()`"},
	{")", "unexpected rbracket in `)`"},
	{"(1.0)", "hard requirement must be closed on both ends in `(1.0)`"},
	{"[1.0]]2.0]", "unexpected rbracket in `[1.0]]2.0]`"},
	{"[1.0][2.0]", "unexpected lbracket in `[1.0][2.0]`"},
}

func TestMavenConstraintError(t *testing.T) {
	testConstraintError(t, Maven, mavenConstraintErrorTests)
}

func TestMavenSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		// Examples from https://maven.apache.org/pom.html's section
		// "Dependency Version Requirement Specification".
		// TODO: Verify these against practice, as the specification is incomplete.
		// The TODOs below suggest some things to verify.
		{"1.0", "{[0:∞.∞.∞]}"}, // 1.0 is a soft constraint, matching anything.
		{"[1.0]", "{1}"},
		{"(,1.0]", "{[0:1]}"},
		{"[1.0,2.0)", "{[1:2)}"},
		{"[1.2,1.3]", "{[1.2:1.3]}"},
		{"[1.5,)", "{[1.5:∞.∞.∞]}"},
		{"(,1.0],[1.2,)", "{[0:1],[1.2:∞.∞.∞]}"},
		{"(,1.1),(1.1,)", "{[0:1.1),(1.1:∞.∞.∞]}"},
		// Other examples.
		{"[1.2],[1.4]", "{1.2,1.4}"},
		{"[1.2.a_4],[1.4]", "{1.2.a_-4,1.4}"},
	}
	for _, test := range tests {
		if !sameSet(Maven, test.con, test.ref) {
			c, _ := Maven.ParseConstraint(test.con)
			t.Errorf("Maven set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}

var mavenCompareTests = []compareTest{
	{"0", "1", -1},
	{"2", "10", -1},
	{"1", "1", 0},
	{"1", "1.1", -1},
	{"1-snapshot", "1", -1},
	{"1", "1-sp", -1},
	{"1-foo2", "1-foo10", -1},
	{"1-FOO2", "1-foo2", 0}, // Case insensitive.
	{"1-foo2", "1-FOO2", 0}, // Case insensitive.
	{"1.foo", "1-foo", -1},  // Changes in Maven 3.9.0
	{"1-foo", "1-1", -1},
	{"1-1", "1.1", -1},
	{"1-1", "1.1", -1},
	{"1.ga", "1-ga", 0},
	{"1-ga", "1-0", 0},
	{"1.release", "1-ga", 0},
	{"1-release", "1-0", 0},
	{"1.final", "1-ga", 0},
	{"1-final", "1-0", 0},
	{"1-0", "1.0", 0},
	{"1.0", "1", 0},
	{"1-sp", "1-ga", 1},
	{"1-sp.1", "1-ga.1", 1},
	{"1-sp-1", "1-ga-1", -1},
	{"1-ga-1", "1-1", 0},
	{"1-a1", "1-alpha-1", 0},
	{"1-a1", "1", -1},
	{"1-alpha.1", "1", -1},
	{"1.1.1-alpha", "1.1.1", -1},
	{"1.1.1", "1.1.1-final", 0},
	{"1.1.1-final", "1.1.1-ga", 0},

	// Also, shortened versions imply zeros.
	{"1", "1.0", 0},
	{"1", "1.0.0", 0},
	{"1.0", "1.0.0", 0},

	// Unrecognized qualifiers appear after known ones.
	{"2.14.jre7", "2.14", 1},
	{"2.14.jre7", "2.14.0", 1},
	{"2.14.jre7", "2.14.1", -1},

	// ... no matter where they are in the version.
	{"RELEASE90", "RELEASE120", -1},
	// The separator (or presence of a separator) matters.
	{"a1", "alpha-1", 0},
	{"a-1", "alpha-1", 1},
	{"a.1", "alpha-1", 1},
	{"aaaaaa", "alpha", 1},
}

func TestMavenCompare(t *testing.T) {
	testCompare(t, Maven, mavenCompareTests)
}

// TestMavenCompareSequential uses an ordered list and does a full cross-check.
// The list is similar to the on in TestCompareSequential, but tweaked for Maven.
func TestMavenCompareSequential(t *testing.T) {
	// Sorted in increasing precedence order by Maven's rules.
	tests := []string{
		"1.0.0-alpha.beta", // "beta" < "".
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
	testCompareSequential(t, Maven, tests)
}

func TestMavenIsSimple(t *testing.T) {
	tests := []struct {
		c      string
		simple bool
	}{
		{"1", true},
		{"1.2.3.4-alpha.pre", true},
		{"1,2", false},
		{"(1,2)", false},
		{"[1]", false},
	}
	for _, test := range tests {
		c, err := Maven.ParseConstraint(test.c)
		if err != nil {
			t.Fatal(err)
		}
		got := c.IsSimple()
		if got != test.simple {
			t.Errorf("Maven.IsSimple(%q) is %t; want %t", test.c, got, test.simple)
		}
	}
}

func TestMavenDifference(t *testing.T) {
	tests := []struct {
		u, v string
		cmp  int
		diff Diff
	}{
		{"1", "1", 0, Same},
		{"1", "1.0.0", 0, Same},
		{"1.2", "1.2", 0, Same},
		{"1.2.3", "1.2.3", 0, Same},
		{"2", "1", 1, DiffMajor},
		{"1.3", "1.2", 1, DiffMinor},
		{"1.2.4", "1.2.3", 1, DiffPatch},
		{"1.0.1", "1.0.0", 1, DiffPatch},
		{"1.a", "1.b", -1, DiffOther},
	}
	for _, test := range tests {
		cmp, diff, err := Maven.Difference(test.u, test.v)
		if err != nil {
			t.Fatal(err)
		}
		if cmp != test.cmp || diff != test.diff {
			t.Errorf("Difference(%q, %q) = (%d %s); want (%d %s)", test.u, test.v, cmp, diff, test.cmp, test.diff)
		}
	}
}
