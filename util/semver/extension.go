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

import (
	"unicode/utf8"
)

// nextVersionElemPos returns the position of the start of the next
// element in s, or if you prefer, the length of the element that
// begins s.
func nextVersionElemPos(s string) int {
	i := 0
	if versionCategory(s, 0) == versionSeparator {
		i++
	}
	prev, _ := versionNext(s, i)
	for i < len(s) {
		cat, wid := versionNext(s, i)
		switch cat {
		case versionSeparator:
			return i
		case versionUnknown, versionStar:
			if i == 0 {
				// A bad character will just become an element.
				i++
			}
			return i
		case versionNumeric, versionQualifier:
			if cat != prev {
				return i
			}
			prev = cat
		}
		i += wid
	}
	return len(s)
}

// These are the categories of bytes in a Maven or RubyGems version strings.
// They are used to parse the elements using the system-specific rules.
const (
	versionSeparator = iota
	versionUnknown
	versionStar
	versionQualifier
	versionNumeric // Must be > qualifier for compare methods.
	versionEOF
)

// versionCategory reports the category of s[i].
func versionCategory(s string, i int) int {
	cat, _ := versionNext(s, i)
	return cat
}

// versionNext reports the category of s[i] and the width of the rune.
func versionNext(s string, i int) (cat, wid int) {
	if i == len(s) {
		return versionEOF, 0
	}
	c, _ := utf8.DecodeRuneInString(s[i:])
	switch {
	case c == '∞':
		return versionNumeric, len("∞")
	case '0' <= c && c <= '9':
		return versionNumeric, 1
	case 'a' <= c && c <= 'z':
		return versionQualifier, 1
	case 'A' <= c && c <= 'Z':
		return versionQualifier, 1
	case c == '_':
		return versionQualifier, 1
	case c == '.', c == '-':
		return versionSeparator, 1
	case c == '*':
		return versionStar, 1
	}
	return versionUnknown, 1
}
