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

// A list of versions to test against is at the bottom of this file.

type matchTest struct {
	sys     int
	cs      string
	matches map[string]bool
}

// Created by looking at, among other things, evaluations at https://semver.npmjs.com/.
var matchTests = []matchTest{
	// Empty.
	{A ^ (P | R), "", universe}, // Special case: matches all versions without prerelease tags.
	{R, "", universeWithPre},    // RubyGems matches prereleases.
	{A ^ (P | R), " ", universe},
	{R, " ", universeWithPre},
	{D, "1 2", nothing}, // Was a bug caused by "" meaning everything.

	// Single versions. Cargo defaults to "^".
	{A ^ (C | P), "0.1.0", m("0.1.0")},
	{A ^ (C | P), "1.1.0", m("1.1.0")},
	{A ^ (C | P), "1.1.1", m("1.1.1")},
	{A ^ (C | P), "1.0.0", m("1.0.0")},
	{A ^ (C | P), "0.5.0-rc.1", m("0.5.0-rc.1")},

	// Single version for Cargo means ^Version.
	{C, "0.1.0", m("0.1.0")},
	{C, "1.1.0", m("1.1.0 1.1.1 1.2.0 1.2.1")},
	{C, "1.1.1", m("1.1.1 1.2.0 1.2.1")},
	{C, "1.0.0", m("1.0.0 1.0.1 1.0.2 1.1.0 1.1.1 1.2.0 1.2.1")},
	{C, "0.5.0-rc.1", m("0.5.0-rc.1 0.5.0 0.5.1 0.5.2")},
	{C, "3.0", m("3.0.0 3.0.1 3.1.0 3.2.0 3.10.0 3.10.1")},
	{C, "3", m("3.0.0 3.0.1 3.1.0 3.2.0 3.10.0 3.10.1")},

	// The equal operator. PyPI and RubyGems zero-pad when required (and Cargo defaults to ^). The others do not.
	{A ^ (C | P | R), "=3", all3},
	{A ^ (C | P | R), "=3.0", all("3.0")},
	{P, "==3", m("3.0.0")}, // Zero-fill.
	{R, "=3", m("3.0.0")},  // Zero-fill.
	{P, "==3.0", m("3.0.0")},

	// Spans.
	{D | N, "1.0.0 - 2.0.0", u(all1, m("2.0.0"))},
	{D | N, "4.1 - 4.3", m("4.1.0 4.2.0 4.2.1 4.3.0")},
	{D | N, "4.1 - 4.17.2", m("4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},

	// Single versions with operands.
	// PyPI zero-fills so get a different answer in comparisons with short version strings.

	// Equal.
	{C | D | N, "=0", all0},
	{R, "=0", m("0.0.0")},
	{P, "==0", m("0.0.0")},
	{C | D | N, "=1.0", all("1.0")},
	{R, "=1.0", m("1.0.0")},
	{P, "==1.0", m("1.0.0")},

	// Less, less or equal.
	{A, "<0", nothing},
	{A ^ (P | R), "<=0", all("0.")},
	{P | R, "<=0", m("0.0.0")},
	{A, "<=0.0.0", m("0.0.0")},
	{A, "<0.0.0", nothing},
	{A, "<0.2.1", m("0.0.0 0.1.0 0.2.0")},
	{A, "<=0.2.1", m("0.0.0 0.1.0 0.2.0 0.2.1")},
	{A ^ (P | R), "<0.5.0", m("0.0.0 0.1.0 0.2.0 0.2.1 0.2.2 0.3.0")}, // Compare with similar case in matchPrereleaseTests
	{R, "<0.5.0", m("0.0.0 0.1.0 0.2.0 0.2.1 0.2.2 0.3.0 0.5.0-rc.1")},
	{A ^ (P | R), "< 2.2", u(all0, all1, m("2.0.0 2.1.0 "))},
	{R, "< 2.2", u(allPre0, allPre1, m("2.0.0 2.0.0 2.0.0-rc 2.0.0-rc.1 2.0.0-rc.2 2.1.0"))},
	{A ^ (P | R), "<= 1.0.0-rc.2", u(all0, m("1.0.0-rc.1 1.0.0-rc.2"))},
	{R, "<= 1.0.0-rc.2", u(allPre0, m("1.0.0-rc.1 1.0.0-rc.2"))},
	{A ^ (P | R), "< 1.0.0-rc.2", u(all0, m("1.0.0-rc.1"))},
	{R, "< 1.0.0-rc.2", u(allPre0, m("1.0.0-rc.1"))},

	// Greater, greater or equal.
	// >3 means >3.anything, that is, 4.0.0 and above.
	{A ^ R, ">= 3.2", u(m("3.2.0 3.10.0 3.10.1"), all4)},
	{R, ">= 3.2", u(m("3.2.0 3.10.0 3.10.1"), allPre4)},
	{A ^ (P | R), ">3", all4},
	{R, ">3", u(m("3.0.1 3.1.0 3.2.0 3.10.0 3.10.1"), allPre4)},
	{A ^ (P | R), ">4", nothing},
	{P, ">4", m("4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2 4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
	{R, ">4", m("4.0.1 4.0.1-rc.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2 4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
	{A ^ R, ">=4", all4},
	{A ^ (P | R), ">2.0.0", u(m("2.1.0 2.2.0 2.2.1 2.4.0 2.4.1 2.4.2"), all3, all4)},
	{R, ">2.0.0", u(m("2.1.0 2.2.0 2.2.1 2.4.0 2.4.1 2.4.2"), all3, allPre4)},
	{A ^ (P | R), ">=2.0.0-rc.1", u(m("2.0.0-rc.1 2.0.0-rc.2"), all2, all3, all4)},
	{P | R, ">=2.0.0-rc.1", u(m("2.0.0-rc.1 2.0.0-rc.2"), all2, all3, allPre4)},
	{A ^ (P | R), ">2.0.0-rc.1", u(m("2.0.0-rc.2"), all2, all3, all4)},
	// Python turns off prereleases when the lower bound is >.
	{P, ">2.0.0-rc.1", u(all2, all3, all4)},
	{R, ">2.0.0-rc.1", u(m("2.0.0-rc.2"), all2, all3, allPre4)},

	// Caret.
	{D | C | N, "^1.0.0", m("1.0.0 1.0.1 1.0.2 1.1.0 1.1.1 1.2.0 1.2.1")},
	{D | C | N, "^1.1.0", m("1.1.0 1.1.1 1.2.0 1.2.1")},
	{D | C | N, "^0.2", all("0.2.")},
	{D | C | N, "^0.2.1", m("0.2.1 0.2.2")},
	{D | C | N, "^1", all("1.")},
	{D | C | N, "^0", all("0.")},
	{D | C | N, "^0.0", all("0.0.")},
	{D | C | N, "^0.0.0", m("0.0.0")},

	// Tilde.
	{D | C | N, "~0.2.1", m("0.2.1 0.2.2")},
	{D | C | N, "~4.17.1", m("4.17.1 4.17.2")},
	{D | C | N, "~4.17", m("4.17.0 4.17.1 4.17.2")},
	{D | C | N, "~0", all("0.")},

	// Twiddle-wakka, tilde-wakka, bacon-eater, compatible-with.
	{D | R, "~>2", m("2.0.0 2.1.0 2.2.0 2.2.1 2.4.0 2.4.1 2.4.2")},
	{D | R, "~>2.0", m("2.0.0 2.1.0 2.2.0 2.2.1 2.4.0 2.4.1 2.4.2")},
	{D | R, "~>1.1", m("1.1.0 1.1.1 1.2.0 1.2.1")},
	{D | R, "~>4.17.1", m("4.17.1 4.17.2")},
	// PyPI.
	{P, "~=2.0", m("2.0.0 2.1.0 2.2.0 2.2.1 2.4.0 2.4.1 2.4.2")},
	{P, "~=1.1", m("1.1.0 1.1.1 1.2.0 1.2.1")},
	{P, "~=4.17.1", m("4.17.1 4.17.2")},
	// NPM accepts ~> but it means the same as ~.
	{N, "~>1.1", m("1.1.0 1.1.1")},

	// Spaces, ors, commas, etc.
	{D | N | P, ">=2.0.0 <3.0.0", all("2.")},
	{D | N, "2.1.0 2.1.0", m("2.1.0")},
	{P, "==2.1.0 ==2.1.0", m("2.1.0")},
	{D | R, "2.1.0 , 2.1.0", m("2.1.0")},
	{P, "==2.1.0 , ==2.1.0", m("2.1.0")},
	{D | R, "2.1.0 , >=2.0.0", m("2.1.0")},
	{P, "==2.1.0 , >=2.0.0", m("2.1.0")},
	{D | N, "2.1.0  >=2.0.0", m("2.1.0")},
	{D | N, "2 2.4", m("2.4.0 2.4.1 2.4.2")}, // Tricky because 2 matches 2.1, 2.2 etc, not just 2.0.0.
	{D | N, "2.1.0 || >= 4.17.1", m("2.1.0 4.17.1 4.17.2")},
	{D | N, "0.5.0-rc.1 || 1.0.0-rc.2", m("0.5.0-rc.1 1.0.0-rc.2")},

	// Single wildcards.
	{D | C | N, "*", universe},
	{D | C | N, "1.*.3", all("1.")}, // The .3 is irrelevant.
	{D | C | N, "1.x.3", all("1.")}, // The .3 is irrelevant.
	{D | C | N, "1.1.*", m("1.1.0 1.1.1")},
	{D | C | N, "1.1.X", m("1.1.0 1.1.1")},

	// Wildcards with others. TODO: Which of these are OK with PyPI?
	{D | N, "* 1.0.0", m("1.0.0")},
	{D | N, "1.2.0", m("1.2.0")},
	{P, "==1.2.0", m("1.2.0")},
	{D | N, "4.* >4.17.1", m("4.17.2")},
	{D, "4.*, >4.17.1", m("4.17.2")},
	{D | N, ">=4.0.0 <4.17.9 4.17.*", m("4.17.0 4.17.1 4.17.2")},
	{D | N, "4.2.* || 4.17.*", m("4.2.0 4.2.1 4.17.0 4.17.1 4.17.2")},
	{D | N, "1.1.* || 1.2.*", m("1.1.0 1.1.1 1.2.0 1.2.1")},

	// Partial match with prerelease.
	{D | N | P, ">=2.0.0-rc <2.0.0", m("2.0.0-rc 2.0.0-rc.1 2.0.0-rc.2")},
	{D | N, ">2.0.0-rc <2.0.0", m("2.0.0-rc.1 2.0.0-rc.2")},
	{P, ">2.0.0-rc <2.0.0", m("")}, // Open lower bound disables prereleases for Python.

	// Fixed bugs.
	// Union did not check maxOpen for both pieces when maxes lined up.
	{D | N, "<1.0.0 || 1.0.0", u(all0, m("1.0.0"))},

	// PyPI uses == for equality.
	{P, "==0.1.0", m("0.1.0")},
	{P, "==1.1.0", m("1.1.0")},
	{P, "==1.1.1", m("1.1.1")},
	{P, "==1.0.0", m("1.0.0")},
	{P, "==0.5.0-rc.1", m("0.5.0-rc.1")},
	{P, "==3.0", m("3.0.0")},
	{U, "*", universe},
	{U, "*-*", universeWithPre},
	{U, "4.0.*", m("4.0.0 4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
	{U, "4.0.*-*", m("4.0.0-rc.1 4.0.0 4.0.1-rc.1 4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
}

var matchPrereleaseTests = []matchTest{
	{D | C | N, "*", universeWithPre},
	{A, ">=0.0.0", universeWithPre},
	{A ^ (C | P), "0.5.0-rc.1", m("0.5.0-rc.1")},
	{P, "==0.5.0-rc.1", m("0.5.0-rc.1")},
	{C, "0.5.0-rc.1", m("0.5.0-rc.1 0.5.0 0.5.1 0.5.2")}, // Default op for Cargo is "^".
	{A ^ (C | P), "1.0.0-rc.1", m("1.0.0-rc.1")},
	{P, "==1.0.0-rc.1", m("1.0.0-rc.1")},
	{C, "1.0.0-rc.1", m("1.0.0-rc.1 1.0.0-rc.2 1.0.0-rc.3 1.0.1 1.0.1-rc.1 1.0.0 1.0.1 1.0.2 1.1.0 1.1.1 1.2.0 1.2.1 ")},
	{N, "^1.0.0-0", m("1.0.0-rc.1 1.0.0-rc.2 1.0.0-rc.3 1.0.1-rc.1 1.0.0 1.0.1 1.0.2 1.1.0 1.1.1 1.2.0 1.2.1")},
	{A, "<0.5.0", m("0.0.0 0.1.0 0.2.0 0.2.1 0.2.2 0.3.0 0.5.0-rc.1")},
	{A, ">=4.0.0", m("4.0.0 4.0.1 4.0.1-rc.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
	{A, ">=4.0.1", m("4.0.1 4.1.0 4.2.0 4.2.1 4.3.0 4.4.0 4.17.0 4.17.1 4.17.2")},
}

// Matches that should fail. Unlike the positive matches, these are not all-encompassing.
var noMatchTests = []matchTest{
	{A ^ P, "1.0.0", m("2.0.0")},
	{N, "1.0.0", m("1.*")}, // Wildcard versions.
	{P, "==1.0.0", m("2.0.0")},
	{A ^ P, "1.0.0", m("2.0.0-rc.1")},
	{P, "==1.0.0", m("2.0.0-rc.1")},
	{D | C | N, "^1.0.0", m("2.0.0-rc.1")},
	{D | C | N, "^1.0.0", m("2.0.0")},
	{D | C | N, "^1.0.0-rc.2", m("2.0.0-rc.1")},
	{D | C | N, "^1.0.0-rc.2", m("2.0.0")},
	{D | R, "~>1.0.0-rc.2", m("2.0.0-rc.1")},
	{D | R, "~>1.0.0-rc.2", m("2.0.0")},
}

// Matches that should fail with MatchVersionPrerelease. Unlike the positive matches, these are not all-encompassing.
var noMatchPrereleaseTests = []matchTest{
	{D | C | N, "2.*", m("3.0.0-pre.0")},
	{N, "1.0.0", m("1.*")},
}

// m is a helper that turns a string of versions into an existence map.
func m(arg string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range strings.Fields(arg) {
		m[a] = true
	}
	return m
}

// u is a helper that returns the union of its arguments as a new map.
func u(arg ...map[string]bool) map[string]bool {
	m := make(map[string]bool)
	for _, a := range arg {
		for k, v := range a {
			m[k] = v
		}
	}
	return m
}

var (
	universe        = all("")
	universeWithPre = allWithPre("")
	nothing         map[string]bool

	all0    = all("0.")
	allPre0 = allWithPre("0.")
	all1    = all("1.")
	allPre1 = allWithPre("1.")
	all2    = all("2.")
	all3    = all("3.") // There are no pre-release 3's.
	all4    = all("4.")
	allPre4 = allWithPre("4.")
)

// all is a helper that turns all non-prerelease test versions with the prefix into an existence map.
func all(prefix string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range testVersions {
		if strings.HasPrefix(a, prefix) && !strings.Contains(a, "-") {
			m[a] = true
		}
	}
	return m
}

// allWithPre is like all but includes prereleases.
func allWithPre(prefix string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range testVersions {
		if strings.HasPrefix(a, prefix) {
			m[a] = true
		}
	}
	return m
}

func testMatch(t *testing.T, matchPrerelease bool, tests []matchTest, testVersions []string) {
	t.Helper()
	for _, test := range tests {
		// Make sure all outputs are in the list of versions. This validates
		// the test expectations: no typos.
		for expect := range test.matches {
			found := false
			for _, have := range testVersions {
				if expect == have {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%q test has unknown version %q", test.cs, expect)
			}
		}
		for x, sys := test.sys, DefaultSystem; x != 0; {
			x, sys = nextSystem(x)
			c := parseConstraint(t, sys, test.cs)
			// Make sure Match results line up with expectations.
			for _, vs := range testVersions {
				v, err := c.sys.Parse(vs)
				if err != nil {
					// Not all versions are legal in all systems. We skip
					// them here, but the goldens will notice if they're
					// missing incorrectly, and if the test fails we'll see
					// this message
					t.Log("bad version in testMatch " + vs)
					continue
				}
				var matched bool
				if matchPrerelease {
					matched = c.MatchVersionPrerelease(v)
				} else {
					matched = c.MatchVersion(v)
				}
				expect := test.matches[vs]
				if matched != expect {
					vp := ""
					if matchPrerelease {
						vp = "Prerelease"
					}
					t.Errorf("%s %q.MatchVersion%s(%q): got %t; expect %t", sys, test.cs, vp, vs, matched, expect)
				}
				// Also test the string Match version (which does not check
				// wildcards).
				if !matchPrerelease {
					matched = c.Match(vs)
					if matched != expect {
						t.Errorf("%s %q.Match(%q): got %t; expect %t", sys, test.cs, vs, matched, expect)
					}
				}

			}
		}
	}
}

func testNoMatch(t *testing.T, matchPrerelease bool, tests []matchTest, testVersions []string) {
	t.Helper()
	for _, test := range tests {
		for x, sys := test.sys, DefaultSystem; x != 0; {
			x, sys = nextSystem(x)
			c := parseConstraint(t, sys, test.cs)
			// Make sure Match results line up with expectations.
			for ms := range test.matches {
				mv, err := c.sys.Parse(ms)
				if err != nil {
					panic("bad version in testNoMatch " + ms)
				}
				if c.set.matchVersion(mv, matchPrerelease) {
					vp := ""
					if matchPrerelease {
						vp = "VersionPrerelease"
					}
					t.Errorf("%s %q.Match%s(%q) incorrectly matches", sys, test.cs, vp, ms)
				}
			}
		}
	}
}

func TestMatch(t *testing.T) {
	testMatch(t, false, matchTests, testVersions)
}

func TestMatchPrerelease(t *testing.T) {
	testMatch(t, true, matchPrereleaseTests, testVersions)
}

func TestNoMatch(t *testing.T) {
	testNoMatch(t, false, noMatchTests, testVersions)
}

func TestNoMatchPrelease(t *testing.T) {
	testNoMatch(t, true, noMatchPrereleaseTests, testVersions)
}

// These versions are valid in all systems run through TestMatch.
var testVersions = strings.Fields(`
0.0.0
0.1.0
0.2.0
0.2.1
0.2.2
0.3.0
0.5.0-rc.1
0.5.0
0.5.1
0.5.2
1.0.0-rc.1
1.0.0-rc.2
1.0.0-rc.3
1.0.1
1.0.1-rc.1
1.0.0
1.0.2
1.1.0
1.1.1
1.2.0
1.2.1
2.0.0-rc
2.0.0-rc.1
2.0.0-rc.2
2.0.0
2.1.0
2.2.0
2.2.1
2.4.0
2.4.1
2.4.2
3.0.0
3.0.1
3.1.0
3.2.0
3.10.0
3.10.1
4.0.0
4.0.0-rc.1
4.0.1
4.0.1-rc.1
4.1.0
4.2.0
4.2.1
4.3.0
4.4.0
4.17.0
4.17.1
4.17.2`)
