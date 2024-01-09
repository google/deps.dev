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
	"strconv"
	"strings"
)

// PyPI-specific support, actually PEP440. https://www.python.org/dev/peps/pep-0440/

// pep440Extension implements the PEP440-specific parts of a Version.
// Most versions in the wild are semver-compliant. In such cases,
// Version will be set but the ext field will be nil.
type pep440Extension struct {
	version *Version
	ext     *pep440 // The details.
}

// pep440 holds the details, if there are any.
// Within the struct, all components are canonicalized
// (lower-cased, "c"->"rc", etc.).
type pep440 struct {
	// The versioning epoch, usually absent, defaults to zero. We've only
	// seen 1 in the wild.
	epoch int // 0 is the default, no epoch.
	// Release segments are stored in the num slice of the Version.
	// Pre-release syntax is special, so we hold it here: "a32" etc.
	pre    string // "a". Empty if not present.
	preNum int    // 32
	// Postrelease is unique to PEP440. The string is always "post".
	postPresent bool
	postNum     int
	// Dev-release is unique to PEP440. The string is always "dev".
	devPresent bool
	devNum     int
	// Local is unique to PEP440. It is separated by +.
	local string
}

// newPep440Extension builds the extension object for an extant PEP440 Version.
func newPEP440Extension(v *Version, str string) (*pep440Extension, error) {
	// We may be re-building v.ext, if being called from opVersionToSpan.
	// We modify v, so initialize the fields we touch.
	v.num = v.num[:0]
	v.userNumCount = 0
	if v.pre != nil {
		v.pre = v.pre[:0]
	}
	e := &pep440Extension{
		version: v,
	}
	return e, e.init(str)
}

// isDev reports whether the version has a .dev element.
func (p *pep440Extension) isDev() bool {
	return p != nil && p.ext != nil && p.ext.devPresent
}

// pep440Version parses a PEP440 Version. The str field is set by the caller;
// most of the rest is stored in the extension, which is constructed here and attached
// to the returned Version. However, we do set v.num, v.pre, and v.userNumCount.
// v.pre is stored in both, in effect.
func (p *versionParser) pep440Version() (*Version, error) {
	var err error
	p.Version.ext, err = p.Version.newExtension(p.Version.str)
	return p.Version, err
}

func (p *pep440Extension) copy(v *Version) extension {
	n := new(pep440Extension)
	n.version = v
	if p.ext != nil {
		n.ext = new(pep440)
		*n.ext = *p.ext
	}
	return n
}

func (p *pep440Extension) clearPre() {
	if p.ext != nil {
		p.ext.pre = ""
		p.ext.preNum = 0
	}
}

func (p *pep440Extension) empty() bool {
	return p == nil || p.ext == nil || *p.ext == pep440{}
}

// canon returns a canonicalized string representation of the version/extension.
// The showBuild argument is ignored; PEP440 doesn't have that concept.
func (p *pep440Extension) canon(showBuild bool) string {
	var b strings.Builder
	v := p.version
	if p.ext == nil {
		// Just the numbers.
		v.printNums(&b)
		return b.String()
	}
	// We have an extension, so there is more work.
	if p.ext.epoch != 0 {
		fmt.Fprintf(&b, "%d!", p.ext.epoch)
	}
	v.printNums(&b)
	if p.ext.pre != "" {
		fmt.Fprintf(&b, "%s%d", p.ext.pre, p.ext.preNum)
	}
	if p.ext.postPresent {
		fmt.Fprintf(&b, ".post%d", p.ext.postNum)
	}
	if p.ext.devPresent {
		fmt.Fprintf(&b, ".dev%d", p.ext.devNum)
	}
	if p.ext.local != "" {
		fmt.Fprintf(&b, "+%s", p.ext.local)
	}
	return b.String()
}

var pypiMinVersion *Version

func init() {
	pypiMinVersion = &Version{
		sys:          PyPI,
		userNumCount: 3,
		isPrerelease: false,
		str:          "0.0.0dev0",
	}
	pypiMinVersion.num = pypiMinVersion.buf[:3]
	pypiMinVersion.ext = &pep440Extension{
		version: pypiMinVersion,
		ext: &pep440{
			epoch:      0,
			devPresent: true,
			devNum:     0,
		},
	}
}

// init parses a PEP440 version string into a slice of elements and stores them in the extension.
func (p *pep440Extension) init(input string) error {
	input = strings.TrimSpace(input) // There are words that surrounding spaces are legal.
	// Quick test for ASCII, not strictly required but provides messages
	// consistent with other systems.
	// TODO: If we used the lexer, this test wouldn't be needed.
	for _, r := range input {
		if r <= ' ' || r >= 0x7F {
			if r != '∞' { // Used in tests.
				return fmt.Errorf("invalid character %q in `%s`", r, input)
			}
		}
	}
	// Is there an epoch?
	bang := strings.IndexByte(input, '!')
	if bang > 0 {
		p.makeExt()
		e, err := strconv.ParseUint(input[:bang], 10, 8)
		if err != nil {
			return err
		}
		p.ext.epoch = int(e)
		input = input[bang+1:]
	}
	// There might be one v.
	if len(input) > 0 && (input[0] == 'v' || input[0] == 'V') {
		input = input[1:]
	}
	// Release segments.
	var i int
	for i = 0; i < len(input); {
		start := i
		var cat, wid int
		for {
			cat, wid = versionNext(input, i)
			if cat != versionNumeric {
				if cat == versionStar && start == i {
					i += wid // Accept but stop here.
				}
				break
			}
			i += wid
		}
		if i == start {
			break
		}
		switch input[start:i] {
		case "∞":
			p.version.addNum(infinity) // TODO: This should be enabled by a boolean.
		case "*":
			if len(p.version.num) == 0 {
				return fmt.Errorf("illegal wildcard as first component in `%s`", input)
			}
			p.version.addNum(wildcard)
		default:
			num, err := parseNum(input[start:i])
			if err != nil {
				return err
			}
			p.version.addNum(value(num))
		}
		if i == len(input) || input[i] != '.' {
			break
		}
		if len(input) == i+1 { // Trailing period.
			return fmt.Errorf("empty component in `%s`", input)
		}
		i++
	}
	if len(p.version.num) == 0 {
		return fmt.Errorf("no numbers in version `%s`", p.version)
	}
	// Pad to at least 3, but record what the user provided.
	p.version.userNumCount = int16(len(p.version.num))
	for len(p.version.num) < 3 && p.version.num[len(p.version.num)-1] != wildcard {
		p.version.addNum(0)
	}
	input = input[i:]
	var err error
	input, err = p.parsePre(input)
	if err != nil {
		return err
	}
	input, err = p.parsePost(input)
	if err != nil {
		return err
	}
	input, err = p.parseDev(input)
	if err != nil {
		return err
	}
	input, err = p.parseLocal(input)
	if err != nil {
		return err
	}
	if input != "" {
		// Common error.
		if input[0] == '.' {
			return fmt.Errorf("empty component in `%s`", p.version.str)
		}
		return fmt.Errorf("invalid text in version string in `%s`", p.version.str)
	}
	return err
}

// pep440PreStrings is an ordered list of the legal names for prereleases.
// The longer string with a shared prefix must come first.
var pep440PreStrings = []struct {
	text, canon string
}{
	{"alpha", "a"},
	{"a", "a"},
	{"beta", "b"},
	{"b", "b"},
	{"preview", "rc"},
	{"pre", "rc"},
	{"rc", "rc"},
	{"c", "rc"},
}

// pep440PostStrings is a list of the legal names for postreleases.
// The longer string with a shared prefix must come first.
var pep440PostStrings = []string{
	"post",
	"rev",
	"r",
}

const lettersInPyPI = "abcdehiloprstvw" // Used by possibleVersionString.

// parsePre parses a prerelease, if present, and adds it to the extension,
// returning the rest of the input.
func (p *pep440Extension) parsePre(originalInput string) (string, error) {
	if originalInput == "" {
		return originalInput, nil
	}
	// Keep original input in case we don't have a prerelease tag.
	input := allowSeparator(originalInput)
	found := false
	for _, s := range pep440PreStrings {
		if hasASCIIPrefix(input, s.text) {
			p.makeExt()
			p.ext.pre = s.canon
			input = input[len(s.text):]
			found = true
			break
		}
	}
	if !found {
		return originalInput, nil
	}
	p.ext.preNum, input = p.number(input)
	// Put the info in the top-level version, mostly for opVersionTo Span.
	p.version.pre = make([]string, 2)
	p.version.pre[0] = p.ext.pre
	p.version.pre[1] = fmt.Sprint(p.ext.preNum) // TODO just take the string.
	p.version.isPrerelease = true
	return input, nil
}

// parsePost parses a postrelease, if present, and adds it to the extension,
// returning the rest of the input.
func (p *pep440Extension) parsePost(originalInput string) (string, error) {
	if originalInput == "" {
		return originalInput, nil
	}
	separatorIsDash := originalInput[0] == '-'
	input := allowSeparator(originalInput)
	length := 0
	for _, pat := range pep440PostStrings {
		if hasASCIIPrefix(input, pat) {
			length = len(pat)
			break
		}
	}
	// "post" can be missing iff the separator is a dash and a number is provided.
	if length == 0 {
		if len(input) == 0 || !separatorIsDash || !isDigit(rune(input[0])) {
			return originalInput, nil
		}
	}
	p.makeExt()
	p.ext.postPresent = true
	p.ext.postNum, input = p.number(input[length:])
	return input, nil
}

// parseDev parses a dev marker, if present, and adds it to the extension,
// returning the rest of the input.
func (p *pep440Extension) parseDev(originalInput string) (string, error) {
	if originalInput == "" {
		return originalInput, nil
	}
	input := allowSeparator(originalInput)
	const dev = "dev"
	if !hasASCIIPrefix(input, dev) {
		return originalInput, nil
	}
	p.makeExt()
	p.ext.devPresent = true
	p.ext.devNum, input = p.number(input[len(dev):])
	return input, nil
}

// hasASCIIPrefix reports whether str beings with the pattern,
// ignoring case. The pattern must be lower case and both
// strings must be ASCII.
func hasASCIIPrefix(str, pat string) bool {
	if len(str) < len(pat) {
		return false
	}
	for i := 0; i < len(pat); i++ {
		if str[i]|0x20 != pat[i] {
			return false
		}
	}
	return true
}

// parseLocal parses a local marker, if present, and adds it to the extension,
// returning the rest of the input.
func (p *pep440Extension) parseLocal(input string) (string, error) {
	if len(input) < 2 || input[0] != '+' { // Starts with +, cannot be empty after.
		return input, nil
	}
	for i := 1; i < len(input); i++ {
		c := input[i]
		if c != '.' && c != '-' && c != '_' && !isAlphanumeric(rune(c)) {
			return input, fmt.Errorf("invalid local version identifier in `%s`", p.version.str)
		}
	}
	// Must start and end with alphanumeric.
	if !isAlphanumeric(rune(input[1])) || !isAlphanumeric(rune(input[len(input)-1])) {
		return input, fmt.Errorf("invalid local version identifier in `%s`", p.version.str)
	}
	p.makeExt()
	str := input[1:]
	// In local only, - and _ are permitted but are not canonical.
	if strings.Contains(str, "-") {
		str = strings.ReplaceAll(str, "-", ".")
	}
	if strings.Contains(str, "_") {
		str = strings.ReplaceAll(str, "_", ".")
	}
	p.ext.local = str
	return "", nil
}

func allowSeparator(input string) string {
	// We are allowed a dot, underscore or minus.
	if len(input) > 0 {
		if c := input[0]; c == '.' || c == '-' || c == '_' {
			input = input[1:]
		}
	}
	return input
}

func (p *pep440Extension) number(input string) (int, string) {
	input = allowSeparator(input)
	cat, wid := versionNext(input, 0)
	if cat != versionNumeric {
		return 0, input
	}
	var i int
	for i = wid; i < len(input); i += wid {
		cat, wid = versionNext(input, i)
		if cat != versionNumeric {
			break
		}
	}
	num, _ := strconv.ParseUint(input[:i], 10, 64)
	return int(num), input[i:]
}

// makeExt allocates a PEP400 struct if the existing one is nil.
func (p *pep440Extension) makeExt() {
	if p.ext == nil {
		p.ext = new(pep440)
	}
}

var zeroPEP440 pep440

// The order in which various attachments compare.
// For instance, 1.0.post > 1.0 > 1.0b > 1.0a.
const (
	pep440Dev int = iota
	pep440Alpha
	pep440Beta
	pep440Prerelease
	pep440Empty
	pep440Local
	pep440Post
)

// rank returns the basic ordering according what attachments are
// present in the extension.
func (p *pep440) rank() int {
	// p is never nil; called only from compare.
	switch {
	case p.pre == "a":
		return pep440Alpha
	case p.pre == "b":
		return pep440Beta
	case p.pre == "rc":
		return pep440Prerelease
	case p.postPresent:
		return pep440Post
	case p.devPresent: // Check this here as it can appear with a pre or post.
		return pep440Dev
	case p.local != "":
		return pep440Local
	}
	return pep440Empty
}

// isPyPIPost reports whether the version is a PEP440 post-release.
func (v *Version) isPyPIPost() bool {
	if v.sys != PyPI || v.ext == nil {
		return false
	}
	ext := v.ext.(*pep440Extension)
	return ext.ext != nil && ext.ext.postPresent
}

// isPyPILocal reports whether the version is a PEP440 local release.
func (v *Version) isPyPILocal() bool {
	if v.sys != PyPI || v.ext == nil {
		return false
	}
	ext := v.ext.(*pep440Extension)
	return ext.ext != nil && ext.ext.local != ""
}

// isPyPIDev reports whether the version is a PEP440 dev release.
func (v *Version) isPyPIDev() bool {
	if v.sys != PyPI || v.ext == nil {
		return false
	}
	ext := v.ext.(*pep440Extension)
	return ext.isDev()
}

// compare uses PEP440's rules to decide the ordering of p and e.
func (p *pep440Extension) compare(e extension) int {
	q := e.(*pep440Extension)
	pExt := p.ext
	if pExt == nil {
		pExt = &zeroPEP440
	}
	qExt := q.ext
	if qExt == nil {
		qExt = &zeroPEP440
	}

	// Epochs win.
	if pExt.epoch != qExt.epoch {
		return sgn(pExt.epoch, qExt.epoch)
	}

	// Release numbers.
	pv := p.version
	qv := q.version
	n := len(pv.num)
	if len(qv.num) > n {
		n = len(qv.num)
	}
	for i := 0; i < n; i++ {
		s := sgnv(pv.getNum(i), qv.getNum(i))
		if s != 0 {
			return s
		}
	}

	if p.ext == nil && q.ext == nil {
		// Nothing else to compare.
		return 0
	}

	// We have the same numbers. We now compare attachments. Their order is:
	//	devN aN bN rcN <empty> postN
	// and within each item, ordered by N. Also, a dev can appear along with
	// any other. If one version has a higher rank than the other, that determines
	// their ordering.
	pRank := pExt.rank()
	qRank := qExt.rank()
	if pRank != qRank {
		return sgn(pRank, qRank)
	}

	// Same rank, so now we must look at the contents of the extension.
	switch pRank {
	case pep440Alpha, pep440Beta, pep440Prerelease:
		if s := sgn(pExt.preNum, qExt.preNum); s != 0 {
			return s
		}
		fallthrough
	case pep440Local:
		if s := pep44CompareLocal(pExt.local, qExt.local); s != 0 {
			return s
		}
		fallthrough
	case pep440Post:
		if s := sgn(pExt.postNum, qExt.postNum); s != 0 {
			return s
		}
	}

	// Dev can attach to anything (although we've never seen one on a post).
	if pExt.devPresent || qExt.devPresent {
		if pExt.devPresent != qExt.devPresent {
			if pExt.devPresent {
				return -1 // Dev is before pre, empty, or post.
			}
			return 1
		}
		if s := sgn(pExt.devNum, qExt.devNum); s != 0 {
			return s
		}
	}

	return 0
}

// pep440CompareLocal compares the local strings elementwise.
// Some of this could be done up front, but they are very rare.
func pep44CompareLocal(pl, ql string) int {
	if pl == ql {
		return 0
	}
	// Numbers dominate strings, and are evaluated numerically.
	// Strings are ASCII-only and compared case-insensitively.
	pn := strings.Count(pl, ".") + 1
	qn := strings.Count(ql, ".") + 1
	n := pn
	if n > qn {
		n = qn
	}
	var pElem, qElem string
	for i := 0; i < pn; i++ {
		pElem, pl = pep440LocalElem(pl)
		qElem, ql = pep440LocalElem(ql)
		if s := p440compareLocalElem(pElem, qElem); s != 0 {
			return s
		}
	}
	return sgn(pn, qn)
}

// pep440LocalElem pulls out the next element from a local string.
func pep440LocalElem(s string) (string, string) {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return s, ""
	}
	return s[:dot], s[dot+1:]
}

// allDigits reports whether the argument is a non-empty string of digits only.
func allDigits(s string) bool {
	for _, c := range s {
		if !isDigit(c) {
			return false
		}
	}
	return s != ""
}

// p440compareLocalElem performs the comparison for a single pair of
// local elements. Numbers dominate strings.
func p440compareLocalElem(a, b string) int {
	aDigits := allDigits(a)
	bDigits := allDigits(b)
	if aDigits != bDigits {
		if aDigits {
			return 1
		}
		return -1
	}
	if aDigits {
		an, _ := strconv.ParseUint(a, 10, 64)
		bn, _ := strconv.ParseUint(b, 10, 64)
		return sgnu64(an, bn)
	}
	return sgnStr(a, b)
}
