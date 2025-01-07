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

package versiontest

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve/version"
)

func buildAttr(args ...any) version.AttrSet {
	var attr version.AttrSet
	for i := 0; i < len(args); i += 2 {
		attr.SetAttr(args[i].(version.AttrKey), args[i+1].(string))
	}
	return attr
}

func TestParseString(t *testing.T) {
	cases := []struct {
		s    string
		attr version.AttrSet
	}{
		{
			s:    "",
			attr: version.AttrSet{},
		},
		{
			s:    "Blocked",
			attr: buildAttr(version.Blocked, ""),
		},
		{
			s:    "Error",
			attr: buildAttr(version.Error, ""),
		},
		{
			s:    "Blocked Redirect there",
			attr: buildAttr(version.Blocked, "", version.Redirect, "there"),
		},
		{
			s:    `Features {"default":[]}`,
			attr: buildAttr(version.Features, `{"default":[]}`),
		},
	}

	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			attr, err := ParseString(c.s)
			if err != nil {
				t.Errorf("ParseString(%q): %v", c.s, err)
			}
			if !attr.Equal(c.attr) {
				t.Errorf("unexpected version attribute:\n got: %s\nwant: %s", attr, c.attr)
			}
			s := String(attr)
			dt2, err := ParseString(s)
			if err != nil {
				t.Errorf("ParseString(%q): %v", s, err)
			}
			if !dt2.Equal(attr) {
				t.Errorf("unexpected attr != ParseString(String(attr)):\n got: %s\nwant: %s", dt2, attr)
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
			s:   "Blocked extra",
			err: errors.New(`unexpected key ("extra")`),
		},
		{
			s:   "Blocked Redirect there extra",
			err: errors.New(`unexpected key ("extra")`),
		},
		{
			s:   "Redirect",
			err: errors.New(`missing value for Redirect`),
		},
	}

	for _, c := range cases {
		_, err := ParseString(c.s)
		if diff := cmp.Diff(err, c.err, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error for %s:\n(-got, +want):\n%s", c.s, diff)
		}
	}
}

func TestParseSingle(t *testing.T) {
	for _, c := range []struct {
		in   string
		want version.AttrSet
	}{
		{`Blocked`, buildAttr(version.Blocked, "")},
		{`Features "quoted value"`, buildAttr(version.Features, "quoted value")},
		{"Features `quoted value`", buildAttr(version.Features, "quoted value")},
		{`Features "quoted \"value\""`, buildAttr(version.Features, `quoted "value"`)},
	} {
		got, err := ParseSingle(c.in)
		if err != nil {
			t.Errorf("ParseSingle(%q): %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseSingle(%q): got %v, want %v", c.in, got, c.want)
		}
	}
}
