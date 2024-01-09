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
	"fmt"
	"strings"
)

// RubyGems-specific support.

// gemExtension implements the RubyGems-specific parts of a Version,
// including its ordering requirements as defined in
// https://github.com/rubygems/rubygems/blob/master/lib/rubygems/version.rb.
// It uses the numbers from the version but breaks the prerelease part, if any,
// into separate elements and stores those.
type gemExtension struct {
	version *Version
	elems   []gemElement
}

// newGemExtension builds the extension object for an extant RubyGems Version.
func newGemExtension(v *Version, str string) (*gemExtension, error) {
	e := &gemExtension{
		version: v,
	}
	err := e.init(str)
	return e, err
}

// gemVersion parses a RubyGems Version. The str field is set by the caller;
// all else is stored in the extension, which is constructed here and attached
// to the returned Version.
func (p *versionParser) gemVersion() (*Version, error) {
	var err error
	p.Version.ext, err = p.Version.newExtension(p.Version.str)
	return p.Version, err
}

func (g *gemExtension) copy(v *Version) extension {
	n := new(gemExtension)
	*n = *g
	n.version = v
	return n
}

func (g *gemExtension) clearPre() {
	g.elems = nil
}

func (g *gemExtension) empty() bool {
	return g == nil || len(g.elems) == 0
}

// A gemElement describes a component of a Gems version string.
// The separator is significant to the ordering.
type gemElement struct {
	str string // Element value as a string.
	int int64  // Value if the element is an integer; must be sizeof(value).
}

// canon returns a canonicalized string representation of the version/extension.
// It is canonical for us, not for RubyGems, in that the first prerelease element
// is always separated by a '-' to handle the case "1.2.3b2", in which the 3 is
// part of the prerelease, not a version number.
// The showBuild argument is ignored; Gems doesn't have that concept.
func (g *gemExtension) canon(showBuild bool) string {
	var b strings.Builder
	n := g.version.atLeast3()
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte('.')
		}
		fmt.Fprint(&b, g.version.getNum(i))
	}
	if len(g.elems) > 0 {
		b.WriteByte('-')
		printed := false
		for i, e := range g.elems {
			if i == 0 && e.str == "pre" {
				continue
			}
			if printed {
				b.WriteByte('.')
			}
			b.WriteString(e.str)
			printed = true
		}
	}
	return b.String()
}

var rubyGemsMinVersion *Version

func init() {
	rubyGemsMinVersion = &Version{
		sys:          RubyGems,
		userNumCount: 3,
		isPrerelease: false,
		str:          "0.0.0.a0",
	}
	rubyGemsMinVersion.num = rubyGemsMinVersion.buf[:3]
	rubyGemsMinVersion.ext = &gemExtension{
		version: rubyGemsMinVersion,
		elems: []gemElement{
			{
				str: "a",
				int: 0,
			},
		},
	}
}

// init parses a Gems version prerelease string into a slice of elements and
// stores them in the extension. The version has already been parsed from
// the input. We can pull the prerelease out by looking for '-' or alphas.
func (g *gemExtension) init(input string) error {
	// Prerelease starts at the earlier of '-' or an alphabetic.
	input = strings.ToLower(input)
	preStart := -1
	for i, c := range input {
		if c == '-' || 'a' <= c && c <= 'z' {
			preStart = i
			break
		}
	}
	if preStart < 0 {
		return nil
	}
	input = input[preStart:]
	elements := make([]gemElement, 0, 5) // Pre-allocated to reduce allocations from growth.
	// Canonicalize the prerelease elementwise.
	for s, i := input, 0; s != ""; s = s[i:] {
		var str string
		if s[0] == '-' {
			str = "pre" // RubyGems does this, changing "-" into ".pre.".
			i = 1
		} else {
			i = nextVersionElemPos(s)
			str = s[:i]
		}
		cat := versionCategory(str, 0)
		if cat == versionUnknown {
			return fmt.Errorf("invalid version %#q", input)
		}
		if cat == versionSeparator {
			str = str[1:]
		}
		if str == "" {
			str = "0"
			cat = versionNumeric
		}
		elements = append(elements, gemElement{str: str})
	}
	// Trim trailing zeros.
	for i := len(elements) - 1; i >= 0; i-- {
		if elements[i].str == "0" {
			elements = elements[:i]
		}
	}
	// Integers for numbers.
	for i, e := range elements {
		if versionCategory(e.str, 0) == versionNumeric {
			if e.str == "∞" {
				elements[i].int = int64(infinity)
			} else {
				val, err := parseNum(e.str)
				if err != nil {
					return err
				}
				elements[i].int = int64(val)
			}
		}
	}
	g.elems = elements
	return nil
}

// compare uses RubyGems's rules to decide the ordering of the receiver and argument.
func (g *gemExtension) compare(e extension) int {
	h := e.(*gemExtension)

	// Start with numbers.
	n := len(g.version.num)
	if len(h.version.num) > n {
		n = len(h.version.num)
	}
	for i := 0; i < n; i++ {
		if s := sgnv(g.version.getNum(i), h.version.getNum(i)); s != 0 {
			return s
		}
	}

	// Numbers are the same
	// A version with zero pres dominates any non-zero number.
	switch {
	case len(g.elems) == 0 && len(h.elems) == 0:
		return 0
	case len(g.elems) == 0:
		return 1
	case len(h.elems) == 0:
		return -1
	}

	// Both have prereleases.

	as := g.elems
	bs := h.elems
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		var a, b gemElement
		if i >= len(as) {
			a = gemElement{"0", 0}
		} else {
			a = as[i]
		}
		if i >= len(bs) {
			b = gemElement{"0", 0}
		} else {
			b = bs[i]
		}
		if a == b {
			continue
		}
		// Numbers > Strings always.
		ac := versionCategory(a.str, 0)
		bc := versionCategory(b.str, 0)
		if ac == versionEOF { // Empty string
			ac = versionQualifier
		}
		if bc == versionEOF { // Empty string
			bc = versionQualifier
		}
		if ac > bc {
			return 1
		}
		if ac < bc {
			return -1
		}
		if ac == versionNumeric {
			return sgn64(a.int, b.int)
		}
		c := strings.Compare(a.str, b.str)
		if c == 0 {
			continue
		}
		return c
	}
	if len(bs) > len(as) {
		return -1
	}
	return 0
}
