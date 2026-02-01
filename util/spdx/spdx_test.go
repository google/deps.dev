// Copyright 2024 Google LLC
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

package spdx

import (
	"errors"
	"testing"
)

func TestParseAndCanon(t *testing.T) {
	tests := []struct {
		in    string
		canon string
	}{
		// These strings are from the SPDX specification 2.1,
		// Appendix IV.
		{"GPL-2.0", "GPL-2.0"},
		{"GPL-2.0+", "GPL-2.0+"},
		{"(LGPL-2.1 OR MIT OR BSD-3-Clause)", "BSD-3-Clause OR LGPL-2.1 OR MIT"},
		{"(LGPL-2.1 AND MIT)", "LGPL-2.1 AND MIT"},
		{"(MIT AND LGPL-2.1)", "LGPL-2.1 AND MIT"},
		{"(GPL-2.0+ WITH Bison-exception-2.2)", "GPL-2.0+ WITH Bison-exception-2.2"},
		{"(MIT AND (LGPL-2.1+ OR BSD-3-Clause))", "MIT AND (BSD-3-Clause OR LGPL-2.1+)"},
		{"LGPL-2.1 OR BSD-3-Clause AND MIT", "LGPL-2.1 OR (BSD-3-Clause AND MIT)"},

		// Repeat some of the previous tests, but with extra parens in the input.
		{"(GPL-2.0+)", "GPL-2.0+"},                       // unnecessary parens around an ID
		{"((GPL-2.0+))", "GPL-2.0+"},                     // even more unnecessary parens around an ID
		{"((((MIT))) AND LGPL-2.1)", "LGPL-2.1 AND MIT"}, // very nested parens

		// Check license ID canonicalization.
		{"Gpl-3.0 OR Bsd-3-ClAuSe", "BSD-3-Clause OR GPL-3.0"},
		// Check exception ID canonicalization.
		{"gpl-3.0 WITH bison-exception-2.2", "GPL-3.0 WITH Bison-exception-2.2"},

		// Check combining WITH and OR.
		// A previous bug in this code would stop at the "OR".
		{"GPL-2.0+ WITH Bison-exception-2.2 OR MIT", "MIT OR GPL-2.0+ WITH Bison-exception-2.2"},

		// Check combining separate ANDs or ORs.
		{"Gpl-3.0 OR Bsd-3-ClAuSe OR MIT", "BSD-3-Clause OR GPL-3.0 OR MIT"},
		{"bsd-3-Clause AND GPL-3.0 AND MIT", "BSD-3-Clause AND GPL-3.0 AND MIT"},

		// More test cases for precedence.

		// A or B and C -> A or (B and C)
		{"LGPL-2.1 OR MIT AND BSD-3-Clause", "LGPL-2.1 OR (BSD-3-Clause AND MIT)"},
		// A and B or C -> (A and B) or C
		{"LGPL-2.1 AND MIT OR BSD-3-Clause", "BSD-3-Clause OR (LGPL-2.1 AND MIT)"},
		{"MIT OR GPL-2.0+ WITH Bison-exception-2.2", "MIT OR GPL-2.0+ WITH Bison-exception-2.2"},
		{"GPL-2.0+ WITH Bison-exception-2.2 OR MIT", "MIT OR GPL-2.0+ WITH Bison-exception-2.2"},
		// A or B or C should remain that way
		{`LGPL-2.1 OR MIT OR BSD-3-Clause`, `BSD-3-Clause OR LGPL-2.1 OR MIT`},
	}
	for _, test := range tests {
		le, err := ParseLicenseExpression(test.in)
		if err != nil {
			t.Errorf("ParseLicenseExpression(%q): %v", test.in, err)
			continue
		}
		le.Canon()
		if out := le.String(); out != test.canon {
			t.Errorf("Round trip of %q ended with %q, want %q", test.in, out, test.canon)
		}
	}

	// Test set that should fail.
	fails := []string{
		// Some errors
		"",
		"(",
		"GPL-2.0 ish",
		"GPL-2.0 OR",
		"GPL-2.0 AND",
		"GPL-2.0 WITH",
		"(GPL-2.0",
	}
	for _, test := range fails {
		le, err := ParseLicenseExpression(test)
		if err == nil {
			t.Errorf("ParseLicenseExpression(%q): wanted error, got %v", test, le)
			continue
		}
	}

	// Also check that the deprecated "/" is accepted.
	const in = "Apache-2.0/MIT"
	le, err := ParseLicenseExpression(in)
	if err != nil {
		t.Fatalf("ParseLicenseExpression(%q): %v", in, err)
	}
	const want = "Apache-2.0 OR MIT"
	if out := le.String(); out != want {
		t.Errorf("Round trip of %q ended with %q, want %q", in, out, want)
	}
}

func TestValid(t *testing.T) {
	e := errors.New
	tests := []struct {
		in      string
		wantErr error
	}{
		{"MIT", nil},
		{"MIT+", nil},
		{"FOO", e(`unknown license "FOO"`)},
		{"GPL-2.0 WITH Bison-exception-2.2", nil},
		{"GPL-2.0 WITH Foo-exception", e(`unknown exception "Foo-exception"`)},
		{"MIT AND GPL-2.0", nil},
		{"MIT AND FOO", e(`unknown license "FOO"`)},
		{"MIT OR GPL-2.0", nil},
		{"MIT OR FOO", e(`unknown license "FOO"`)},
		{"(MIT)", nil},
		{"(FOO)", e(`unknown license "FOO"`)},

		// License and exception identifiers are case-insensitive.
		// https://spdx.github.io/spdx-spec/appendix-IV-SPDX-license-expressions/#case-sensitivity
		{"gpl-2.0 WITH bison-exception-2.2", nil},
	}
	for _, test := range tests {
		le, err := ParseLicenseExpression(test.in)
		if err != nil {
			t.Errorf("ParseLicenseExpression(%q): %v", test.in, err)
			continue
		}
		if gotErr := le.Valid(); !errorsEqual(gotErr, test.wantErr) {
			t.Errorf("%q.Valid: got %q, want %q", test.in, gotErr, test.wantErr)
		}
	}
}

func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}
