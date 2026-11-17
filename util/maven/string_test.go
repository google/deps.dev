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

package maven

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	var got struct {
		Str String `xml:"string"`
	}
	err := xml.Unmarshal([]byte(`<metadata><string> test </string></metadata>`), &got)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if got.Str != "test" {
		t.Fatalf("unmarshal string want: %s, got: %s", "test", got.Str)
	}
}

func TestTruthyBool(t *testing.T) {
	var got struct {
		Str TruthyBool `xml:"bool"`
	}
	err := xml.Unmarshal([]byte(`<xml><bool>haha</bool></xml>`), &got)
	if err == nil {
		t.Errorf("expected error %v but got %v", fmt.Errorf("unrecognized boolean"), err)
	}

	tests := []struct {
		xml      String
		want     TruthyBool
		wantBool bool
	}{
		{"<xml><bool> true </bool></xml>", "true", true},
		{"<xml><bool>TRue</bool></xml>", "true", true},
		{"<xml><bool>FalSE</bool></xml>", "false", false},
		{"<xml><bool></bool></xml>", "", true},
	}
	for _, test := range tests {
		err = xml.Unmarshal([]byte(test.xml), &got)
		if err != nil {
			t.Errorf("failed to unmarshal: %v", err)
		}
		if got.Str != test.want {
			t.Errorf("unmarshal string want: %s, got: %s", test.want, got.Str)
		}
		if got.Str.Boolean() != test.wantBool {
			t.Errorf("Boolean(): got %v, want: %v", got.Str.Boolean(), test.wantBool)
		}
	}
}

func TestFalsyBool(t *testing.T) {
	var got struct {
		Str FalsyBool `xml:"bool"`
	}
	err := xml.Unmarshal([]byte(`<xml><bool>haha</bool></xml>`), &got)
	if err == nil {
		t.Errorf("expected error %v but got %v", fmt.Errorf("unrecognized boolean"), err)
	}

	tests := []struct {
		xml      String
		want     FalsyBool
		wantBool bool
	}{
		{"<xml><bool> true </bool></xml>", "true", true},
		{"<xml><bool>TRue</bool></xml>", "true", true},
		{"<xml><bool>FalSE</bool></xml>", "false", false},
		{"<xml><bool></bool></xml>", "", false},
	}
	for _, test := range tests {
		err = xml.Unmarshal([]byte(test.xml), &got)
		if err != nil {
			t.Errorf("failed to unmarshal: %v", err)
		}
		if got.Str != test.want {
			t.Errorf("unmarshal string want: %s, got: %s", test.want, got.Str)
		}
		if got.Str.Boolean() != test.wantBool {
			t.Errorf("Boolean(): got %v, want: %v", got.Str.Boolean(), test.wantBool)
		}
	}
}

func TestInterpolateString(t *testing.T) {
	dictionary := map[string]string{
		"foo":    "1",
		"bar":    "2",
		"recur":  "${recur}",
		"recur1": "${recur2}",
		"recur2": "${recur3}",
		"recur3": "${recur1}",
		"x":      "${y}",
		"y":      "z",
		"a":      "${b}",
		"b":      "c",
		"d":      "${a}-${x}",
		"key":    "${unknown}",
	}
	tests := []struct {
		s        String
		got      String
		want     String
		resolved bool
	}{
		{"foo", "foo", "foo", true},
		{"${foo", "${foo", "${foo", true},
		{"foo}", "foo}", "foo}", true},
		{"${foo}", "${foo}", "1", true},
		{"${foo}.${foo}", "${foo}.${foo}", "1.1", true},
		{"${foo}.${bar}", "${foo}.${bar}", "1.2", true},
		{"${foo.bar}", "${foo.bar}", "${foo.bar}", false},
		{"${${foo}}", "${${foo}}", "${${foo}}", false},
		// Unknown keys.
		{"${foo}.${unknown}", "${foo}.${unknown}", "1.${unknown}", false},
		{"${unknown}.${bar}", "${unknown}.${bar}", "${unknown}.2", false},
		// Should detect the cycle.
		{"${recur}", "${recur}", "${recur}", false},
		{"${recur1}", "${recur1}", "${recur1}", false},
		// Placeholders in a property.
		{"${x}", "${x}", "z", true},
		{"${a}-${x}", "${a}-${x}", "c-z", true},
		{"${d}", "${d}", "c-z", true},
		{"${key}", "${key}", "${unknown}", false},
	}
	for _, test := range tests {
		ok := test.got.interpolate(dictionary)
		if test.got != test.want || ok != test.resolved {
			t.Errorf("resolve(%s): got %s %t, want %s %t", test.s, test.got, ok, test.want, test.resolved)
		}
	}
}

func TestInterpolateStringLimits(t *testing.T) {
	flatDict := make(map[string]string, 64)
	var flatInput strings.Builder
	for i := range 64 {
		fmt.Fprintf(&flatInput, "${k%d}", i)
		flatDict[fmt.Sprintf("k%d", i)] = "v"
	}

	chainDict := func(depth int, leaf string) map[string]string {
		d := make(map[string]string, depth+1)
		d["valid"] = "ok"
		for i := range depth - 1 {
			d[fmt.Sprintf("p%d", i)] = fmt.Sprintf("${p%d}", i+1)
		}
		d[fmt.Sprintf("p%d", depth-1)] = leaf
		return d
	}

	expDict := func(levels int, leaf string) map[string]string {
		d := make(map[string]string, levels+1)
		for i := range levels {
			d[fmt.Sprintf("p%d", i)] = fmt.Sprintf("${p%d}${p%d}", i+1, i+1)
		}
		d[fmt.Sprintf("p%d", levels)] = leaf
		return d
	}

	maxStepsOver := String(strings.Repeat("${v}", maxInterpolateSteps+1))
	prefixOver := String(strings.Repeat("A", maxInterpolateBytes+1) + "${v}")
	unknownOver := String(strings.Repeat("A", maxInterpolateBytes-2) + "${unknown}")
	suffixOver := String("${v}" + strings.Repeat("A", maxInterpolateBytes))

	tests := []struct {
		name   string
		dict   map[string]string
		input  String
		want   String
		wantOK bool
	}{
		{
			name:   "many_distinct_flat_properties",
			dict:   flatDict,
			input:  String(flatInput.String()),
			want:   String(strings.Repeat("v", 64)),
			wantOK: true,
		},
		{
			name:   "linear_chain_at_max_steps",
			dict:   chainDict(maxInterpolateSteps, "leaf"),
			input:  "${p0}",
			want:   "leaf",
			wantOK: true,
		},
		{
			name:   "step_limit_exceeded_rolls_back_partial_writes",
			dict:   chainDict(maxInterpolateSteps, "leaf"),
			input:  "${valid}-${p0}",
			want:   "${valid}-${p0}",
			wantOK: false,
		},
		{
			name:   "at_max_steps",
			dict:   map[string]string{"v": "a"},
			input:  String(strings.Repeat("${v}", maxInterpolateSteps)),
			want:   String(strings.Repeat("a", maxInterpolateSteps)),
			wantOK: true,
		},
		{
			name:   "step_limit_exceeded",
			dict:   map[string]string{"v": "a"},
			input:  maxStepsOver,
			want:   maxStepsOver,
			wantOK: false,
		},
		{
			name:   "exponential_non_empty",
			dict:   expDict(40, "X"),
			input:  "${p0}",
			want:   "${p0}",
			wantOK: false,
		},
		{
			name:   "exponential_empty_leaf",
			dict:   expDict(25, ""),
			input:  "${p0}",
			want:   "${p0}",
			wantOK: false,
		},
		{
			name: "at_max_output_size",
			dict: map[string]string{
				"chunk": strings.Repeat("A", maxInterpolateBytes/16),
				"exact": strings.Repeat("${chunk}", 16),
			},
			input:  "${exact}",
			want:   String(strings.Repeat("A", maxInterpolateBytes)),
			wantOK: true,
		},
		{
			name: "output_size_exceeded_by_value",
			dict: map[string]string{
				"chunk": strings.Repeat("A", maxInterpolateBytes/16),
				"big":   strings.Repeat("${chunk}", 17),
			},
			input:  "${big}",
			want:   "${big}",
			wantOK: false,
		},
		{
			name:   "output_size_exceeded_by_prefix",
			dict:   map[string]string{"v": "B"},
			input:  prefixOver,
			want:   prefixOver,
			wantOK: false,
		},
		{
			name:   "output_size_exceeded_by_unknown",
			dict:   nil,
			input:  unknownOver,
			want:   unknownOver,
			wantOK: false,
		},
		{
			name:   "output_size_exceeded_by_suffix",
			dict:   map[string]string{"v": "B"},
			input:  suffixOver,
			want:   suffixOver,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input
			gotOK := got.interpolate(tc.dict)
			if got != tc.want || gotOK != tc.wantOK {
				t.Errorf("interpolate() = (len=%d, ok=%t), want (len=%d, ok=%t)", len(got), gotOK, len(tc.want), tc.wantOK)
				if len(got) <= 128 && len(tc.want) <= 128 {
					t.Errorf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}
