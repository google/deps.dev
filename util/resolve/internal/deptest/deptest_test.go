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

package deptest

import (
	"errors"
	"reflect"
	"testing"

	"deps.dev/util/resolve/dep"
)

func TestParseString(t *testing.T) {
	buildDepType := func(args ...any) dep.Type {
		var dt dep.Type
		for i := 0; i < len(args); i += 2 {
			dt.AddAttr(args[i].(dep.AttrKey), args[i+1].(string))
		}
		return dt
	}

	cases := []struct {
		s  string
		dt dep.Type
	}{
		{
			s:  "",
			dt: dep.Type{},
		},
		{
			s:  "Opt",
			dt: buildDepType(dep.Opt, ""),
		},
		{
			s:  "Dev",
			dt: buildDepType(dep.Dev, ""),
		},
		{
			s:  "Test",
			dt: buildDepType(dep.Test, ""),
		},
		{
			s:  "Opt Dev Test",
			dt: buildDepType(dep.Opt, "", dep.Dev, "", dep.Test, ""),
		},
		{
			s:  "Framework .NETStandard1.0",
			dt: buildDepType(dep.Framework, ".NETStandard1.0"),
		},
		{
			s:  "XTest xtest",
			dt: buildDepType(dep.XTest, "xtest"),
		},
		{
			s:  "Opt Dev Framework .NETStandard1.0 XTest xtest Test",
			dt: buildDepType(dep.Opt, "", dep.Dev, "", dep.Test, "", dep.Framework, ".NETStandard1.0", dep.XTest, "xtest"),
		},
		{
			s:  "Opt Framework Dev XTest Test",
			dt: buildDepType(dep.Opt, "", dep.Framework, "Dev", dep.XTest, "Test"),
		},
		{
			s:  "MavenDependencyOrigin management Scope runtime",
			dt: buildDepType(dep.MavenDependencyOrigin, "management", dep.Scope, "runtime"),
		},
		{
			s:  "Opt Scope provided MavenDependencyOrigin direct",
			dt: buildDepType(dep.Opt, "", dep.Scope, "provided", dep.MavenDependencyOrigin, "direct"),
		},
		{
			s:  "Opt EnabledDependencies ssl,serde,simd",
			dt: buildDepType(dep.Opt, "", dep.EnabledDependencies, "ssl,serde,simd"),
		},
		{
			s:  "Selector",
			dt: buildDepType(dep.Selector, ""),
		},
		{
			s:  `Opt Environment "extras == 'test'"`,
			dt: buildDepType(dep.Opt, "", dep.Environment, "extras == 'test'"),
		},
		{
			s:  `Environment "python_version < \"2\""`,
			dt: buildDepType(dep.Environment, `python_version < "2"`),
		},
		{
			s:  `Environment "python_version < \"3.7\" and implementation_name == \"cpython\""`,
			dt: buildDepType(dep.Environment, `python_version < "3.7" and implementation_name == "cpython"`),
		},
	}

	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			dt, err := ParseString(c.s)
			if err != nil {
				t.Errorf("ParseString(%q): %v", c.s, err)
			}
			if !dt.Equal(c.dt) {
				t.Errorf("unexpected dependency type:\n got: %s\nwant: %s", dt, c.dt)
			}
		})
	}
}

func TestParseStringErrors(t *testing.T) {
	cases := []struct {
		s   string
		err error
	}{
		{
			s:   "Unknown",
			err: errors.New(`unexpected key ("Unknown")`),
		},
		{
			s:   "Opt extra",
			err: errors.New(`unexpected key ("extra")`),
		},
		{
			s:   "Opt Framework .NETStandard1.0 extra",
			err: errors.New(`unexpected key ("extra")`),
		},
		{
			s:   "Framework",
			err: errors.New(`missing value for Framework`),
		},
		{
			s:   "Scope",
			err: errors.New(`missing value for Scope`),
		},
	}

	for _, c := range cases {
		if _, err := ParseString(c.s); !reflect.DeepEqual(err, c.err) {
			t.Errorf("unexpected error for %s:\n got: %v\nwant: %v", c.s, err, c.err)
		}
	}
}
