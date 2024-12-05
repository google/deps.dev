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

func TestTrusyBoolString(t *testing.T) {
	var got struct {
		Str TrusyBool `xml:"bool"`
	}
	err := xml.Unmarshal([]byte(`<xml><bool>haha</bool></xml>`), &got)
	if err == nil {
		t.Errorf("expected error %v but got %v", fmt.Errorf("unrecognized boolean"), err)
	}

	tests := []struct {
		xml      String
		want     TrusyBool
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

func TestFalsyBoolString(t *testing.T) {
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
