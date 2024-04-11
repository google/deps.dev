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

package schema

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/internal/deptest"
	"deps.dev/util/resolve/version"
)

const testSchema = `
# Whole-line comment
package-1
	1.0.0
		another-package@^1.0.0
	1.1.1
		@scoped/package@^1.1.1
	Blocked|2.0.0
		Opt|another-package@2.0.0
another-package
	1.0.0
		Dev|package-1@1.0.0
	2.0.0
		ATTR: Blocked
		ATTR: Redirect go somewhere else
		package-1@2.0.0 # End of line comment
package-2
	0.0.0
		XTest someXTest|package-2@v0.0.0
		Framework "some framework" Opt Dev|package-1@1.0.0
@scoped/package
	1.1.1
`

var (
	pkg1       = resolve.PackageKey{System: resolve.NPM, Name: "package-1"}
	anotherPkg = resolve.PackageKey{System: resolve.NPM, Name: "another-package"}
	pkg2       = resolve.PackageKey{System: resolve.NPM, Name: "package-2"}
	scopedPkg  = resolve.PackageKey{System: resolve.NPM, Name: "@scoped/package"}
)

func date(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return t
}

var wantTestSchema = &Schema{Packages: []Package{{
	PackageKey: pkg1,
	Versions: []Version{{
		VersionKey: resolve.VersionKey{
			PackageKey:  pkg1,
			VersionType: resolve.Concrete,
			Version:     "1.0.0",
		},
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  anotherPkg,
				Version:     "^1.0.0",
				VersionType: resolve.Requirement,
			},
		}},
	}, {
		VersionKey: resolve.VersionKey{
			PackageKey:  pkg1,
			VersionType: resolve.Concrete,
			Version:     "1.1.1",
		},
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  scopedPkg,
				Version:     "^1.1.1",
				VersionType: resolve.Requirement,
			},
		}},
	}, {
		VersionKey: resolve.VersionKey{
			PackageKey:  pkg1,
			VersionType: resolve.Concrete,
			Version:     "2.0.0",
		},
		Attr: buildVersionAttr(version.Blocked),
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  anotherPkg,
				Version:     "2.0.0",
				VersionType: resolve.Requirement,
			},
			Type: deptest.Must(deptest.ParseString("Opt")),
		}},
	}},
}, {
	PackageKey: anotherPkg,
	Versions: []Version{{
		VersionKey: resolve.VersionKey{
			PackageKey:  anotherPkg,
			VersionType: resolve.Concrete,
			Version:     "1.0.0",
		},
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  pkg1,
				Version:     "1.0.0",
				VersionType: resolve.Requirement,
			},
			Type: deptest.Must(deptest.ParseString("Dev")),
		}},
	}, {
		VersionKey: resolve.VersionKey{
			PackageKey:  anotherPkg,
			VersionType: resolve.Concrete,
			Version:     "2.0.0",
		},
		Attr: buildVersionAttr(version.Blocked, version.Redirect, "go somewhere else"),
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  pkg1,
				Version:     "2.0.0",
				VersionType: resolve.Requirement,
			},
		}},
	}},
}, {
	PackageKey: pkg2,
	Versions: []Version{{
		VersionKey: resolve.VersionKey{
			PackageKey:  pkg2,
			VersionType: resolve.Concrete,
			Version:     "0.0.0",
		},
		Requirements: []resolve.RequirementVersion{{
			VersionKey: resolve.VersionKey{
				PackageKey:  pkg2,
				Version:     "v0.0.0",
				VersionType: resolve.Requirement,
			},
			Type: deptest.Must(deptest.ParseString("XTest someXTest")),
		}, {
			VersionKey: resolve.VersionKey{
				PackageKey:  pkg1,
				Version:     "1.0.0",
				VersionType: resolve.Requirement,
			},
			Type: deptest.Must(deptest.ParseString(`Framework "some framework" Opt Dev`)),
		}},
	}},
}, {
	PackageKey: scopedPkg,
	Versions: []Version{{
		VersionKey: resolve.VersionKey{
			PackageKey:  scopedPkg,
			VersionType: resolve.Concrete,
			Version:     "1.1.1",
		},
	}},
}}}

func buildVersionAttr(args ...any) version.AttrSet {
	var attr version.AttrSet
	for i := 0; i < len(args); i++ {
		key := args[i].(version.AttrKey)
		value := ""
		if key > 0 {
			i++
			value = args[i].(string)
		}
		attr.SetAttr(key, value)
	}
	return attr
}

func TestParse(t *testing.T) {
	s, err := New(testSchema, resolve.NPM)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantTestSchema, s); diff != "" {
		t.Errorf("New: (- want, + got):\n%s", diff)
	}
}

const versionFirst = `
	1.0.0
`
const importFirst = `
		repo@ver
`
const skipVersion = `
repo
		repo@ver
`
const manyTabs = `
			wat
`
const leadingSpace = `
 wat
`

var parseErrorTests = []struct {
	text string
	want string // Error string to match
}{
	{versionFirst, `version "1.0.0" before package`},
	{importFirst, `import "repo@ver" before package`},
	{skipVersion, `import "repo@ver" before version`},
	{manyTabs, `line 1 begins with 3 tabs, which is too many`},
	{leadingSpace, `line 1 has leading space where tabs are expected`},
}

func TestParseError(t *testing.T) {
	for _, c := range parseErrorTests {
		_, err := New(c.text, resolve.NPM)
		if err == nil {
			t.Errorf("New(%q) returned nil error, want %q", c.text, c.want)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("ew(%q) returned %q, want %q", c.text, err, c.want)
		}
	}
}

const testGraph = `

alice
	1.0.0
		brook@1.0.0
	1.0.1
		bob@^1.0.0
		chris@^1.0.0
	2.0.0
		dan@^1.0.0
brook
	1.0.0
		chris@main
	1.0.1
		chris@^1.0.0
chris
	1.0.0
		dan@1.0.0
	1.0.1
dan
	ATTR: Tags main
	1.0.0
		dan/pkg@1.0.0
dan/pkg
	1.0.0
`

func TestPopulateValidate(t *testing.T) {
	schema, err := New(testGraph, resolve.NPM)
	if err != nil {
		t.Fatal(err)
	}

	client := schema.NewClient()
	if err := schema.ValidateClient(client); err != nil {
		t.Error(err)
	}
}
