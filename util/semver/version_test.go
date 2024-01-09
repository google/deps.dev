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
	"fmt"
	"math/bits"
	"strings"
	"testing"
)

const (
	C = 1 << Cargo
	D = 1 << DefaultSystem
	G = 1 << Go
	M = 1 << Maven
	N = 1 << NPM
	P = 1 << PyPI
	R = 1 << RubyGems
	U = 1 << NuGet
	// All but Maven and NuGet (they use set-style constraints) and Go, which
	// doesn't have constraints at all.
	A = D | C | N | P | R
)

var sysMap = map[int]System{
	C: Cargo,
	D: DefaultSystem,
	G: Go,
	M: Maven,
	N: NPM,
	P: PyPI,
	R: RubyGems,
	U: NuGet,
}

var (
	allSystems = []System{DefaultSystem, Cargo, Go, Maven, NPM, NuGet, PyPI, RubyGems}
)

// nextSystem returns the next System defined by the bitmask x.
// x must be non-zero. It returns the updated value of x with one
// bit cleared.
func nextSystem(x int) (int, System) {
	if x == 0 {
		panic("no bits set in nextSystem")
	}
	bit := 1 << uint(bits.TrailingZeros(uint(x)))
	x &^= bit
	sys, ok := sysMap[bit]
	if !ok {
		panic(fmt.Sprintf("bad system bit %#x in test", bit))
	}
	return x, sys
}

func (v *Version) userNums() string {
	var b strings.Builder
	v.printNumsN(&b, int(v.userNumCount))
	return b.String()
}

func parseVersion(t *testing.T, sys System, str string) *Version {
	t.Helper()
	v, err := sys.Parse(str)
	if err != nil {
		t.Fatalf("%q: %v", str, err)
	}
	return v
}

type versionParseTest struct {
	// The string to parse. It might not be a valid Version string.
	str    string
	err    string // If non-empty, the error to expect.
	numStr string // The numerical portion of the parsed result; * is a wild card.
	// meta is used to construct the pre and build fields; if an argument begins
	// with '+', it is used for the build field, and must be last.
	meta []string
}

// v is a helper to make it easier to construct versionParseTests.
func v(str, err, numStr string, meta ...string) versionParseTest {
	return versionParseTest{
		str,
		err,
		numStr,
		meta,
	}
}

// All systems except Maven and Go should pass all these versions, or all fail.
// Maven version 2 at least is just too different.
// All the funny business happens elsewhere, in system-specific tests.
var basicVersionParseTests = []versionParseTest{
	v("1.2.3", "", "1.2.3"),
	v("1.2.3-alpha", "", "1.2.3", "alpha"),
	v("1.2.3-alpha.1", "", "1.2.3", "alpha", "1"),
	v("1.2.3-beta.01", "", "1.2.3", "beta", "01"), // The 01 is legal, but not a "number".

	// All should fail....
	v("", "invalid version ``", ""),
	v("☃", "invalid version `☃`", ""),
	v("1..7", "empty component in `1..7`", ""),
	v("1.0. 0", "invalid character ' ' in `1.0. 0`", ""),
	v("1.0.0-alpha..", "empty component in `1.0.0-alpha..`", ""),
	v("1.0.0-alpha..x", "empty component in `1.0.0-alpha..x`", ""),
	v("1.0.0-alpha.☃", "invalid character '☃' in `1.0.0-alpha.☃`", ""),

	// Very large value.
	v("1.2.20181231235959", "", "1.2.20181231235959"),

	// Values too large to represent.
	v("1.2.9223372036854775807", "number out of range: 9223372036854775807", ""),                              // 2^63-1
	v("1.2.9223372036854775808", `strconv.ParseInt: parsing "9223372036854775808": value out of range`, ""),   // 2^63
	v("1.2.18446744073709551615", `strconv.ParseInt: parsing "18446744073709551615": value out of range`, ""), // 2^64-1
	v("1.2.18446744073709551616", `strconv.ParseInt: parsing "18446744073709551616": value out of range`, ""), // 2^64
}

// Tests for wildcards.
var wildcardVersionParseTests = []versionParseTest{
	v("*", "", "*"),
	v("x", "", "*"),
	v("X", "", "*"),
	v("1.*", "", "1.*"),
	v("1.2.*", "", "1.2.*"),
	v("1.2.X", "", "1.2.*"),
	v("1.*-alpha", "", "1.*", "alpha"),
	v("1.*.3-alpha", "", "1.*", "alpha"),
	v("1.x.3-alpha", "", "1.*", "alpha"),
	v("1.*.3-alpha.1", "", "1.*", "alpha", "1"),
}

func testVersionParse(t *testing.T, sys System, tests []versionParseTest) {
Outer:
	for _, test := range tests {
		v, err := sys.Parse(test.str)
		// Do we expect an error?
		if test.err != "" {
			if err == nil {
				t.Errorf("%s.Parse(%q): expected error %q; got nil", sys, test.str, test.err)
			} else if err.Error() != test.err {
				t.Errorf("%s.Parse(%q): expected error %q; got %q", sys, test.str, test.err, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s.Parse(%q): %v", sys, test.str, err)
			continue
		}

		if v.sys != sys {
			t.Errorf("%s.Parse(%q): got system %#q", sys, test.str, v.sys)
			continue
		}
		if v.str != test.str {
			t.Errorf("%s.Parse(%q): got string %q", sys, test.str, v.str)
			continue
		}

		numStr := v.userNums()
		if numStr != test.numStr {
			t.Errorf("%s.Parse(%q): got %s for numbers; want %s", sys, test.str, numStr, test.numStr)
			continue
		}
		if sys == Maven || sys == PyPI {
			// PyPI and Maven handle prerelease specially and do not have build tags per se.
			continue
		}

		pre := []string(nil)
		build := ""
		for _, m := range test.meta {
			if m[0] == '+' {
				build = m
				break
			}
			pre = append(pre, m)
		}
		if len(v.pre) != len(pre) {
			t.Errorf("%s.Parse(%q): got %d prerelease elems; want %d", sys, test.str, len(v.pre), len(pre))
			continue
		}
		if len(pre) == 0 && v.pre != nil {
			t.Errorf("%s.Parse(%q): expected nil prerelease slice", sys, test.str)
			continue
		}
		for i, p := range v.pre {
			if p != pre[i] {
				t.Errorf("%s.Parse(%q): got prerelease %q; want %q", sys, test.str, v.pre, pre)
				continue Outer
			}
		}
		if v.build != build {
			t.Errorf("%s.Parse(%q): got build string %q; want %q", sys, test.str, v.build, build)
			continue Outer
		}
	}
}

func TestBasicVersionParse(t *testing.T) {
	testVersionParse(t, DefaultSystem, basicVersionParseTests)
	testVersionParse(t, Cargo, basicVersionParseTests)
	testVersionParse(t, NPM, basicVersionParseTests)
	testVersionParse(t, NuGet, basicVersionParseTests)
	testVersionParse(t, PyPI, basicVersionParseTests)
	testVersionParse(t, RubyGems, basicVersionParseTests)
}

func TestWildcardVersionParse(t *testing.T) {
	testVersionParse(t, Cargo, wildcardVersionParseTests)
	testVersionParse(t, DefaultSystem, wildcardVersionParseTests)
	testVersionParse(t, NPM, wildcardVersionParseTests)
}

type canonTest struct {
	vs           string
	canonBuild   string
	canonNoBuild string // Empty if same as canonBuild.
}

func testVersionCanon(t *testing.T, sys System, tests []canonTest) {
	for _, test := range tests {
		v := parseVersion(t, sys, test.vs)
		if got := v.Canon(true); got != test.canonBuild {
			t.Errorf("%s Canon(%q, true) = %q; expect %q", sys, test.vs, got, test.canonBuild)
			continue
		}
		noBuild := test.canonNoBuild
		if noBuild == "" {
			noBuild = test.canonBuild
		}
		if got := v.Canon(false); got != noBuild {
			t.Errorf("%s Canon(%q, false) = %q; expect %q", sys, test.vs, got, noBuild)
			continue
		}
		// Check round trip.
		once := v.Canon(true)
		v, err := sys.Parse(once)
		if err != nil {
			t.Errorf("%s.Parse(%q).Canon(true)): %v", sys, test.vs, err)
			continue
		}
		twice := v.Canon(true)
		if twice != once {
			t.Errorf("%s Canon(%q): non idempotent: %q then %q", sys, test.vs, once, twice)
		}
	}
}

type compareTest struct {
	a, b string
	want int
}

// Tests valid for most systems except Maven.
// They work for Go after adding a 'v'.
var basicCompareTests = []compareTest{
	{"", "", 0},
	{"1", "1", 0},
	{"1", "2", -1},
	{"2", "1", 1},

	{"1", "1.0", 0},
	{"1", "1.0.0", 0},
	{"1", "1.0.1", -1},
	{"1.0", "1.0", 0},
	{"1.0.0", "1.0.0", 0},
	{"1", "1.1", -1},
	{"1.0", "1.1", -1},
	{"1.0.1", "1.1", -1},
	{"1.5", "1.11", -1},

	// Ordering of "pre-release versions".
	// Example from the semver.org Version 2.0.0 spec.
	{"1.0.0-alpha.beta", "1.0.0-beta", -1},
	{"1.0.0-beta", "1.0.0-beta.2", -1},
	{"1.0.0-beta.2", "1.0.0-beta.11", -1},
	{"1.0.0-beta.11", "1.0.0-rc.1", -1},
	{"1.0.0-rc.1", "1.0.0", -1},

	// Numeric is compared numerically.
	{"1.0.0-alpha.5", "1.0.0-alpha.10", -1},

	// Shortened versions imply zeros.
	{"1", "1.0", 0},
	{"1", "1.0.0", 0},
	{"1.0", "1.0.0", 0},
}

func TestBasicCompare(t *testing.T) {
	testCompare(t, Cargo, basicCompareTests)
	testCompare(t, NPM, basicCompareTests)
	testCompare(t, NuGet, basicCompareTests)
	testCompare(t, PyPI, basicCompareTests)
	testCompare(t, RubyGems, basicCompareTests)
	// Add the leading v's for Go.
	tests := make([]compareTest, len(basicCompareTests))
	for i, test := range basicCompareTests {
		tests[i] = compareTest{
			"v" + test.a,
			"v" + test.b,
			test.want,
		}
	}
	testCompare(t, Go, tests)
}

func testCompare(t *testing.T, sys System, tests []compareTest) {
	for _, test := range tests {
		if got := sys.Compare(test.a, test.b); got != test.want {
			t.Errorf("%s.Compare(%q, %q) = %d, want %d", sys, test.a, test.b, got, test.want)
			continue
		}
		// Check that the comparison is anticommutative.
		if got := sys.Compare(test.b, test.a); got != -test.want {
			t.Errorf("%s.Compare(%q, %q) = %d, want %d", sys, test.b, test.a, got, -test.want)
		}
	}
}

func testCompareSequential(t *testing.T, sys System, tests []string) {
	t.Helper()
	for i, s1 := range tests {
		// Go always needs a 'v'. We add it here to keep things simple.
		if sys == Go {
			s1 = "v" + s1
		}
		for j, s2 := range tests {
			if sys == Go {
				s2 = "v" + s2
			}
			got := sys.Compare(s1, s2)
			want := sgn(i, j)
			if got != want {
				t.Errorf("%s.Compare(%q, %q) = %d; want %d", sys, s1, s2, got, want)
			}
			// Reverse.
			got = sys.Compare(s2, s1)
			want = sgn(j, i)
			if got != want {
				t.Errorf("%s.Compare(%q, %q) = %d; want %d", sys, s2, s1, got, want)
			}
		}
	}
}

// TestCompareSequential uses an ordered list and does a full cross-check.
// The list comes from Semver.org version 2.0.0.
func TestCompareSequential(t *testing.T) {
	// Sorted in increasing precedence order, based on semver.org 2.0.0 example.
	tests := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
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
	for _, sys := range allSystems {
		// Some systems have their own ordering rules, tested in Test$System$CompareSequential.
		switch sys {
		case Maven, PyPI, RubyGems: // Different rules, separate test.
			continue
		}
		testCompareSequential(t, sys, tests)
	}
}

func TestPossibleVersionString(t *testing.T) {
	tests := []struct {
		sys     int // Bitmap of systems this applies to.
		version string
		want    bool
	}{
		{A, "", false},
		{A, ".", false},
		{A, "abc", false},
		{A, "a.0", false},
		{A, "*", true},
		{A, "x", true},
		{A, "X", true},
		{A, "1.0.0", true},
		{A, "1-alpha", true},
		{A, "1+build", true},
		{A, "1", true},
		{A, "!=1.2.3", false},

		// No leading 'v' outside NPM, PyPI.
		{N | P, "v1", true},
		{A ^ (N | P), "v1", false},

		// Python epoch.
		{P, "1!v1.3.3", true},
	}

	var sys System
	for _, test := range tests {
		for x := test.sys; x != 0; {
			x, sys = nextSystem(x)
			if got := sys.possibleVersionString(test.version); got != test.want {
				if test.want {
					t.Errorf("%s: string \"%q\" might be a version, but got %v", sys, test.version, got)
				} else {
					t.Errorf("%s: string \"%q\" cannot be a version, but got %v", sys, test.version, got)
				}
			}
		}
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		u, v string
		cmp  int
		diff Diff
	}{
		{"1", "1", 0, Same},
		{"1.2", "1.2", 0, Same},
		{"1.2.3", "1.2.3", 0, Same},
		{"1.2.3.4", "1.2.3.4", 0, Same},
		{"1.2.3-pre+build", "1.2.3-pre+build", 0, Same},
		{"2", "1", 1, DiffMajor},
		{"1.3", "1.2", 1, DiffMinor},
		{"1.2.4", "1.2.3", 1, DiffPatch},
		{"1.2.3.5", "1.2.3.4", 1, DiffOther},
		{"1.2.3-pre+build", "1.2.3-pre1+build", -1, DiffPrerelease},
		{"1.2.3", "1.2.3-pre1+build", 1, DiffPrerelease},
		{"1.2.3-pre+build", "1.2.3-pre+build1", 0, DiffBuild},
	}
	for _, test := range tests {
		// We use NuGet because it's close to Semver but allows more than 3 numbers.
		cmp, diff, err := NuGet.Difference(test.u, test.v)
		if err != nil {
			t.Errorf("%q, %q: %v", test.u, test.v, err)
			continue
		}
		if cmp != test.cmp || diff != test.diff {
			t.Errorf("Difference(%q, %q) = (%d %s); want (%d %s)", test.u, test.v, cmp, diff, test.cmp, test.diff)
		}
	}
}

var versionMallocTest = []struct {
	count int
	str   string
}{
	{1, "*"},
	{1, "1.2.3"},
	{5, "1.2.3.alpha1+build"},
}

var constraintMallocTest = []struct {
	count int
	str   string
}{
	{5, "*"},
	{4, "1.2.3"},
	{5, "1.2.3-alpha1+build"},
}

func TestCountMallocs(t *testing.T) {
	for _, mt := range versionMallocTest {
		mallocs := testing.AllocsPerRun(10000, func() { DefaultSystem.Parse(mt.str) })
		// TODO: Switch != to > when things have settled.
		if got, max := mallocs, float64(mt.count); got != max {
			t.Errorf("Parse(%q): got %v allocs, want %v", mt.str, got, max)
		}
	}
	for _, mt := range constraintMallocTest {
		mallocs := testing.AllocsPerRun(10000, func() { DefaultSystem.ParseConstraint(mt.str) })
		// TODO: Switch != to > when things have settled.
		if got, max := mallocs, float64(mt.count); got != max {
			t.Errorf("ParseConstraint(%q): got %v allocs, want %v", mt.str, got, max)
		}
	}
}

func TestParseNumErrors(t *testing.T) {
	tests := []string{
		"",
		"-1",
		"*",
		"∞",
		"x",
		fmt.Sprint(infinity),
		// Too large.
		"9223372036854775807", // 2^63-1, same as the line above.
		"9223372036854775808",
		"18446744073709551615", // 2^64-1
		"18446744073709551616",
	}
	for _, test := range tests {
		_, err := parseNum(test)
		if err == nil {
			t.Errorf("expected error parsing %q", test)
		}
	}
}
