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

func parseConstraint(t *testing.T, sys System, str string) *Constraint {
	t.Helper()
	c, err := sys.ParseConstraint(str)
	if err != nil {
		t.Fatalf("%s: %q: %v", sys, str, err)
	}
	return c
}

// Debug returns a string representation of the Set that defines the constraint.
// The returned string is not itself a valid semver constraint description.
func (c *Constraint) Debug() string {
	if c == nil {
		return "<nil>"
	}
	return c.set.String()
}

// TestConstraintSpans uses the string representation to check correct construction of spans.
func TestConstraintSpans(t *testing.T) {
	tests := []struct {
		cs    string
		debug string
	}{
		{"", "{[0.0.0:∞.∞.∞]}"}, // Empty constraint is OK; it matches the universe (except pre-releases).
		{"1.0.0", "{1.0.0}"},
		{"1.x.0", "{[1.0.0:1.∞.∞]}"}, // Ignore what follows the wildcard.
		{">1.0.0", "{[1.0.1:∞.∞.∞]}"},
		{">=1.0.0", "{[1.0.0:∞.∞.∞]}"},
		{"1.0.x", "{[1.0.0:1.0.∞]}"},
		{"1.0.0 1.2.0", "{<empty>}"},
		{"1.0.0 1.2.0 >3.2", "{<empty>}"},
		{"3 3.3 >3.2", "{[3.3.0:3.3.∞]}"},
		{"1.0.0 - 1.2.0", "{[1.0.0:1.2.0]}"},
		{"3.2.0||<2.0", "{[0.0.0-0:2.0.0),3.2.0}"}, // No spaces around ||.
		{"3.0.0||>=5.0", "{3.0.0,[5.0.0:∞.∞.∞]}"},  // No spaces around ||.
		{">3.2 || 4.0", "{[3.3.0:∞.∞.∞]}"},
		{">3.2 || 4.0", "{[3.3.0:∞.∞.∞]}"},
		{">3.2 || 1.*", "{[1.0.0:1.∞.∞],[3.3.0:∞.∞.∞]}"},
		{"1.0.0 1.2.0 >3.2 || 4.0", "{[4.0.0:4.0.∞]}"},
		{"1.0.0 1.2.0 >3.2 || 4.0, 4.0.1", "{4.0.1}"},
		{"1.0.0   1.2.0   > 3.2 ||    4.0,  1.2 ,  1.3", "{<empty>}"},
		// Leading spaces don't matter.
		{" 1.0.0 1.2.0 >3.2 || 4.0 4.0.1 ", "{4.0.1}"},
		{" 1.0.0 1.2.0 >3.2 || 4.0, 4.*.2 ", "{[4.0.0:4.0.∞]}"},
		{"  1.0.0   1.2.0   > 3.2 ||    4.0,  1.2 ,  1.3", "{<empty>}"},
	}
	for _, test := range tests {
		c := parseConstraint(t, DefaultSystem, test.cs)
		if got := c.Debug(); got != test.debug {
			t.Errorf("Debug(%q) is %q; expect %q\n", test.cs, got, test.debug)
		}
	}
}

type constraintErrorTest struct {
	str string
	err string
}

func testConstraintError(t *testing.T, sys System, tests []constraintErrorTest) {
	for _, test := range tests {
		_, err := sys.ParseConstraint(test.str)
		if err == nil {
			t.Errorf("%s: no error for %q", sys, test.str)
			continue
		}
		if err.Error() != test.err {
			t.Errorf("%s.ParseConstraint(%q) got error %#q; should have %q", sys, test.str, err, test.err)
		}
	}
}

func TestParseSetConstraint(t *testing.T) {
	tests := []string{
		"{}", // Short form for {<empty>}.
		"{<empty>}",
		"{1.0.0}",
		"{1.2.3.4}",
		"{[1.0.0:2.0.0]}",
		"{[1.0.0:1.∞.∞]}",
		"{[1.0.1:∞.∞.∞]}",
		"{[3.2.0:3.2.∞],[4.0.0:4.0.∞]}",
	}
	for _, test := range tests {
		c, err := RubyGems.ParseSetConstraint(test) // Use RubyGems so we can have many numbers
		if err != nil {
			t.Errorf("parsing %s: %v", test, err)
			continue
		}
		// Does it come back?
		if test == "{}" {
			// Special case.
			test = "{<empty>}"
		}
		str := c.set.String()
		if str != test {
			t.Fatal(test, str)
		}
	}
}

func TestParseSetConstraintError(t *testing.T) {
	tests := []struct {
		c   string
		err string
	}{
		{"", "missing or misplaced braces: ``"},
		{"{}x", "missing or misplaced braces: `{}x`"},
		{"{<empty>", "missing or misplaced braces: `{<empty>`"},
		{"{1.0.0}}", "invalid character '}' in `1.0.0}`"},
		{"{1.2.3.4.∞}", "invalid character '∞' in `1.2.3.4.∞`"},
		{"{1.☃.3.4}", "invalid character '☃' in `1.☃.3.4`"},
		{"{[1.0.0,2.0.0]}", "syntax error parsing span `[1.0.0`"},
		{"{1.0.0:1.∞.∞}", "invalid character ':' in `1.0.0:1.∞.∞`"},
	}
	for _, test := range tests {
		_, err := RubyGems.ParseSetConstraint(test.c) // Use RubyGems so we can have many numbers
		if err == nil {
			t.Errorf("expected error parsing %s", test.c)
			continue
		}
		if err.Error() != test.err {
			t.Errorf("%s: got error %q; expected %q", test.c, err, test.err)
		}
	}
}

func TestIsSimple(t *testing.T) {
	tests := []struct {
		c      string
		simple bool
	}{
		{"1", true},
		{"=1", true},
		{"1.2", true},
		{">1", false},
		{"1 2", false},
	}
	for _, test := range tests {
		c, err := DefaultSystem.ParseConstraint(test.c)
		if err != nil {
			t.Fatal(err)
		}
		got := c.IsSimple()
		if got != test.simple {
			t.Errorf("DefaultSystem.IsSimple(%q) is %t; want %t", test.c, got, test.simple)
		}
	}
}
