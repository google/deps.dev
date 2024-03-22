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
)

type String string

func (s *String) ContainsProperty() bool {
	str := string(*s)
	i := strings.Index(str, "${")
	return i >= 0 && strings.Contains(str[i+2:], "}")
}

// UnmarshalXML trims the whitespaces when unmarshalling a string.
func (s *String) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := d.DecodeElement(&str, &start); err != nil {
		return err
	}
	*s = String(strings.TrimSpace(str))
	return nil
}

func (s *String) merge(s2 String) {
	if *s == "" {
		*s = s2
	}
}

func (s *String) interpolate(dictionary map[string]string) bool {
	result, ok := interpolating(string(*s), dictionary, make(map[string]bool))
	*s = String(result)
	return ok
}

// BoolString represents a string field that holds a boolean value.
// BoolString may contain placeholders which need to be interpolated.
type BoolString string

func (bs *BoolString) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var str string
	err := d.DecodeElement(&str, &start)
	if err != nil {
		return err
	}
	str = strings.TrimSpace(str)
	if strings.Contains(str, "${") && strings.Contains(str, "}") {
		*bs = BoolString(str)
		return nil
	}
	if ss := strings.ToLower(str); ss == "true" || ss == "false" || ss == "" {
		*bs = BoolString(ss)
		return nil
	}
	return fmt.Errorf("unrecognized boolean %q", str)
}

func (bs *BoolString) interpolate(dictionary map[string]string) bool {
	result, ok := interpolating(string(*bs), dictionary, make(map[string]bool))
	*bs = BoolString(result)
	return ok
}

// interpolating resolves all property placeholders in s with their
// values defined in dictionary.
// resolving stores the key strings being resolved, it is used to detect cycles.
func interpolating(s string, dictionary map[string]string, resolving map[string]bool) (string, bool) {
	resolved := true
	var dst strings.Builder
	for {
		i := strings.Index(s, "${")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "}")
		if j < 0 {
			break
		}
		dst.WriteString(s[:i])
		s = s[i:]
		key := s[2:j]
		if exist, ok := resolving[key]; ok && exist {
			// A cycle of keys detected.
			resolved = false
			break
		}
		// Interpolation starts.
		resolving[key] = true
		if value, ok := dictionary[key]; ok {
			// Try to resolve the value.  If resolved, write the new value.
			if value, ok = interpolating(value, dictionary, resolving); !ok {
				resolved = false
			}
			dst.WriteString(value)
		} else {
			dst.WriteString(s[:j+1])
			resolved = false
		}
		// Resolution finishes.
		resolving[key] = false
		s = s[j+1:]
	}
	dst.WriteString(s)
	return dst.String(), resolved
}
