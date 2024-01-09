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

type opVersionToSpanTest struct {
	sys     int // Bitmap of systems for which this test should be run; see version_test.go
	op      string
	version string
	debug   string
}

// TestOpVersionToSpan tests that we create correct spans, and also that
// sys.parseSpan can parse their printed representation.
// We exclude Maven from these test as it is (mostly) not built using
// opVersionToSpan.
func TestOpVersionToSpan(t *testing.T) {
	tests := []opVersionToSpanTest{
		// Simple.
		{A ^ (P | R), "", "1", "[1.0.0:1.∞.∞]"},
		{A ^ (P | R), "", "1.2", "[1.2.0:1.2.∞]"},
		{A, "", "1.2.3", "1.2.3"},
		{A ^ (P | R), "", "1.2.3+broken", "1.2.3"},
		{A ^ P, "", "1.2.3-potato", "1.2.3-potato"},

		// Simple with wildcards.
		{D | C | N, "", "*", "[0.0.0-0:∞.∞.∞]"},
		{D | C | N, "", "1.*", "[1.0.0:1.∞.∞]"},
		{D | C | N, "", "1.2.*", "[1.2.0:1.2.∞]"},
		{D | C | N, "", "1.2.*+broken", "[1.2.0:1.2.∞]"},
		{D | C | N, "", "1.2.*-potato", "[1.2.0:1.2.∞]"}, // Wildcard stops the prerelease.

		// Greater and greater-equal.
		{A ^ (P | R), ">", "1", "[2.0.0:∞.∞.∞]"},
		{A ^ (P | R), ">", "1.2", "[1.3.0:∞.∞.∞]"},
		{P, ">", "1.2", "[1.2.1:∞.∞.∞]"},
		{A ^ R, ">", "1.2.3", "[1.2.4:∞.∞.∞]"},
		{R, ">", "1.2.3", "(1.2.3:∞.∞.∞]"},
		{A ^ P, ">=", "1", "[1.0.0:∞.∞.∞]"},
		{P, ">=", "1", "[1.0.0:∞.∞.∞]"},
		{A ^ P, ">=", "1.2", "[1.2.0:∞.∞.∞]"},
		{P, ">=", "1.2", "[1.2.0:∞.∞.∞]"},
		{A, ">=", "1.2.3", "[1.2.3:∞.∞.∞]"},

		// Greater and greater-equal, with wildcards.
		{D | C | N, ">", "*", "<empty>"},
		{D | C | N, ">", "1.*", "[2.0.0:∞.∞.∞]"},
		{D | C | N, ">", "1.2.*", "[1.3.0:∞.∞.∞]"},
		{D | C | N, ">=", "*", "[0.0.0-0:∞.∞.∞]"},
		{D | C | N, ">=", "1.*", "[1.0.0:∞.∞.∞]"},
		{D | C | N, ">=", "1.2.*", "[1.2.0:∞.∞.∞]"},

		// Prerelease tags
		{A ^ P, "", "1.2.3-potato", "1.2.3-potato"},
		{A ^ P, "=", "1.2.3-potato", "1.2.3-potato"},
		{A ^ P, ">=", "1.2.3-potato", "[1.2.3-potato:∞.∞.∞]"},
		{A ^ P, ">", "1.2.3-potato", "(1.2.3-potato:∞.∞.∞]"},
		{A ^ P, ">", "1.2.3-rc.9", "(1.2.3-rc.9:∞.∞.∞]"},

		// Build tag is always ignored.
		{A ^ (P | R), "", "1.2.3+potato", "1.2.3"},
		{A ^ (P | R), "=", "1.2.3+potato", "1.2.3"},
		{A ^ (P | R), ">=", "1.2.3+potato", "[1.2.3:∞.∞.∞]"},

		// Less and less-equal.
		{A ^ (P | R), "<", "1", "[0.0.0-0:1.0.0)"},
		{P, "<", "1", "[0.0.0.dev0:1.0.0)"},
		{R, "<", "1", "[0.0.0-a:1.0.0)"},
		{A ^ (P | R), "<", "1.2", "[0.0.0-0:1.2.0)"},
		{P, "<", "1.2", "[0.0.0.dev0:1.2.0)"},
		{R, "<", "1.2", "[0.0.0-a:1.2.0)"},
		{A ^ (P | R), "<", "1.2.3", "[0.0.0-0:1.2.3)"},
		{P, "<", "1.2.3", "[0.0.0.dev0:1.2.3)"},
		{R, "<", "1.2.3", "[0.0.0-a:1.2.3)"},
		{A ^ (P | R), "<=", "1", "[0.0.0-0:1.∞.∞]"},
		{P, "<=", "1", "[0.0.0.dev0:1.0.0]"},
		{R, "<=", "1", "[0.0.0-a:1.0.0]"},
		{A ^ (P | R), "<=", "1.2", "[0.0.0-0:1.2.∞]"},
		{P, "<=", "1.2", "[0.0.0.dev0:1.2.0]"},
		{R, "<=", "1.2", "[0.0.0-a:1.2.0]"},
		{A ^ (P | R), "<=", "1.2.3", "[0.0.0-0:1.2.3]"},
		{P, "<=", "1.2.3", "[0.0.0.dev0:1.2.3]"},
		{R, "<=", "1.2.3", "[0.0.0-a:1.2.3]"},
		{A ^ (P | R), "<", "1.2.3-potato", "[0.0.0-0:1.2.3-potato)"},
		{A ^ (P | R), "<", "1.2.3-rc.10", "[0.0.0-0:1.2.3-rc.10)"},
		{P, "<", "1.2.3-rc.10", "[0.0.0.dev0:1.2.3rc10)"},
		{R, "<", "1.2.3-rc.10", "[0.0.0-a:1.2.3-rc.10)"},
		{A ^ (P | R), "<=", "1.2.3-potato", "[0.0.0-0:1.2.3-potato]"},
		{R, "<=", "1.2.3-potato", "[0.0.0-a:1.2.3-potato]"},

		// Less and less-equal, with wildcards.
		{D | C | N, "<", "*", "<empty>"},
		{D | C | N, "<", "1.*", "[0.0.0-0:1.0.0)"},
		{D | C | N, "<", "1.2.*", "[0.0.0-0:1.2.0)"},
		{D | C | N, "<=", "*", "[0.0.0-0:∞.∞.∞]"},
		{D | C | N, "<=", "1.*", "[0.0.0-0:1.∞.∞]"},
		{D | C | N, "<=", "1.2.*", "[0.0.0-0:1.2.∞]"},

		// Caret.
		{D | C | N, "^", "*", "[0.0.0-0:∞.∞.∞]"},
		{D | C | N, "^", "1", "[1.0.0:1.∞.∞]"},
		{D | C | N, "^", "1.2", "[1.2.0:1.∞.∞]"},
		{D | C | N, "^", "1.2.3", "[1.2.3:1.∞.∞]"},

		// Special cases for caret with zeros.
		{D | C | N, "^", "0", "[0.0.0:0.∞.∞]"},
		{D | C | N, "^", "0.2", "[0.2.0:0.2.∞]"},
		{D | C | N, "^", "0.0", "[0.0.0:0.0.∞]"},
		{D | C | N, "^", "0.2.1", "[0.2.1:0.2.∞]"},

		// Tilde.
		{D | C | N, "~", "1", "[1.0.0:1.∞.∞]"},
		{D | C | N, "~", "1.2", "[1.2.0:1.2.∞]"},
		{D | C | N, "~", "1.2.3", "[1.2.3:1.2.∞]"},
		// Special cases for zeros.
		{D | C | N, "~", "0", "[0.0.0:0.∞.∞]"},
		{D | C | N, "~", "0.2", "[0.2.0:0.2.∞]"},
		{D | C | N, "~", "0.2.1", "[0.2.1:0.2.∞]"},

		// Bacon-eater.
		{D | R, "~>", "1", "[1.0.0:1.∞.∞]"},
		{P, "~=", "1.1", "[1.1.0:1.∞.∞]"},
		{D, "~>", "1.2", "[1.2.0:1.∞.∞]"},
		{D, "~>", "1.2.3", "[1.2.3:1.2.∞]"},

		// PyPI/RubyGems-specific =, ==, and zero-fill.
		{P, "==", "1.2.3", "1.2.3"},
		{A ^ P, "=", "1.2.3", "1.2.3"},
		{P, "==", "1.2", "1.2.0"},
		{P, "==", "1", "1.0.0"},
		{R, "=", "1", "1.0.0"},

		// RubyGems-specific variants.
		{R, "", "1.2.3.4.5", "1.2.3.4.5"},
		{R, ">", "1", "(1.0.0:∞.∞.∞]"},
		{R, ">", "1.2", "(1.2.0:∞.∞.∞]"},
		{R, ">=", "1.2.3.4.5", "[1.2.3.4.5:∞.∞.∞.∞.∞]"},
		{R, ">", "1.2.3.4.5", "(1.2.3.4.5:∞.∞.∞.∞.∞]"},
		{R, ">", "1.2.3.4.5-beta", "(1.2.3.4.5-beta:∞.∞.∞.∞.∞]"},
		{R, "<", "1.2.3.4.5", "[0.0.0-a:1.2.3.4.5)"},
		{R, "<=", "1.2.3.4.5", "[0.0.0-a:1.2.3.4.5]"},
		{R, "~>", "1.2.3.4.5", "[1.2.3.4.5:1.2.3.4.∞]"},
		// Note: != is done in the parser, not in opVersionToSpan.
	}
	for _, test := range tests {
		for x, sys := test.sys, DefaultSystem; x != 0; {
			x, sys = nextSystem(x)
			v := parseVersion(t, sys, test.version)
			typ := operators[sys][test.op]
			if test.op == "" {
				typ = tokEmpty
			}
			span, err := opVersionToSpan(typ, test.op, v)
			if err != nil {
				t.Fatalf("%s: %q: %v", sys, test.op+test.version, err)
			}
			debug := span.String()
			if debug != test.debug {
				t.Errorf("%s: opVersionToSpan(%q, %q) = %s; want %s", sys, test.op, test.version, debug, test.debug)
			}
			span2, _, err := sys.parseSpan(debug)
			if err != nil {
				t.Errorf("%s: %q: can't parse printed string %q: %v", sys, test.op+test.version, debug, err)
				continue
			}
			if span.rank != span2.rank || !span.min.equal(span2.min) || !span.max.equal(span2.max) {
				t.Errorf("%s: %s doesn't round-trip: %s in, %s out", sys, test.op+test.version, span, span2)
			}
		}
	}
}

func TestInc(t *testing.T) {
	tests := []struct {
		version string
		next    string
	}{
		{"1", "2.0.0"}, // ">1" means "2 or greater".
		{"1.2", "1.3.0"},
		{"1.2.3", "1.2.4"},
		{"1.2.*", "1.3.0"},
		{"1.*.*", "2.0.0"},
	}
	for _, test := range tests {
		v := parseVersion(t, DefaultSystem, test.version)
		err := v.inc()
		if err != nil {
			t.Fatal(err)
		}
		str := v.Canon(false)
		if str != test.next {
			t.Errorf("inc(%q) = %q; want %q", test.version, str, test.next)
		}
	}
}

func TestConstraintToSet(t *testing.T) {
	tests := []struct {
		constraint string
		str        string
	}{
		{"", "{[0.0.0:∞.∞.∞]}"}, // Empty constraint is OK, means everything.
		{"<0", "{<empty>}"},
		{"<*", "{<empty>}"},
		{">*", "{<empty>}"},
		{"1.0.0", "{1.0.0}"},
		{"1.x.0", "{[1.0.0:1.∞.∞]}"},
		{"1.x.1", "{[1.0.0:1.∞.∞]}"},
		{"=1.2.3", "{1.2.3}"},
		{">1.0.0", "{[1.0.1:∞.∞.∞]}"},
		{">=1.0.0", "{[1.0.0:∞.∞.∞]}"},
		{"1.0.x", "{[1.0.0:1.0.∞]}"},
		{"4.0.0 || 4.2.0 >3.2", "{4.0.0,4.2.0}"},
		{"3 3.3 >3.2", "{[3.3.0:3.3.∞]}"},
		{"1.0.0 - 1.2.0", "{[1.0.0:1.2.0]}"},
		{"1.0.0 - 1.2.*", "{[1.0.0:1.2.∞]}"},
		{"1.0.* - 1.2.*", "{[1.0.0:1.2.∞]}"},
		{"1.* - 1.2.*", "{[1.0.0:1.2.∞]}"},
		{"0.8 - 0.9", "{[0.8.0:0.9.∞]}"},
		{"1.0.0 - 1.2.0 || 3.0", "{[1.0.0:1.2.0],[3.0.0:3.0.∞]}"},
		{"1.0.0 - 4.2.0 || 5.2.*", "{[1.0.0:4.2.0],[5.2.0:5.2.∞]}"},
		{"3.2 || 4.0", "{[3.2.0:3.2.∞],[4.0.0:4.0.∞]}"},
		{">3.2 || 4.0", "{[3.3.0:∞.∞.∞]}"},
		{">3.2 || 1.*", "{[1.0.0:1.∞.∞],[3.3.0:∞.∞.∞]}"},
		{"1.0.0 1.2.0 >3.2 || 4.0", "{[4.0.0:4.0.∞]}"},
		{" 1.0.0 1.2.0 >3.2 || 4.0 4.0.1 ", "{4.0.1}"},
		{" 1.0.0 1.2.0 >3.2 || 4.0, 4.*.2 ", "{[4.0.0:4.0.∞]}"},
		{"1.0.0 1.2.0 >3.2 || 4.0, 4.0.1", "{4.0.1}"},
		{"  3.0.0   3.2.0   > 2.9 ||   4.0.2 || 4.0.3", "{[4.0.2:4.0.3]}"},
	}
	for _, test := range tests {
		constraint := parseConstraint(t, DefaultSystem, test.constraint)
		str := constraint.set.String()
		if str != test.str {
			t.Errorf("String(%q) = %q; want %q", test.constraint, str, test.str)
		}
	}
}
