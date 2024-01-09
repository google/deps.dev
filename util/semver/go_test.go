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

import "testing"

// Go-specific tests.

var goVersionParseTests = []versionParseTest{
	// Largely copied from basicVersionParseTests, with mandatory 'v'.
	v("v1.2.3", "", "1.2.3"),
	v("v1.2.3-alpha", "", "1.2.3", "alpha"),
	v("v1.2.3-alpha.1", "", "1.2.3", "alpha", "1"),
	v("v1.2.3-beta.01", "", "1.2.3", "beta", "01"), // The 01 is legal, but not a "number".

	// Very large value.
	v("v1.2.20181231235959", "", "1.2.20181231235959"),

	// Errors.
	v("", "invalid version ``", ""),
	v("☃", "invalid version `☃`", ""),
	v("1..7", "invalid version `1..7`", ""),
	v("1.0. 0", "invalid version `1.0. 0`", ""),
	v("1.0.0-alpha..", "invalid version `1.0.0-alpha..`", ""),
	v("1.0.0-alpha..x", "invalid version `1.0.0-alpha..x`", ""),
	v("1.0.0-alpha.☃", "invalid version `1.0.0-alpha.☃`", ""),

	// Now some more general errors.
	v("1.2.3", "invalid version `1.2.3`", ""),
	v("vvvvvvvvvvvv1.2.3", "invalid version `vvvvvvvvvvvv1.2.3`", ""),
	v("v1.2.3.beta1", "invalid text in version string in `v1.2.3.beta1`", ""),
	v("v2.0.0b5", "invalid text in version string in `v2.0.0b5`", ""),
	v("v010", "number has leading zero in `v010`", ""),
	v("v1.0.0.0", "more than 3 numbers present in `v1.0.0.0`", ""),
	v("v1.rabbit", "non-numeric version in `v1.rabbit`", ""),
	v("v0.0.01", "number has leading zero in `v0.0.01`", ""),
	v("v1.2.", "empty component in `v1.2.`", ""),
	v("v1.0.0 ", "invalid character ' ' in `v1.0.0 `", ""),
	v("v1.*x.0-alpha.0", "non-numeric version in `v1.*x.0-alpha.0`", ""),
	v("vv2.0.0", "invalid version `vv2.0.0`", ""),
}

func TestGoVersionParse(t *testing.T) {
	testVersionParse(t, Go, goVersionParseTests)
}

var goConstraintErrorTests = []constraintErrorTest{
	// Go constraints are just versions, so we don't need much.
	{"v1.0.0 || v2.0.0", "invalid character ' ' in `v1.0.0 || v2.0.0`"},
	{"v1.0.0, v2.0.0", "invalid character ',' in `v1.0.0, v2.0.0`"},
	{"v1.0.0 v2.0.0", "invalid character ' ' in `v1.0.0 v2.0.0`"},
	{">=v1.0.0", "invalid version `>=v1.0.0`"},
}

func TestGoConstraintError(t *testing.T) {
	testConstraintError(t, Go, goConstraintErrorTests)
}

var goCanonTests = []canonTest{
	{"v1", "v1.0.0", ""},
	{"v1.0", "v1.0.0", ""},
	{"v1.0.0", "v1.0.0", ""},
	{"v1.0.0+beta.2", "v1.0.0+beta.2", "v1.0.0"},
	{"v1.0.0-alpha", "v1.0.0-alpha", ""},
	{"v1.0.0-alpha.1", "v1.0.0-alpha.1", ""},
	{"v1.0.0-alpha.1+beta.2", "v1.0.0-alpha.1+beta.2", "v1.0.0-alpha.1"},

	// Pre-release and build metadata containing hyphens.
	{"v1.0.0-a-b", "v1.0.0-a-b", ""},
	{"v1.0.0+a-b", "v1.0.0+a-b", "v1.0.0"},

	{"v6.0.0-beta1", "v6.0.0-beta1", ""},
}

func TestGoCanon(t *testing.T) {
	testVersionCanon(t, Go, goCanonTests)
}

var goCompareTests = []compareTest{
	// Ordering of "pre-release versions".
	// Example from the semver.org Version 2.0.0 spec.
	{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
	{"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1},

	// Zero-prefixed elements are not numbers.
	{"v1.0.0-rc.12", "v1.0.0-rc.011", -1},
	{"v0.0.1-alpha.006", "v0.0.1-alpha.6", 1},

	// Hyphens are part of the identifiers.
	{"v1.0.0-alpha", "v1.0.0-alpha-1", -1},
	{"v1.0.0-alpha.1", "v1.0.0-alpha-1", -1},
	{"v1.0.0-alpha-1", "v1.0.0-alpha-1.1", -1},

	// Numeric is below alphabetic.
	{"v1.0.0-1", "v1.0.0-alpha", -1},
	{"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1},

	// Build tags are ignored.
	{"v6.0.0+build1", "v6.0.0+build1", 0},
	{"v6.0.0+build1", "v6.0.0+build2", 0},
	{"v6.0.0+build2", "v6.0.0+build1", 0},
	{"v6.0.0", "v6.0.0+build1", 0},
	{"v6.0.0+build2", "v6.0.0-aaa", 1},
	{"v6.0.0+build2", "v6.0.0-zzz", 1},

	// Shortened versions imply zeros.
	{"v1", "v1.0", 0},
	{"v1", "v1.0.0", 0},
	{"v1.0", "v1.0.0", 0},
}

func TestGoCompare(t *testing.T) {
	testCompare(t, Go, goCompareTests)
}

func TestGoSets(t *testing.T) {
	tests := []struct {
		con string
		ref string
	}{
		{"v0.2.3", "{[v0.2.3:v2.0.0)}"}, // v0 is compatible with v1 in Go.
		{"v1.2.3", "{[v1.2.3:v2.0.0)}"},
		{"v2.2.3", "{[v2.2.3:v3.0.0)}"},
		{"v3.2.3", "{[v3.2.3:v4.0.0)}"},
	}
	for _, test := range tests {
		if !sameSet(Go, test.con, test.ref) {
			c, _ := Go.ParseConstraint(test.con)
			t.Errorf("Go set mismatch: (%q) is %q; expect %q\n", test.con, c.set, test.ref)
		}
	}
}
