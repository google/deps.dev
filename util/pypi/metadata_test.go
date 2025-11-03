// Copyright 2025 Google LLC
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

package pypi

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

var numpyPkgInfoRaw = `Metadata-Version: 1.2
Name: numPy
Version: 1.16.4
Summary:  NumPy is the fundamental package for array computing with Python.
Home-page: https://www.numpy.org
Author: NumPy Developers
Author-email: numpy-discussion@python.org
License: BSD
Download-URL: https://pypi.python.org/pypi/numpy
Description-Content-Type: UNKNOWN
Description: It provides:
        
        - a powerful N-dimensional array object
        - sophisticated...
        
Platform: Windows
Platform: Linux
Platform: Solaris
Platform: Mac OS-X
Platform: Unix
Classifier: Development Status :: 5 - Production/Stable
Classifier: License :: OSI Approved
Classifier: Programming Language :: C
Classifier: Programming Language :: Python
Classifier: Programming Language :: Python :: Implementation :: CPython
Classifier: Topic :: Software Development
Classifier: Topic :: Scientific/Engineering
Classifier: Operating System :: Microsoft :: Windows
Classifier: Operating System :: POSIX
Classifier: Operating System :: Unix
Classifier: Operating System :: MacOS
Requires-Python: >=2.7,!=3.0.*,!=3.1.*,!=3.2.*,!=3.3.*
Project-URL: Homepage, https://www.numpy.org
`

var numpyPkgInfo = Metadata{
	Name:        "numPy",
	Version:     "1.16.4",
	Summary:     "NumPy is the fundamental package for array computing with Python.",
	Description: "It provides:  - a powerful N-dimensional array object - sophisticated... ",
	Homepage:    "https://www.numpy.org",
	Author:      "NumPy Developers",
	AuthorEmail: "numpy-discussion@python.org",
	License:     "BSD",
	Classifiers: []string{
		"Development Status :: 5 - Production/Stable",
		"License :: OSI Approved",
		"Programming Language :: C",
		"Programming Language :: Python",
		"Programming Language :: Python :: Implementation :: CPython",
		"Topic :: Software Development",
		"Topic :: Scientific/Engineering",
		"Operating System :: Microsoft :: Windows",
		"Operating System :: POSIX",
		"Operating System :: Unix",
		"Operating System :: MacOS",
	},
	ProjectURLs: []string{"Homepage, https://www.numpy.org"},
}

// A real life METADATA file from a wheel, with the description in the body.
var numbaMetadataRaw = `Metadata-Version: 2.1
Name: Numba
Version: 0.44.0
Summary: compiling Python code using LLVM
Home-page: https://github.com/numba/numba
Author: Anaconda, Inc.
Author-email: numba-users@continuum.io
License: BSD
Platform: UNKNOWN
Requires-Dist: llvmlite (>=0.29.0)
Requires-Dist: numpy
Requires-Dist: funcsigs; python_version < "3.3"
Requires-Dist: enum34; python_version < "3.4"
Requires-Dist: singledispatch; python_version < "3.4"

*****
Numba
*****

.. image:: https://badges.gitter.im/numba/numba.svg
   :target: https://gitter.im/numba/numba?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge
   :alt: Gitter

A Just-In-Time Compiler for Numerical Functions in Python
#########################################################

Numba is an open source,
`

var numbaMetadataParsed = Metadata{
	Name:        "Numba",
	Version:     "0.44.0",
	Summary:     "compiling Python code using LLVM",
	Description: "*****\nNumba\n*****\n\n.. image:: https://badges.gitter.im/numba/numba.svg\n   :target: https://gitter.im/numba/numba?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge\n   :alt: Gitter\n\nA Just-In-Time Compiler for Numerical Functions in Python\n#########################################################\n\nNumba is an open source,\n",
	Homepage:    "https://github.com/numba/numba",
	Author:      "Anaconda, Inc.",
	AuthorEmail: "numba-users@continuum.io",
	License:     "BSD",
	Dependencies: []Dependency{
		{"llvmlite", "", ">=0.29.0", ""},
		{"numpy", "", "", ""},
		{"funcsigs", "", "", "python_version < \"3.3\""},
		{"enum34", "", "", "python_version < \"3.4\""},
		{"singledispatch", "", "", "python_version < \"3.4\""},
	},
}

// badPyPIMetadata contains some invalid metadata that should trigger a parse
// error.
var badPyPIMetadata = []string{
	// Missing bracket in the requirement.
	`Metadata-Version: 2.1
Name: numba
Version: 0.44.0
Summary: compiling Python code using LLVM
Requires-Dist: llvmlite[banana (>=0.29.0)

*****
Numba
`,
	// Incorrect line folding.
	`Metadata-Version: 2.1
Name: numba
Version: 0.44.0
Summary: compiling Python code using LLVM
License: A long license that require
many lines to express.
Yes.
Requires-Dist: llvmlite (>=0.29.0)
`,
	// Invalid UTF-8, uses an ISO-8859 non-breaking space.
	`Metadata-Version: 2.1
Name: numba
Version: 0.44.0
Summary: compiling Python` + string([]byte{0xA0}) + ` code using LLVM
`,
}

func TestParseMetadata(t *testing.T) {
	ctx := context.Background()

	// real examples we want to be able to parse
	got, err := ParseMetadata(ctx, numpyPkgInfoRaw)
	if err != nil {
		t.Errorf("Parsing numpy metadata: %v", err)
	}
	if !reflect.DeepEqual(got, numpyPkgInfo) {
		t.Errorf("numpy metadata:\n got: %#v\nwant: %#v", got, numpyPkgInfo)
	}
	got, err = ParseMetadata(ctx, numbaMetadataRaw)
	if err != nil {
		t.Errorf("Parsing numba metadata: %v", err)
	}
	if !reflect.DeepEqual(got, numbaMetadataParsed) {
		t.Errorf("numba metadata:\n got: %#v\nwant: %#v", got, numbaMetadataParsed)
	}
	for i, md := range badPyPIMetadata {
		got, err := ParseMetadata(ctx, md)
		var pErr ParseError
		if ok := errors.As(err, &pErr); !ok {
			t.Errorf("Parsing bad metadata %d: got: (%v, %#v), want ParseError", i, got, err)
		}
	}
}

func TestParseDependency(t *testing.T) {
	for _, c := range []struct {
		r string
		w *Dependency
	}{
		// Cases we do handle.
		// plain names:
		{"plain", &Dependency{"plain", "", "", ""}},
		{"colon;", &Dependency{"colon", "", "", ""}},
		{" leading-space", &Dependency{"leading-space", "", "", ""}},
		{"trailing-space\t", &Dependency{"trailing-space", "", "", ""}},
		// extras:
		{"empty-extra[]", &Dependency{"empty-extra", "", "", ""}},
		{"spaced\t[hello ] ", &Dependency{"spaced", "hello", "", ""}},
		{"extra[more]", &Dependency{"extra", "more", "", ""}},
		{"extras[even, more]", &Dependency{"extras", "even, more", "", ""}},
		// bare constraints, including with non-canonical names:
		{"constraint >=2.1.2", &Dependency{"constraint", "", ">=2.1.2", ""}},
		{"Multi ~=3.6, !=3.8.1", &Dependency{"multi", "", "~=3.6, !=3.8.1", ""}},
		{"no_space>=1,!=3.4", &Dependency{"no-space", "", ">=1,!=3.4", ""}},
		// conditions:
		{"condition;python_version < \"3.6\"", &Dependency{"condition", "", "", "python_version < \"3.6\""}},
		{"space_condition ; platform_machine == x86_64", &Dependency{"space-condition", "", "", "platform_machine == x86_64"}},
		// combinations:
		{"extra-constraint[more] ==2.0", &Dependency{"extra-constraint", "more", "==2.0", ""}},
		{"extra-condition[stuff]; implementation_name == cpython", &Dependency{"extra-condition", "stuff", "", "implementation_name == cpython"}},
		{"constraint-condition <1.0.0-alpha; extra == \"stuff\"", &Dependency{"constraint-condition", "", "<1.0.0-alpha", "extra == \"stuff\""}},
		{"alltheabove[all,the,things] >=0.0; python_version >= 2.0", &Dependency{"alltheabove", "all,the,things", ">=0.0", "python_version >= 2.0"}},
		{"parens (!=2.0)", &Dependency{"parens", "", "!=2.0", ""}},

		// unsalvageable errors:
		{"", nil},
		{";", nil},
		{"unterminated[something >2.1", nil},
	} {
		t.Run(c.r, func(t *testing.T) {
			r, err := ParseDependency(c.r)
			if err != nil {
				if c.w != nil {
					t.Errorf("want %q to parse: got %#v", c.r, err)
				}
				return
			}
			if c.w == nil {
				t.Errorf("want %q to fail: got %#v", c.r, r)
				return
			}
			if !reflect.DeepEqual(c.w, &r) {
				t.Errorf("parse %q: want: %#v, got: %#v", c.r, c.w, r)
			}
		})
	}
}

func TestCanonPackageName(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		// Test cases from https://github.com/pypa/packaging/blob/20.0/tests/test_utils.py.
		{"foo", "foo"},
		{"Foo", "foo"},
		{"fOo", "foo"},
		{"foo.bar", "foo-bar"},
		{"Foo.Bar", "foo-bar"},
		{"Foo.....Bar", "foo-bar"},
		{"foo_bar", "foo-bar"},
		{"foo___bar", "foo-bar"},
		{"foo-bar", "foo-bar"},
		{"foo----bar", "foo-bar"},
		{"foo-Տ", "foo-"}, // Strip out non-ASCII
	}
	for _, test := range tests {
		if got := CanonPackageName(test.in); got != test.out {
			t.Errorf("CanonPackageName(%s): got %s, want %s", test.in, got, test.out)
		}
	}
}
