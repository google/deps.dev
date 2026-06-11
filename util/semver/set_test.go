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
	"strings"
	"testing"
)

// sameSet reports whether, in the specified system, the reference
// and actual constraint are the same. The reference constraint uses
// the set notation parsed by ParseSetConstraint, while the actual
// constraint uses the native notation for the system.
// It is assumed both strings are valid constraint specifications. The
// function panics if either is not.
func sameSet(sys System, actual, reference string) bool {
	con, err := sys.ParseConstraint(actual)
	if err != nil {
		panic(fmt.Sprintf("%s.ParseConstraint(%q): %v", sys, actual, err))
	}
	ref, err := sys.ParseSetConstraint(reference)
	if err != nil {
		panic(fmt.Sprintf("%s.ParseSetConstraint(%q): %v", sys, reference, err))
	}
	return ref.set.equal(con.set)
}

func (s Set) equal(t Set) bool {
	if s.sys != t.sys {
		return false
	}
	if len(s.span) != len(t.span) {
		return false
	}
	for i, span := range s.span {
		if !span.equal(t.span[i]) {
			return false
		}
	}
	return true
}

func (s span) equal(t span) bool {
	if s.rank != t.rank || s.minOpen != t.minOpen || s.maxOpen != t.maxOpen {
		return false
	}
	return s.min.equal(t.min) && s.max.equal(t.max)
}

func strs(s ...string) []string {
	return s
}

func simpleConstraintToSpan(sys System, str string) span {
	nums := strings.IndexAny(str, "0123456789")
	v, err := sys.Parse(str[nums:])
	if err != nil {
		panic(err)
	}
	op := str[:nums]
	typ := operators[sys][op]
	if op == "" {
		typ = tokEmpty
	}
	span, err := opVersionToSpan(typ, op, v)
	if err != nil {
		panic(err)
	}
	return span
}
func spans(t *testing.T, s ...string) []span {
	t.Helper()
	out := []span{}
	for i := 0; i < len(s); i++ {
		str := s[i]
		if len(str) > 0 && (str[0] == '[' || str[0] == '(') {
			minOpen := str[0] == '('
			maxOpen := str[len(str)-1] == ')'
			str = str[1 : len(str)-1] // Drop []
			comma := strings.Index(str, ":")
			if comma >= 0 {
				s1 := simpleConstraintToSpan(DefaultSystem, str[:comma])
				s2 := simpleConstraintToSpan(DefaultSystem, str[comma+1:])
				span, err := newSpan(s1.min, minOpen, s2.max, maxOpen)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, span)
				continue
			}
		}
		out = append(out, simpleConstraintToSpan(DefaultSystem, str))
	}
	return out
}

func TestUnion(t *testing.T) {
	tests := []struct {
		s1, s2 []string
		out    string
	}{
		// Similar elements.
		{strs("1"), strs("1.0"), "{[1.0.0:1.∞.∞]}"},
		{strs("1.0"), strs("1.0.0"), "{[1.0.0:1.0.∞]}"},
		{strs("1.0.0"), strs("1.0.0"), "{1.0.0}"},
		{strs("1.0.0-pre"), strs("1.0.0-pre"), "{1.0.0-pre}"},

		// Units.
		{strs("1"), strs("2"), "{[1.0.0:2.∞.∞]}"},
		{strs("1.2"), strs("2.4"), "{[1.2.0:1.2.∞],[2.4.0:2.4.∞]}"},
		{strs("1.2.3"), strs("2.4.5"), "{1.2.3,2.4.5}"},

		// Distinct spans.
		{strs("[1.0.0:2.0.0]"), strs("[3.0.0:4.0.0]"), "{[1.0.0:2.0.0],[3.0.0:4.0.0]}"},

		// Open/closed touching spans.
		{strs("[1.0.0:2.0.0]"), strs("[2.0.0:3.0.0]"), "{[1.0.0:3.0.0]}"},
		{strs("[1.0.0:2.0.0)"), strs("[2.0.0:3.0.0]"), "{[1.0.0:3.0.0]}"},
		{strs("[1.0.0:2.0.0]"), strs("(2.0.0:3.0.0]"), "{[1.0.0:3.0.0]}"},
		{strs("[1.0.0:2.0.0)"), strs("(2.0.0:3.0.0]"), "{[1.0.0:2.0.0),(2.0.0:3.0.0]}"}, // Don't merge when both are open.

		// Overlapping spans.
		{strs("[1.0.0:2.0.0}"), strs("[1.5:4.0.0}"), "{[1.0.0:4.0.0]}"},

		{strs("3"), strs("3.9"), "{[3.0.0:3.∞.∞]}"}, // Tricky because 3 matches 3.1, 3.2 etc, not just 3.0.0.

		// Symmetry, open/closed.
		{strs("(1.0.0:2.0.0]"), strs("[1.0.0:2.0.0]"), "{[1.0.0:2.0.0]}"},
		{strs("[1.0.0:2.0.0]"), strs("(1.0.0:2.0.0]"), "{[1.0.0:2.0.0]}"},
		{strs("[1.0.0:2.0.0)"), strs("[1.0.0:2.0.0]"), "{[1.0.0:2.0.0]}"},
		{strs("[1.0.0:2.0.0]"), strs("[1.0.0:2.0.0)"), "{[1.0.0:2.0.0]}"},
		// NOTE: "{[0.0.0-0:1.2.3]}" would be better, but canon currently avoids
		// merging prereleases and non-preleases, and "0.0.0-0" is technically a
		// prerelease.
		{strs("<1.2.3"), strs("<=1.2.3"), "{[0.0.0-0:1.2.3),[0.0.0-0:1.2.3]}"},
		{strs("<=1.2.3"), strs("<1.2.3"), "{[0.0.0-0:1.2.3),[0.0.0-0:1.2.3]}"},
	}
	for _, test := range tests {
		set1 := Set{DefaultSystem, spans(t, test.s1...)}
		set2 := Set{DefaultSystem, spans(t, test.s2...)}
		err := set1.Union(set2)
		if err != nil {
			t.Error(err)
			continue
		}
		out := set1.String()
		if out != test.out {
			t.Errorf("Union(%q, %q) = %s; want %s", test.s1, test.s2, out, test.out)
		}
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		s1, s2 []string
		out    string
	}{
		// Related elements.
		{strs("1"), strs("1.0"), "{[1.0.0:1.0.∞]}"},
		{strs("1.0"), strs("1.0.0"), "{1.0.0}"},
		{strs("1.2.3"), strs("1.2.3"), "{1.2.3}"},
		{strs(">=1.3.0"), strs("<1.3.0"), "{<empty>}"},
		{strs(">1.3.0"), strs("<=1.3.0"), "{<empty>}"},

		// Disjoint.
		{strs("1"), strs("2"), "{<empty>}"},

		// Contained.
		{strs("[1:2]"), strs("[1.3:1.5]"), "{[1.3.0:1.5.∞]}"},

		// Abutting.
		{strs("[1:1.5.3]"), strs("[1.5.3:2]"), "{1.5.3}"},
		{strs("[1:1.5]"), strs("[1.5:2]"), "{[1.5.0:1.5.∞]}"},
		{strs("[1.5]"), strs("[1.5:2]"), "{[1.5.0:1.5.∞]}"},

		// Overlapping.
		{strs(">=1.0"), strs("<3.2"), "{[1.0.0:3.2.0)}"},

		// Symmetry, open/closed.
		{strs("(1.0.0:2.0.0]"), strs("[1.0.0:2.0.0]"), "{(1.0.0:2.0.0]}"},
		{strs("[1.0.0:2.0.0]"), strs("(1.0.0:2.0.0]"), "{(1.0.0:2.0.0]}"},
		{strs("[1.0.0:2.0.0)"), strs("[1.0.0:2.0.0]"), "{[1.0.0:2.0.0)}"},
		{strs("[1.0.0:2.0.0]"), strs("[1.0.0:2.0.0)"), "{[1.0.0:2.0.0)}"},
		{strs("<1.2.3"), strs("<=1.2.3"), "{[0.0.0-0:1.2.3)}"},
		{strs("<=1.2.3"), strs("<1.2.3"), "{[0.0.0-0:1.2.3)}"},

		// Bug regression: Intersecting [1.0.0, 1.0.0] with (1.0.0, inf) should be empty.
		// We use <1.0.0 vs 1.0.0 to trigger [0, 1) vs [1, 1].
		{strs("<1.0.0"), strs("1.0.0"), "{<empty>}"},
		// We use >1.0.0 vs 1.0.0 to trigger (1, inf) vs [1, 1].
		{strs(">1.0.0"), strs("1.0.0"), "{<empty>}"},
	}
	for _, test := range tests {
		set1 := Set{DefaultSystem, spans(t, test.s1...)}
		set2 := Set{DefaultSystem, spans(t, test.s2...)}
		err := set1.Intersect(set2)
		if err != nil {
			t.Errorf("Intersect(%q, %q): %v", test.s1, test.s2, err)
			continue
		}
		out := set1.String()
		if out != test.out {
			t.Errorf("Intersect(%q, %q) = %s; want %s", test.s1, test.s2, out, test.out)
		}
	}
}
