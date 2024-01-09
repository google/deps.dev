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

type simplifyTest struct {
	before string
	after  string
}

var simplifyTests = []simplifyTest{
	{"", "{[0.0.0:∞.∞.∞]}"},
	{"1.0.0", "{1.0.0}"},

	// Duplications with space.
	{"1.0.0 1.0.0", "{1.0.0}"},
	{"1.0.0 1.0.0 1.0.0", "{1.0.0}"},
	{">1.0.0 >1.0.0 >1.0.0", "{[1.0.1:∞.∞.∞]}"},
	{"1.0.0 2.0.0 1.0.0", "{<empty>}"},
	{">1.0.0 >1.0.0 1.0.0", "{<empty>}"},
	{">1.0.0 >1.0.0 1.0.1", "{1.0.1}"},
	{">=1.0.0 >=1.0.0 1.0.0", "{1.0.0}"},

	// Duplications with OR.
	{"1.0.0 || 1.0.0", "{1.0.0}"},
	{"1.0.0 || 1.0.0 || 1.0.0", "{1.0.0}"},
	{">1.0.0 || >1.0.0 || >1.0.0", "{[1.0.1:∞.∞.∞]}"},
	{"1.0.0 || 2.0.0 || 1.0.0", "{1.0.0,2.0.0}"},

	// Duplications with comma.
	{"1.0.0 , 1.0.0", "{1.0.0}"},
	{"1.0.0 , 1.0.0 , 1.0.0", "{1.0.0}"},
	{">1.0.0 , >1.0.0 , >1.0.0", "{[1.0.1:∞.∞.∞]}"},
	{"1.0.0 , 2.0.0 , 1.0.0", "{<empty>}"},

	// Combinations of duplications.
	{"1.0.0 || 1.0.0 1.0.0", "{1.0.0}"},
	{"1.0.0 1.0.0 || 1.0.0 1.0.0, 1.0.0", "{1.0.0}"},

	// Values.
	{"1.0.0 - 1.0.0", "{1.0.0}"},
	{"1.x.3", "{[1.0.0:1.∞.∞]}"},
	{"x.x.3", "{[0.0.0-0:∞.∞.∞]}"},
	{"1.0.0 1.0.0 || 1.0.0 1.0.0, 1.0.0 || 1.x.3-pre , 1.0.0", "{1.0.0}"}, // Prerelease suppressed for wildcard.

	// Fixed bugs.
	// Don't merge pre and non-pre.
	{"^7.0.0-pre || 7.x", "{[7.0.0-pre:7.∞.∞],[7.0.0:7.∞.∞]}"},
	{"0.3.x || 7.x", "{[0.3.0:0.3.∞],[7.0.0:7.∞.∞]}"},
}

// Simplify is gone, but this old test still verifies that construction
// of constraints leads to simplified results.
// We also use it as a secondary test of Constraint.String().
func TestSimplify(t *testing.T) {
	for _, test := range simplifyTests {
		c := parseConstraint(t, DefaultSystem, test.before)
		if str := c.Debug(); str != test.after {
			t.Errorf("Simplify(%q).Debug() = %q; expected %q", test.before, str, test.after)
		}
	}
}
