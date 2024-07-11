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

/*
Package semver handles versions as defined by semver.org Version 2.0.0
as well as constraints applied to them.

By default the package is permissive, honoring practice rather than
the standard, since no packaging systems implement the standard
strictly. The System type controls which packaging system's rules
apply.

A semantic version string is a sequence of dot-separated components,
usually numbers that specify a version, possibly followed by
alphanumeric tags. The specification is at https://semver.org, but
this practice is variant. For instance, RubyGems does not follow
it exactly in that a pre-release tag can be introduced with a period
rather than a hyphen. More generally, semver.org requires three
version numbers (major, minor, patch) but in practice some are often
missing and sometimes, especially with RubyGems, there may be more
than three.

The default syntax is parsed as follows. Variations are discussed below.

No spaces may appear in a version string, and version strings are
entirely printable ASCII from the set:

	0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.+-

	- There must be one, two or three dot-separated components.
	  If there are fewer than three, the missing ones are taken to be 0.
	- Those components must be unsigned decimal integers.
	- Following those may appear zero, one or two types of alphanumeric
	  suffixes. If present (either may be absent) they are, in this order:
		- A hyphen followed by a dot-punctuated pre-release tag. For RubyGems,
		  we also allow this to be introduced by a period instead of a hyphen.
		- A plus sign followed by a dot-punctuated build tag.
	  There may be multiple tags of each type, but only in this order.

Except for elements of the build tag, numbers may not begin with a
zero (except of course for literal value 0).

Comparisons between versions match the semver.org specification:
  - Corresponding version components in order are compared. Numerical values
    are compared numerically, non-numeric ones lexically.
  - Any pre-release tagged version compares earlier than an equivalent
    non-pre-release.
  - Build tags are ignored in comparison.

Variants:

	NPM
		A version string may begin with one or more 'v' characters.
	Go
		A version string must begin with one 'v' character.
	RubyGems
		There may be more than 3 numbers.
		A prerelease tag may be separated by a period rather than a
		than a hyphen. Such a prerelease tag must not be numeric.

The constraint grammar is derived from documentation, examples, and
examination of public usage. (There is no standard uniform constraint
algebra; Semver only specifies versions.) Again, we first describe
the default grammar (that of DefaultSystem) and then list variants.

The grammar is written assuming VERSION and WILDCARD as terminals,
with syntax defined above except that WILDCARD requires at least
one of major, minor, or patch numbers to be a single wildcard
character from the set

	*xX

although PyPI accepts only * as a wildcard character.
Items within a constraint can be separated by spaces. The character
set is that of versions with the addition of

	*^~><=

The default constraint grammar is listed here. Comments identify
which non-default system supports each binary operator.

	constraint = orList

	orList = andList
		| orList '||' andList // NPM only.

	andList = value
		| andList value
		| andList ',' value // Cargo only, but see below re: RubyGems

	value = VERSION
		| span
		| unop VERSION
		| WILDCARD

	unop = '='
		| '==' // PyPI only; other systems use '='.
		| '>'
		| '>='
		| '<'
		| '<='
		| '^'
		| '~'
		| '~>'

	span = VERSION ' ' '-' ' ' VERSION // NPM only.

A span must be the only item in an andList.
That is, a span may not be "anded", only "ored".

Operators supported:

	NPM
		= > >= < <= ^ ~ ~>
	Cargo
		= > >= < <= ^ ~
		A missing (empty) operator defaults to ^.
	Go
		None.
	Python
		== > >= < <= ~= ~=
		In Python, ~= is the same as RubyGems ~>.
	RubyGems
		= != > >= < <= ~>

Other variants:

	Go
		The constraint grammar is just a version, which then represents
		a constraint as follows:
			v.1.2.3 means >=1.2.3 and <2.0.0.
	RubyGems
		Officially RubyGems does not support comma, but the web site and
		its API print lists of constraints separated by commas, so we
		accept them here.
	Maven
		TODO: The implementation supports Maven version 2 only.
		Maven uses a grammar with open and closed ranges and
		comma-separated or-ed together range lists:
			[2.0,2.1),[3.0.0,3.4.0]
		means (2.0 <= v && v < 2.1) || (3.0.0 <= v && v <= 3.4.0).
		Versions can be omitted, so (,2.0) means v <= 2.0 and
		(,) means everything. Version syntax is standard, without
		wildcards.
	NuGet
		NuGet uses a set grammar with the same syntax as Maven.
		Version syntax permits * as a wildcard.
*/
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

const eof rune = -1

// System identifies a packaging system: RubyGems, NPM, etc. The "DefaultSystem"
// value represents a fictional but accommodating semantic versioning standard
// that permits most common operations and syntaxes. For more precise adherence to the
// rules of a known packaging system such as RubyGems, use the corresponding
// System value.
type System byte

//go:generate stringer -type System

// Supported systems.
const (
	DefaultSystem System = iota // Unknown or default.
	Cargo
	Go
	Maven
	NPM
	NuGet
	PyPI
	RubyGems
	Composer
)

// supportsAnd reports whether the system supports space or comma as an
// AND operator in its constraint grammar.
func (sys System) supportsAnd() bool {
	switch sys {
	case DefaultSystem, NPM, PyPI, RubyGems:
		return true
	default:
		return false
	}
}

// Version represents a specific semantic version, or possibly a wildcard pattern.
type Version struct {
	sys          System    // Packaging system in which version was expressed.
	userNumCount int16     // Number of numbers provided by user.
	isPrerelease bool      // The user provided a prerelease string.
	str          string    // Original representation.
	buf          [3]value  // Backing store for num; usually all that's needed. Avoids allocation.
	num          []value   // Dot-separated numerical components.
	pre          []string  // Must be kept as individual elements for comparison, stripped of leading '-'.
	build        string    // Build tags; concatenated for efficiency (unlike with pre); the '+' is present.
	ext          extension // Only set for some Systems (Maven, RubyGems).
}

type extension interface {
	canon(showBuild bool) string
	compare(e extension) int
	copy(*Version) extension // Return a copy of the receiver.
	clearPre()               // Clear the prerelease for this version.
	empty() bool             // Extension has no useful information.
}

const (
	nMajor = iota
	nMinor
	nPatch
)

// newVersion returns a new Version with the specified system, user string,
// and prerelease tags. The major, minor, and patch are set to val. The
// extension is created from the extension string.
func newVersion(sys System, str, ext string, val value, pre []string) (*Version, error) {
	v := &Version{
		sys: sys,
		str: str,
		pre: pre,
	}
	for i := 0; i < 3; i++ {
		v.setNum(i, val)
	}
	if v.ext != nil {
		var err error
		v.ext, err = v.newExtension(ext)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

type versionParser struct {
	*Version // Accumulator for result.
	lex      lexer
}

// Parse returns the result of parsing the version string in the packaging
// system.
// The syntax is System-dependent and is defined in the package comment.
func (sys System) Parse(str string) (*Version, error) {
	if !sys.possibleVersionString(str) {
		return nil, fmt.Errorf("invalid version %#q", str)
	}
	return sys.parse(str, false)
}

func (sys System) parse(str string, allowInfinity bool) (*Version, error) {
	version := &Version{
		sys: sys,
		str: str,
	}
	parser := versionParser{
		Version: version,
		lex: lexer{
			str:           str,
			allowInfinity: allowInfinity,
		},
	}
	return parser.version()
}

// possibleVersionString reports whether the string has a chance of being a
// valid version by checking the first few bytes. It's used only as a quick
// check for likely valid versions before spending time parsing, so the "false"
// must be accurate but the "true" could be optimistic. Valid semver versions
// usually start with a 'v' (optional if NPM, PyPI; required if Go) and one or two
// numbers followed by a full stop so the first few bytes will be enough to
// filter out most invalid versions.
func (sys System) possibleVersionString(str string) bool {
	switch sys {
	// Any string can be a Maven version.
	case Maven:
		return true
	// For NPM, PyPI, Go and Composer, peel off leading v's. For NPM, there can be many.
	case NPM:
		str = strings.TrimLeft(str, "v")
	case PyPI, Composer:
		if len(str) > 0 && (str[0] == 'v' || str[0] == 'V') {
			str = str[1:]
		}
	case Go:
		if !strings.HasPrefix(str, "v") {
			return false
		}
		str = str[1:]
	}
	if len(str) == 0 {
		return false
	}
	if len(str) > 3 {
		str = str[:3]
	}
	for i, c := range str {
		switch c {
		case '.', '-', '+':
			return i != 0
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '*', 'x', 'X':
			// ok
		case '!', '_':
			// PyPI epoch.
			if sys != PyPI {
				return false
			}
		default:
			// PyPI doesn't require punctuation, so 1a0 is legal.
			// The charset is limited, though, and set in pep440.go.
			if sys == PyPI && i > 0 && strings.ContainsRune(lettersInPyPI, c) {
				continue
			}
			return false
		}
	}
	return true
}

func (v *Version) String() string {
	return v.str // TODO: Update if version has been modified?
}

// Canon returns a canonicalized string representation of the version.
// The showBuild argument specifies whether to include the build metadata.
func (v *Version) Canon(showBuild bool) string {
	// Build metadata is always omitted in NuGet canonicalisation.
	if v.sys == NuGet {
		showBuild = false
	}
	if v.ext != nil {
		return v.ext.canon(showBuild)
	}
	var b strings.Builder
	// Go requires leading 'v'.
	if v.sys == Go {
		b.WriteByte('v')
	}
	v.printNums(&b)
	if v.IsWildcard() {
		return b.String() // Metadata is irrelevant.
	}
	for i, pre := range v.pre {
		if i == 0 {
			b.WriteByte('-')
		} else {
			b.WriteByte('.')
		}
		if v.sys == NuGet {
			fmt.Fprint(&b, strings.ToLower(pre))
		} else {
			fmt.Fprint(&b, pre)
		}
	}
	if showBuild {
		fmt.Fprint(&b, v.build)
	}
	return b.String()
}

func (v *Version) printNums(b *strings.Builder) {
	v.printNumsN(b, v.atLeast3())
}

func (v *Version) printNumsN(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		val := v.getNum(i)
		if i > 0 {
			b.WriteByte('.')
		}
		if val == wildcard {
			b.WriteByte('*')
			break
		} else {
			fmt.Fprint(b, val)
		}
	}
}

// IsWildcard reports whether the Version contains wildcards.
// A nil Version is not a wildcard.
func (v *Version) IsWildcard() bool {
	if v == nil {
		return false
	}
	for _, n := range v.num {
		if n == wildcard {
			return true
		}
	}
	return false
}

// allNumbers reports whether the Version contains only numbers, no wildcards.
// Note: It does not check for infinity, only wildcard.
func (v *Version) allNumbers() bool {
	if v == nil {
		return false
	}
	return !v.IsWildcard()
}

// IsPrerelease reports whether the Version contains a pre-release suffix.
// TODO: Does this need to be system-independent?
func (v *Version) IsPrerelease() bool {
	return v != nil && v.isPrerelease
}

// IsBuild reports whether the Version contains a build suffix.
func (v *Version) IsBuild() bool {
	return len(v.build) > 0
}

// Prerelease returns the prelease tags, if any, including the leading hyphen.
// TODO: What should Maven return here?
func (v *Version) Prerelease() string {
	if len(v.pre) == 0 {
		return ""
	}
	return "-" + strings.Join(v.pre, ".")
}

// version
//
//	1
//	1.2
//	1.2.3
//	1.2.3-alpha
//	1.2.3.alpha   RubyGems non-compliant version.
//	1.2.3+build.2
//	1.2.3-alpha.3+build.2
//
// ... all possibly containing wildcards.
// If NPM, there may be a leading 'v' - or many 'v's.
func (p *versionParser) version() (*Version, error) {
	sys := p.Version.sys
	// Special case for testing, to avoid needing to handle it in every
	// custom parser.
	if p.lex.allowInfinity && p.str == "∞.∞.∞" {
		// Return an "infinity"
		p.Version.num = p.Version.buf[:3]
		p.Version.setMajor(infinity)
		p.Version.setMinor(infinity)
		p.Version.setPatch(infinity)
		return p.Version, nil
	}

	// Some systems just don't implement anything like semver proper.
	// These have separate implementations based on their extension.
	switch sys {
	case Maven:
		return p.mavenVersion()
	case PyPI:
		return p.pep440Version()
	}
	// Get rid of leading v's.
	switch sys {
	case NPM:
		for p.lex.peek() == 'v' {
			p.lex.next()
		}
	case Go:
		if p.lex.next() != 'v' {
			p.lex.setErr("Go versions require leading 'v'")
		}
	case Composer:
		if p.lex.peek() == 'v' || p.lex.peek() == 'V' {
			p.lex.next()
		}

	}
	// Semver requires 3 numbers, but we allow 1, 2, or 3 before canonicalization.
	// In RubyGems, the maximum number of numbers is unbounded.
	if !p.number() {
		p.lex.setErr("no number in version string")
		return nil, p.lex.err
	}
	r := p.lex.next()
	for i := 0; r == '.' && p.number(); i++ {
		r = p.lex.next()
	}
	// NuGet ignores a final 0 if it is the fourth number.
	if sys == NuGet && len(p.Version.num) == 4 && p.Version.getNum(3) == 0 {
		p.Version.num = p.Version.num[:3]
	}
	// We allow ".abc" for RubyGems, even if it's the second or third number.
	// TODO NuGet early versions allow . as prerelease separator.
	if r == '.' && len(p.Version.num) < 3 && sys != RubyGems {
		switch p.lex.peek() {
		case '.', eof:
			p.lex.setErr("empty component")
		}
		p.lex.setErr("non-numeric version")
		return nil, p.lex.err
	}
	// RubyGems allows things like 1.2.3b5 meaning 1.2.3-b5. Catch that here by injecting a minus.
	if p.Version.sys == RubyGems && isAlphanumeric(r) {
		p.lex.back()
		r = '-'
	}
	if r == '-' {
		// Go doesn't allow prereleases without three version numbers.
		if p.Version.sys == Go && len(p.Version.num) < 3 {
			p.lex.setErr("prerelease with truncated version")
			return nil, p.lex.err
		}
		p.isPrerelease = true
		r = p.metadata(&p.pre, false, "pre-release")
	} else if r == '*' && sys == NuGet {
		// NuGet allows for the metadata part to start and end with an
		// asterisk.
		r = p.lex.next()
		if r != eof {
			p.isPrerelease = true
			r = p.metadata(&p.pre, false, "pre-release")
			l := p.pre[len(p.pre)-1]
			if l[len(l)-1] != '*' {
				p.lex.setErr("missing asterisk at end of prerelease")
				return nil, p.lex.err
			}
		}
	} else if sys == RubyGems && r == '.' {
		p.isPrerelease = true
		// RubyGems allows a . for pre-release separator. Semver.org does not.
		r = p.metadata(&p.pre, false, "pre-release")
	}
	if r == '+' && sys != RubyGems { // RubyGems does not support build tags.
		// Go doesn't allow build tags without three version numbers.
		if p.Version.sys == Go && len(p.Version.num) < 3 {
			p.lex.setErr("build tag with truncated version")
			return nil, p.lex.err
		}
		start := p.lex.pos - 1
		r = p.metadata(nil, true, "build")
		p.Version.build = p.lex.str[start:p.lex.pos]
	}
	if r != eof {
		p.lex.setErr("invalid text in version string")
	}
	p.Version.userNumCount = int16(len(p.Version.num))
	// PyPI and RubyGems behave as if all missing digits are zero.
	// TODO: NuGet appears to as well, but this must be verified.
	switch sys {
	case RubyGems, NuGet:
		for len(p.Version.num) < 3 {
			p.Version.addNum(0)
		}
	}
	if p.lex.err != nil {
		return nil, p.lex.err
	}
	if sys == RubyGems {
		return p.gemVersion()
	}
	return p.Version, nil
}

// metadata parses a pre-release or build metadata list and stores the
// result in its slice argument. The type identifies the metatadata variety
// for error messages: "pre-release" or "build". The return value is the
// rune that stopped the parse.
func (p *versionParser) metadata(sp *[]string, build bool, typ string) rune {
	var r rune
	var n int
	for {
		elem, ok := p.elem(build)
		if !ok {
			break
		}
		if sp != nil {
			*sp = append(*sp, elem)
		}
		n++
		r = p.lex.next()
		if r != '.' {
			break
		}
	}
	if n == 0 {
		p.lex.setErr("empty " + typ + " metadata")
	}
	return r
}

// number reports whether the next token is a number.
// If it is, the value is remembered.
func (p *versionParser) number() bool {
	start := p.lex.pos
	if p.lex.allowInfinity && p.lex.peek() == '∞' {
		p.lex.next()
		return p.addNum(infinity)
	}
	for p.lex.digit() {
	}
	if p.lex.pos == start {
		if start < len(p.str) {
			// No digits. Might be a wildcard, if allowed.
			if p.sys.validWildcard(rune(p.str[start])) {
				if p.sys == NuGet && p.Version.IsWildcard() {
					p.lex.setErr("version has multiple wildcards")
				}
				p.lex.pos++
				return p.addNum(wildcard)
			}
		}
		return false
	}
	// No leading zero allowed, but of course just plain 0 is OK and we are forgiving in RubyGems
	// NPM also allows them, if by accident.
	if p.lex.pos > start+1 && p.lex.str[start] == '0' {
		switch p.Version.sys {
		case NPM, NuGet, RubyGems, Composer:
		default:
			p.lex.setErr("number has leading zero")
			return false
		}
	}

	val, err := parseNum(p.lex.str[start:p.lex.pos])
	if err != nil {
		p.lex.setError(err)
		return false
	}
	if p.sys == NuGet && p.Version.IsWildcard() {
		p.lex.setErr("wildcard in middle of version")
	}
	return p.addNum(val)
}

func (p *versionParser) addNum(v value) bool {
	if len(p.Version.num) == 3 {
		switch p.Version.sys {
		case NuGet, PyPI, RubyGems, Composer:
			// OK
		default:
			p.lex.setErr("more than 3 numbers present")
		}
	}
	if p.Version.sys == NuGet && len(p.Version.num) == 4 {
		// Some old packages have many elements but
		// as of Jan 2020 NuGet rejects them.
		p.lex.setErr("more than 4 numbers present")
		return false
	}

	if v > infinity {
		p.lex.setErr("numerical component too large")
	}
	p.Version.addNum(v)
	return true
}

func (v *Version) addNum(val value) {
	if v.num == nil {
		v.num = v.buf[:0]
	}
	v.num = append(v.num, val)
}

// parseNum parses the string as a number to be used as a semver version.
// It checks against overflow and infinity. The latter check is not required
// in isNumeric, which is a more general parser.
func parseNum(s string) (value, error) {
	if len(s) == 1 {
		// Easy, fast case.
		c := s[0]
		if '0' <= c && c <= '9' {
			return value(c - '0'), nil
		}
		return 0, fmt.Errorf("illegal number syntax: %s", s)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	val := value(n)
	if err != nil {
		return val, err
	}
	if n < 0 || val >= infinity {
		return 0, fmt.Errorf("number out of range: %s", s)
	}
	return val, nil
}

// isNumeric reports the value of the string as a decimal, if it is a valid decimal.
// It is used in prerelease strings and infinity is not a consideration, and also
// handles a special NPM case.
func isNumeric(sys System, s string) (int64, bool) {
	// Peculiar contentious case: a numerical string beginning with a zero
	// is not a number. See https://github.com/semver/semver/issues/181.
	if len(s) > 1 && s[0] == '0' {
		// Contradicting semver.org, NPM accepts strings (len > 1) with
		// a leading zero as a number.
		if sys != NPM {
			return 0, false
		}
	}
	var (
		n   int64
		err error
	)
	if sys == NuGet {
		// NuGet uses int32 for prerelease components.
		n, err = strconv.ParseInt(s, 10, 32)
	} else {
		n, err = strconv.ParseInt(s, 10, 64)
	}
	if err != nil {
		return 0, false
	}
	return n, true
}

// elem reports whether the next item is an alphanumeric item, and returns it.
func (p *versionParser) elem(isBuild bool) (string, bool) {
	start := p.lex.pos
	accept := p.lex.alphanumericOrHyphen
	if p.sys == NuGet {
		var seenWildCard bool
		accept = func() bool {
			if p.lex.alphanumericOrHyphen() {
				return true
			}
			if p.lex.next() == '*' && !seenWildCard {
				seenWildCard = true
				return true
			}
			p.lex.back()
			return false
		}
	}
	for accept() {
	}
	if p.lex.pos == start {
		if p.lex.peek() == '.' {
			p.lex.setErr("empty component")
		}
		return "", false
	}
	return p.lex.str[start:p.lex.pos], p.lex.pos > start
}

// Compare compares the two strings, considering them as versions in the
// (single) packaging system. It returns -1 if str1 represents an earlier
// version, +1 a later version, and 0 if they are equal. If versions match
// numerically, tags are compared lexicographically.
// Comparison of otherwise valid versions from different Systems
// returns the comparison of the System values in unspecified but
// stable order.
// A nil version compares below a non-nil version.
// Pre-release versions compare earlier than otherwise equal non-prerelease versions.
// Invalid version strings compare earlier than valid ones.
// Build metadata is ignored.
// Comparison ordering is defined by semver.org Version 2.0.0.
func (sys System) Compare(str1, str2 string) int {
	v1, err1 := sys.Parse(str1)
	v2, err2 := sys.Parse(str2)
	switch {
	case err1 == nil && err2 != nil:
		return 1
	case err1 != nil && err2 == nil:
		return -1
	case err1 != nil || err2 != nil:
		return 0
	}
	return compare(v1, v2)
}

// Compare compares two versions. See the Compare func for the semantics.
func (v *Version) Compare(o *Version) int { return compare(v, o) }

func compare(v1, v2 *Version) int {
	if v1 == v2 {
		return 0
	}
	if v1 == nil || v2 == nil { // Can happen with empty spans.
		if v1 == nil {
			return -1
		}
		return 1
	}

	if v1.sys != v2.sys {
		return sgn(int(v1.sys), int(v2.sys))
	}

	if v1.ext != nil && v2.ext != nil {
		return v1.ext.compare(v2.ext)
	}

	n := len(v1.num)
	if len(v2.num) > n {
		n = len(v2.num)
	}
	for i := 0; i < n; i++ {
		if s := sgnv(v1.getNum(i), v2.getNum(i)); s != 0 {
			return s
		}
	}

	// Version numbers match. Check pre-release, elementwise.
	// Build metadata is ignored.

	// A version with zero pres dominates any non-zero number.
	switch {
	case len(v1.pre) == 0 && len(v2.pre) == 0:
		return 0
	case len(v1.pre) == 0:
		return 1
	case len(v2.pre) == 0:
		return -1
	}

	return comparePrerelease(v1, v2)
}

// compareElem compares the strings s1 and s2 as elements of a prerelease tag.
// Numbers sort before non-numbers.
func compareElem(sys System, s1, s2 string) int {
	n1, ok1 := isNumeric(sys, s1)
	n2, ok2 := isNumeric(sys, s2)
	if ok1 && ok2 {
		return sgn64(n1, n2)
	}
	// Numbers are lower than alphas.
	if ok1 {
		return -1
	}
	if ok2 {
		return 1
	}
	if sys == NuGet {
		// Case insensitive.
		return compareNugetPrerelease(s1, s2)
	}

	return strings.Compare(s1, s2)
}

// NuGet does a modified case insensitive comparison of pre-release tags.
// Since this is alphanumeric (ascii), we try to avoid runing things.
func compareNugetPrerelease(s1, s2 string) int {
	l1 := len(s1)
	l2 := len(s2)
	for i := 0; i < l1; i++ {
		if i >= l2 {
			return 1
		}
		c1 := s1[i]
		c2 := s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32
		}

		d := sgn(int(c1), int(c2))
		if d != 0 {
			return d
		}
	}
	if l1 < l2 {
		return -1
	}
	return 0
}

// comparePrerelease compares the two versions's prerelease tags
func comparePrerelease(v1, v2 *Version) int {
	// Longer pres dominate shorter pres.
	for i, p1 := range v1.pre {
		if i >= len(v2.pre) {
			return 1
		}
		c := compareElem(v1.sys, p1, v2.pre[i])
		if c != 0 {
			return c
		}
	}
	if len(v1.pre) < len(v2.pre) {
		return -1
	}
	return 0
}

// equalPrerelease reports whether the two versions have the same prelease tags.
func equalPrerelease(v1, v2 *Version) bool {
	if v1 == v2 {
		return true
	}
	return comparePrerelease(v1, v2) == 0
}

// sgn64 returns the signum of a-b.
func sgn64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// sgnu64 returns the signum of (unsigned) a-b.
func sgnu64(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// sgn returns the signum of a-b.
func sgn(a, b int) int {
	return sgn64(int64(a), int64(b))
}

// sgnv returns the signum of a-b. It's a signed comparison so
// infinities work right: 1.* is less than 1.2.*.
func sgnv(a, b value) int {
	return sgn64(int64(a), int64(b))
}

// sgnStr returns the signum of the string "subtraction" a-b
func sgnStr(a, b string) int {
	if a > b {
		return 1
	}
	if a < b {
		return -1
	}
	return 0
}

func (v *Version) getNum(i int) value {
	if i < len(v.num) {
		return v.num[i]
	}
	return 0
}

func (v *Version) setNum(i int, val value) {
	if len(v.num) == 0 {
		v.num = v.buf[:0]
	}
	for len(v.num) <= i {
		v.num = append(v.num, 0)
	}
	v.num[i] = val
}

func (v *Version) incN(n int) {
	v.setNum(n, v.num[n].inc())
}

func (v *Version) major() value {
	return v.getNum(nMajor)
}

func (v *Version) setMajor(val value) {
	v.setNum(nMajor, val)
}

func (v *Version) minor() value {
	return v.getNum(nMinor)
}

func (v *Version) setMinor(val value) {
	v.setNum(nMinor, val)
}

func (v *Version) patch() value {
	return v.getNum(nPatch)
}

func (v *Version) setPatch(val value) {
	v.setNum(nPatch, val)
}

func (v *Version) clearPre() {
	v.pre = nil
	if v.ext != nil {
		v.ext.clearPre()
	}
}

// Semver requires 3 numbers, although in practice we often see fewer.
func (v *Version) atLeast3() int {
	n := len(v.num)
	if n < 3 {
		return 3
	}
	return n
}

func (v *Version) equal(u *Version) bool {
	return compare(v, u) == 0
}

func (v *Version) lessThan(u *Version) bool {
	return compare(v, u) < 0
}

func (v *Version) lessThanOrEqual(u *Version) bool {
	return compare(v, u) <= 0
}

func (v *Version) greaterThan(u *Version) bool {
	return compare(v, u) > 0
}

func (v *Version) all(val value) bool {
	for _, num := range v.num {
		if num != val {
			return false
		}
	}
	return true
}

func (v *Version) newExtension(str string) (extension, error) {
	switch v.sys {
	case Maven:
		return newMavenExtension(v, str)
	case PyPI:
		return newPEP440Extension(v, str)
	case RubyGems:
		return newGemExtension(v, str)
	}
	return nil, nil
}

func (v *Version) copy() *Version {
	n := *v
	// Make sure the num slice uses a different backing array, n.buf if possible.
	if len(n.num) <= len(n.buf) {
		n.num = n.buf[:len(n.num)]
	} else {
		n.num = append([]value(nil), v.num...)
	}
	if n.pre != nil {
		// Make sure the pre slice uses a different backing array.
		n.pre = append([]string(nil), v.pre...)
	}
	if n.ext != nil {
		n.ext = v.ext.copy(&n)
	}
	return &n
}

// Epoch extracts the epoch from the parsed semantic version. If the
// version did not specify an epoch, or the system doesn't support it,
// 0 and false are returned.
func (v *Version) Epoch() (int, bool) {
	if p, ok := v.ext.(*pep440Extension); ok && p.ext != nil {
		return p.ext.epoch, true
	}
	return 0, false
}

// Major extracts the major from the parsed semantic version. If the
// version did not specify a major version 0 and false are returned.
func (v *Version) Major() (int64, bool) {
	if len(v.num) == 0 {
		return 0, false
	}
	return int64(v.num[0]), true
}

// Semver 2.0 states that all prereleases sort before releases,
// and that numbers sort before strings. Thus 0.0.0-0 is
// the lowest version.
var minPre = []string{"0"}

// MinVersion returns a new Version for that system that precedes
// all other versions in comparison order.
func (sys System) MinVersion(v *Version) *Version {
	switch sys {
	default:
		// Semver-compliant systems are mostly the same.
		// We overwrite the argument to avoid allocation.
		for i := range v.buf {
			v.buf[i] = 0
		}
		v.num = v.buf[:3]
		// Although there is a prerelease, the user did not provide it so
		// we do not want it to trigger prerelease matching in span.contains.
		v.isPrerelease = false
		if sys == Go {
			v.str = "v0.0.0-0"
		} else {
			v.str = "0.0.0-0"
		}
		v.pre = minPre
		v.build = ""
		v.ext = nil
		return v
	case Maven:
		return mavenMinVersion.copy()
	case PyPI:
		return pypiMinVersion.copy()
	case RubyGems:
		return rubyGemsMinVersion.copy()
	}
}
