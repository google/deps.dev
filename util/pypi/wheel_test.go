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

package pypi

import (
	"reflect"
	"testing"
)

func TestParseWheelName(t *testing.T) {
	// TODO: Should the pyWheelInfo.Name fields go through canon.PackageName?
	cases := []struct {
		in  string
		out *WheelInfo
	}{
		{
			in: "generic-0.0.1-py2.py3-none-any.whl",
			out: &WheelInfo{
				Name:    "generic",
				Version: "0.0.1",
				Platforms: []PEP425Tag{{
					Python:   "py2",
					ABI:      "none",
					Platform: "any",
				}, {
					Python:   "py3",
					ABI:      "none",
					Platform: "any",
				}},
			},
		},
		{
			in: "very_generic-0.0.2-cp3.cp2-cp3m.cp2m-win_amd64.win32.whl",
			out: &WheelInfo{
				Name:    "very_generic",
				Version: "0.0.2",
				Platforms: []PEP425Tag{{
					Python:   "cp3",
					ABI:      "cp3m",
					Platform: "win_amd64",
				}, {
					Python:   "cp3",
					ABI:      "cp3m",
					Platform: "win32",
				}, {
					Python:   "cp3",
					ABI:      "cp2m",
					Platform: "win_amd64",
				}, {
					Python:   "cp3",
					ABI:      "cp2m",
					Platform: "win32",
				}, {
					Python:   "cp2",
					ABI:      "cp3m",
					Platform: "win_amd64",
				}, {
					Python:   "cp2",
					ABI:      "cp3m",
					Platform: "win32",
				}, {
					Python:   "cp2",
					ABI:      "cp2m",
					Platform: "win_amd64",
				}, {
					Python:   "cp2",
					ABI:      "cp2m",
					Platform: "win32",
				}},
			},
		},
		{
			in: "build_num-1.1.1.1.1-2a-cp3-cp3m-manylinux1_i686.whl",
			out: &WheelInfo{
				Name:    "build_num",
				Version: "1.1.1.1.1",
				BuildTag: WheelBuildTag{
					Num: 2,
					Tag: "a",
				},
				Platforms: []PEP425Tag{{
					Python:   "cp3",
					ABI:      "cp3m",
					Platform: "manylinux1_i686",
				}},
			},
		},
		{
			in: "long_num-1.2-12341234-cp3-cp3um-manylinux1_i686.whl",
			out: &WheelInfo{
				Name:    "long_num",
				Version: "1.2",
				BuildTag: WheelBuildTag{
					Num: 12341234,
				},
				Platforms: []PEP425Tag{{
					Python:   "cp3",
					ABI:      "cp3um",
					Platform: "manylinux1_i686",
				}},
			},
		},
		{
			in:  "too_short-py3-macosx_10_6_intel.whl",
			out: nil,
		},
		{
			in:  "obvious-too-long-1.3.4-abcd--py3-none-any.whl",
			out: nil,
		},
		{
			in:  "not-a-wheel-at-all.zip",
			out: nil,
		},
		{
			in:  "badtag-1.1-ab123-cp2.cp3-cp2d.cp3d-linux_x86_64.whl",
			out: nil,
		},
		// Some cases that are invalid are quite hard to distinguish.
		{
			in: "too-long-1.2.3-py2-none-win_amd64.whl",
			out: &WheelInfo{
				Name:    "too",
				Version: "long",
				BuildTag: struct {
					Num int
					Tag string
				}{
					Num: 1,
					Tag: ".2.3",
				},
				Platforms: []PEP425Tag{{
					Python:   "py2",
					ABI:      "none",
					Platform: "win_amd64",
				}},
			},
		},
	}
	for _, c := range cases {
		if got, err := ParseWheelName(c.in); c.out == nil && err == nil {
			t.Errorf("parse wheel name %q: want error, got: %+v", c.in, got)
		} else if c.out != nil && err != nil {
			t.Errorf("parse wheel name %q: want success, got err: %v", c.in, err)
		} else if c.out != nil && !reflect.DeepEqual(c.out, got) {
			t.Errorf("parse wheel name %q:\nwant: %#v\n got: %#v", c.in, c.out, got)
		}
	}
}
