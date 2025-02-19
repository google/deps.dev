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
	"unicode/utf8"
)

// Maven-specific support - roughly equivalent to v3.6.0 / v3.8.6
// Note that v3.8.7/v3.9.0 changed some of these rules and it's not
// clear if those will be reverted yet.

// mavenExtension implements the Maven-specific parts of a Version,
// including its peculiar ordering requirements as defined in
// https://maven.apache.org/pom.html#Version_Order_Specification.
type mavenExtension struct {
	version *Version
	elems   []mavenElement
}

// newMavenExtension builds the extension object for an extant Maven Version.
func newMavenExtension(v *Version, str string) (*mavenExtension, error) {
	e := &mavenExtension{
		version: v,
	}
	return e, e.init(str)
}

// mavenVersion parses a Maven Version. The str field is set by the caller;
// all else is stored in the extension, which is constructed here and attached
// to the returned Version.
func (p *versionParser) mavenVersion() (*Version, error) {
	var err error
	p.Version.ext, err = p.Version.newExtension(p.Version.str)
	return p.Version, err
}

func (m *mavenExtension) copy(v *Version) extension {
	n := new(mavenExtension)
	*n = *m
	n.version = v
	return n
}

func (m *mavenExtension) clearPre() {
	// TODO: Implement this when Maven 3 happens.
	// For Maven version 2, it's irrelevant.
}

func (m *mavenExtension) empty() bool {
	return m == nil || len(m.elems) == 0
}

// A mavenElement describes a component of a Maven version string.
// The separator is significant to the ordering.
type mavenElement struct {
	sep byte   // Will be zero for first element.
	str string // Element value as a string.
	int int64  // Value if the element is an integer; must be sizeof(value).
}

// canon returns a canonicalized string representation of the version/extension.
// The showBuild argument is ignored; Maven doesn't have that concept.
func (m *mavenExtension) canon(showBuild bool) string {
	var b strings.Builder
	for i, e := range m.elems {
		if i > 0 {
			b.WriteByte(e.sep)
		}
		b.WriteString(e.str)
	}
	return b.String()
}

var mavenMinVersion *Version

func init() {
	mavenMinVersion = &Version{
		sys:          Maven,
		userNumCount: 1,
		isPrerelease: false,
		str:          "0.alpha",
	}
	mavenMinVersion.ext = &mavenExtension{
		version: mavenMinVersion,
		elems: []mavenElement{
			{
				sep: 0,
				str: "0",
				int: 0,
			},
			{
				sep: '.',
				str: "alpha",
				int: 0,
			},
		},
	}
}

// mavenCategory determines the category of the first rune in the string, like
// versionCategory but specifically for Maven. Maven allows any character
// anywhere in the version, but treats numbers specially and uses "." or "-" as
// separators. Also returns the width of the rune that was checked.
func mavenCategory(s string) (cat, width int) {
	if len(s) == 0 {
		return versionEOF, 0
	}
	c, w := utf8.DecodeRuneInString(s)
	switch {
	case c == '∞':
		return versionNumeric, w
	case '0' <= c && c <= '9':
		return versionNumeric, w
	case c == '.', c == '-':
		return versionSeparator, w
	}
	return versionQualifier, w
}

// nextMavenElem collects the next element of the version string s. An element
// is an optional separator, followed by a run of characters until either the
// category changes (as defined by mavenCategory) or we reach a separator.
// Returns the prefix of the input corresponding the next element and the
// remainder of the input string.
func nextMavenElem(s string) (string, string) {
	if len(s) <= 1 {
		return s, ""
	}
	var (
		i       = 0
		prev, _ = mavenCategory(s)
	)
	if prev == versionSeparator {
		i++
		prev, _ = mavenCategory(s[1:])
	}
	for i < len(s) {
		cat, w := mavenCategory(s[i:])
		if cat != prev || cat == versionSeparator {
			return s[:i], s[i:]
		}
		i += w
	}
	return s, ""
}

// init parses a Maven version string into a slice of elements and stores them in the extension.
func (m *mavenExtension) init(input string) error {
	elements := make([]mavenElement, 0, 5) // Pre-allocated to reduce allocations from growth.
	// Canonicalize elementwise.
	first := true
	prevCat := versionUnknown
	input = strings.ToLower(input)
	for str, s := "", input; s != ""; {
		var e mavenElement
		str, s = nextMavenElem(s)
		cat, _ := mavenCategory(str)
		if cat == versionUnknown {
			return fmt.Errorf("invalid version %#q", input)
		}
		if cat == versionSeparator {
			e.sep = str[0]
			str = str[1:]
			if str == "" {
				str = "0"
			}
			cat, _ = mavenCategory(str)
		} else if !first {
			e.sep = '-'
			if cat == versionNumeric {
				if prevCat == versionNumeric {
					e.sep = '.'
				} else if prevCat == versionQualifier {
					// This is a qualifer followed by a number, check for
					// the alpha/beta/milestone shortcut
					prev := len(elements) - 1
					switch elements[prev].str {
					case "a":
						elements[prev].str = "alpha"
					case "b":
						elements[prev].str = "beta"
					case "m":
						elements[prev].str = "milestone"
					}
				}
			}
		}

		e.str = str
		elements = append(elements, e)
		prevCat = cat
		first = false
	}
	// Trim step 1: remove trailing zeros at end and before each -.
	for i := 1; i < len(elements); i++ {
		if i < len(elements)-1 && elements[i+1].sep != '-' {
			continue
		}
		for i > 0 && isEmptyMavenElem(elements[i].str) {
			copy(elements[i:], elements[i+1:])
			elements = elements[:len(elements)-1]
			i--
		}
	}
	// Final step: Integers for numbers.
	for i, e := range elements {
		if cat, _ := mavenCategory(e.str); cat == versionNumeric {
			if e.str == "∞" {
				elements[i].int = int64(infinity)
			} else {
				val, err := parseNum(e.str)
				if err != nil {
					return err
				}
				elements[i].int = int64(val)
			}
		} else {
			m.version.isPrerelease = true // TODO is this right?
		}
	}
	m.elems = elements
	return nil
}

// isEmptyMavenElem reports whether is defined to be equivalent
// to the empty string for the purpose of ordering.
func isEmptyMavenElem(s string) bool {
	if s == "0" {
		return true
	}
	return mavenVersionQualifierOrder[s] == mavenEmptyQualifier
}

// mavenPadElement returns an element for padding out a short
// Version; which one depends on the separator.
func mavenPadElement(sep byte) mavenElement {
	if sep == '-' {
		return mavenElement{'-', "", 0}
	}
	return mavenElement{'.', "0", 0}
}

// compare uses Maven's rules to decide the ordering of m and e.
func (m *mavenExtension) compare(e extension) int {
	n := e.(*mavenExtension)
	as := m.elems
	bs := n.elems
	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}
	for i := 0; i < max; i++ {
		var a, b mavenElement
		var ac, bc int
		if i >= len(as) {
			a = mavenPadElement(bs[i].sep)
			ac = versionEOF
		} else {
			a = as[i]
			ac, _ = mavenCategory(a.str)
		}
		if i >= len(bs) {
			b = mavenPadElement(as[i].sep)
			bc = versionEOF
		} else {
			b = bs[i]
			bc, _ = mavenCategory(b.str)
		}
		if a == b {
			continue
		}
		// Special cases for unknown qualifiers (or positive epsilon qualifiers like SP),
		// which sort funny: 1.0 < 1.SP < 1.foo < 1.1
		if ac == versionQualifier {
			if ao := mavenVersionQualifierOrder[a.str]; ao > mavenEmptyQualifier {
				return mavenUnknownQualifierCompare(a, b, ao, bc)
			}
		}
		if bc == versionQualifier {
			if bo := mavenVersionQualifierOrder[b.str]; bo > mavenEmptyQualifier {
				return -mavenUnknownQualifierCompare(b, a, bo, ac)
			}
		}
		if ac == versionEOF { // Empty string / padding
			ac = versionQualifier
		}
		if bc == versionEOF { // Empty string / padding
			bc = versionQualifier
		}
		// Ordinary things now.
		if ac > bc {
			return 1
		}
		if ac < bc {
			return -1
		}
		if ac == versionNumeric {
			if a.sep != b.sep {
				return int(a.sep) - int(b.sep) // Magic: '-'+1 = '.'.
			}
			return sgn64(a.int, b.int)
		}
		if a.sep != b.sep {
			return int(b.sep) - int(a.sep) // Note: reversed compared to numeric. Nice.
		}
		c := compareMavenQualifier(a.str, b.str)
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

// mavenUnknownQualifierCompare handles the case where a is an unknown
// qualifier, say foo. See the comment at the call site.
func mavenUnknownQualifierCompare(a, b mavenElement, aOrder, bCategory int) int {
	switch bCategory {
	case versionQualifier:
		bOrder := mavenVersionQualifierOrder[b.str]
		if aOrder == bOrder {
			// b is also undefined or SP; use normal ordering rules.
			if a.sep != b.sep {
				return int(b.sep) - int(a.sep)
			}
			return sgnStr(a.str, b.str)
		}
		return sgn(aOrder, bOrder)
	case versionEOF:
		// Pad elements compare as 0, but there's no later non-zero elements.
		return sgn(aOrder, mavenEmptyQualifier)
	case versionNumeric:
		// x.0.y means there is an y > 0 later because we trim, so it's less than.
		return -1
	}

	// Should not happen.
	panic(bCategory)
}

const mavenEmptyQualifier = -2

// mavenVersionQualifierOrder defines the order in which predefined qualifiers sort.
// 0 is deliberately missing, so unrecognized elements sort as 0, above all these.
var mavenVersionQualifierOrder = map[string]int{
	"alpha":     mavenEmptyQualifier - 5,
	"beta":      mavenEmptyQualifier - 4,
	"milestone": mavenEmptyQualifier - 3,
	"rc":        mavenEmptyQualifier - 2,
	"cr":        mavenEmptyQualifier - 2, // sic.
	"snapshot":  mavenEmptyQualifier - 1,
	"":          mavenEmptyQualifier,
	"release":   mavenEmptyQualifier, // Undocumented but prevalent.
	"final":     mavenEmptyQualifier,
	"ga":        mavenEmptyQualifier,
	"sp":        mavenEmptyQualifier + 1,
}

func compareMavenQualifier(a, b string) int {
	aOrder := mavenVersionQualifierOrder[a]
	bOrder := mavenVersionQualifierOrder[b]
	if aOrder < 0 || bOrder < 0 { // One or the other is predefined.
		return sgn(aOrder, bOrder)
	}
	return sgnStr(a, b)
}

// num returns the number of the i'th element. If the element
// is empty or not a number, it returns 0.
func (m *mavenExtension) num(i int) int64 {
	if i >= len(m.elems) {
		return 0
	}
	str := m.elems[i].str
	// By the rules of building elements, we know that if the
	// first byte is a digit, they all are.
	if len(str) == 0 || str[0] < '0' || '9' < str[0] {
		return -1
	}
	return m.elems[i].int
}

// mavenDifference is a weak version of the Difference method. If the first
// difference appears when we have seen up to three numbers and nothing else, it
// returns MajorDiff etc. For anything else it returns OtherDiff. When called,
// we know that there is some difference.
func mavenDifference(u, v *Version) Diff {
	// If we have numeric versions up to 3, we go with it.
	ue := u.ext.(*mavenExtension)
	ve := v.ext.(*mavenExtension)

	switch {
	case ve.num(nMajor) != ue.num(nMajor):
		return DiffMajor
	case ve.num(nMinor) != ue.num(nMinor):
		return DiffMinor
	case ve.num(nPatch) != ue.num(nPatch):
		return DiffPatch
	}

	// Otherwise too hard.
	return DiffOther
}
