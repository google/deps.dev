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

// This file implements the core of a simple interval arithmetic used to define
// sets representing ranges of version values.

import (
	"fmt"
)

// A value represents a numeric element of a Version. It needs to be a
// 64-bit number because we see some large values in the wild, especially
// involving dates: "1.2.20181231235959"
// A full semver has three values: a major, minor, and a patch number.
type value int64

const (
	// infinity represents a number larger than any sensible version would define.
	infinity value = 1<<63 - 1
	// wildcard elements are represented by a negative and impossible value.
	// For compare to work seamlessly so 1.*-1.2.* is not impossible,
	// wildcard must also be lower than any valid version.
	wildcard value = -1
)

func (v value) String() string {
	if v == infinity {
		return "∞"
	}
	if v == wildcard {
		return "*"
	}
	return fmt.Sprint(int(v))
}

// inc returns the value increased by 1, capping at infinity.
func (v value) inc() value {
	v++
	if v > infinity {
		v = infinity
	}
	return v
}

// inc steps the receiver forward to the first possible version greater than
// the argument. The version following "1" is "2", and following "1.1" is "1.2".
// If a wildcard is present, it increments the value before the wildcard and
// zeroes the rest: inc(1.*.*) -> 2.0.0.
// There must be no prerelease string.
func (v *Version) inc() error {
	if len(v.pre) > 0 {
		return fmt.Errorf("internal error: inc with pre-release string: %v", v)
	}
	// >1, >1.0, and >1.0.0 do _not_ mean the same thing.
	switch len(v.num) {
	case 0:
		return fmt.Errorf("internal error: no numbers in version %v", v)
	case 1:
		if v.major() == wildcard || v.major() == infinity {
			v.setMajor(infinity)
			v.setMinor(infinity)
			v.setPatch(infinity)
			return nil
		}
		v.incN(nMajor)
	case 2:
		if v.minor() == wildcard || v.minor() == infinity {
			v.incN(nMajor)
			v.setMinor(0)
			v.setPatch(0)
			return nil
		}
		v.incN(nMinor)
	default:
		wildcardIndex := -1
		for i, val := range v.num {
			if val == wildcard || val == infinity {
				wildcardIndex = i
				break
			}
		}
		switch wildcardIndex {
		case -1:
			// No wildcards.
			v.incN(len(v.num) - 1)
		case 0:
			// First field is wild; nothing to do.
		default:
			v.incN(wildcardIndex - 1)
			for i := wildcardIndex; i < len(v.num); i++ {
				v.setNum(i, 0)
			}
		}
	}
	return nil
}

// opVersionToSpan takes a possibly empty operator and a version and returns
// the span represented by applying the operator to the version.
func opVersionToSpan(typ tokType, op string, lo *Version) (span, error) {
	// If the version has a wildcard, any prerelease info is irrelevant, so
	// drop it. NuGet wildcard constraints exclude pre-releases unless
	// explicitly specified.
	if lo.IsWildcard() && lo.sys != NuGet {
		lo.clearPre()
	}
	// A prerelease with <3 numbers is meaningless, except Cargo accepts
	// them in a constraint specification. Since opVersionToSpan means
	// we are in a constraint, we can accept it.
	if lo.sys != Cargo && len(lo.num) < 3 && len(lo.pre) > 0 {
		return span{}, fmt.Errorf("prerelease requires 3 numbers: %#q", lo.str)
	}
	// Avoid calling copy unless needed.
	if (typ == tokEmpty || typ == tokEqual) && len(lo.num) >= 3 && lo.allNumbers() {
		// A case like "1.2.3-alpha".
		return newSpan(lo, closed, lo, closed)
	}
	hi := lo.copy()
	minOpen, maxOpen := closed, closed
	// TODO: PyPI has the special operator ===, which we do not handle.
	switch typ {
	case tokEmpty, tokEqual:
		switch len(lo.num) {
		case 1:
			hi.setMinor(infinity)
			hi.setPatch(infinity)
		case 2:
			hi.setPatch(infinity)
		default:
		}
		return newSpan(lo, closed, hi, closed)

	case tokGreater:
		if lo.all(wildcard) {
			return span{rank: empty}, nil
		}
		if len(lo.pre) > 0 || lo.sys == RubyGems || lo.sys == PyPI {
			// >1.2 matches 1.2.3 in RubyGems but not in NPM (for example).
			minOpen = open
		} else {
			err := lo.inc()
			if err != nil {
				return span{}, err
			}
		}
		lo.build = ""
		fallthrough

	case tokGreaterEqual:
		wildcard := false
		// Remove wildcard asterisk in NuGet floating pre-release constraints.
		if lo.sys == NuGet && len(lo.pre) > 0 {
			last := len(lo.pre) - 1
			p := lo.pre[last]
			if p[len(p)-1] == '*' {
				p = p[:len(p)-1]
				wildcard = true
			}
			if p == "" {
				p = "0"
			}
			lo.pre[last] = p
		}
		for i := range hi.num {
			hi.num[i] = infinity
		}
		hi.clearPre()
		// lo.IsWildcard doesn't report prerelease wildcards.
		if lo.sys == NuGet && (lo.IsWildcard() || wildcard) {
			return newSpan(lo, closed, hi, open)
		}
		hi.build = ""

	case tokLess:
		// Special horrible cases.
		if lo.all(wildcard) || lo.all(0) {
			return span{rank: empty}, nil
		}
		for i, val := range hi.num {
			if val == wildcard {
				hi.setNum(i, 0)
			}
		}
		maxOpen = open
		hi.build = ""
		lo = lo.sys.MinVersion(lo)

	case tokLessEqual:
		switch len(lo.num) {
		case 1:
			hi.setMinor(infinity)
			hi.setPatch(infinity)
		case 2:
			hi.setPatch(infinity)
		case 3:
		}
		lo = lo.sys.MinVersion(lo)

	case tokCaret:
		// There is no ^ in Ruby so we don't worry about >3 numbers.
		if len(lo.num) == 2 && lo.major() == 0 && lo.minor() == 0 {
			// Special case: ^0.0 means <0.1.0.
			hi.setPatch(infinity)
			hi.clearPre()
			return newSpan(lo, closed, hi, closed)
		}
		if lo.major() == 0 && len(lo.num) >= 2 {
			// Leave Minor alone, but if it's zero, things get trickier.
			if lo.minor() != 0 {
				hi.setPatch(infinity) // ^0.0.1 matches only itself.
			}
			hi.clearPre()
			return newSpan(lo, closed, hi, closed)
		}
		// ^* -> everything.
		if lo.major() == wildcard {
			hi.setMajor(infinity)
			hi.setMinor(infinity)
			hi.setPatch(infinity)
			hi.clearPre()
			lo = lo.sys.MinVersion(lo)
			return newSpan(lo, closed, hi, closed)
		}
		// ^1.2.3 -> [1.2.3,1.∞.∞].
		// value
		hi.setMinor(infinity)
		hi.setPatch(infinity)
		hi.clearPre()
		return newSpan(lo, closed, hi, closed)

	case tokTilde:
		// There is no ~ in Ruby so we don't worry out >3 numbers.
		// Tilde is special with Major == 0
		if lo.major() == 0 && len(lo.num) >= 2 {
			hi.setPatch(infinity)
		} else {
			switch len(lo.num) {
			case 1:
				hi.setMinor(infinity)
				hi.setPatch(infinity)
			case 2:
				hi.setPatch(infinity)
			case 3:
				hi.setPatch(infinity)
			}
		}

	case tokBacon:
		n := len(lo.num)
		if lo.sys == RubyGems || lo.sys == PyPI {
			// RubyGems and PyPI fill trailing zeros, but this operator needs to
			// know the number of digits actually provided.
			n = int(lo.userNumCount)
		}
		switch n {
		case 0:
			return span{}, fmt.Errorf("internal error: no numbers in %s.opVersionToSpan ~>%s; %#v", lo.sys.String(), lo, lo)
		case 1:
			if lo.sys == PyPI {
				return span{}, fmt.Errorf("need two numbers for %q", op)
			}
			hi.setMinor(infinity)
			hi.setPatch(infinity)
		case 2:
			// Treat as ^1.2 so we have the same form for both for canonicalization.
			if lo.major() != infinity {
				hi.setMinor(infinity)
				hi.setPatch(infinity)
				return newSpan(lo, closed, hi, closed)
			}
			hi.setMinor(infinity)
			hi.setPatch(infinity)
		case 3:
			hi.setPatch(infinity)
		default:
			hi.setNum(len(hi.num)-1, infinity)
		}

	default:
		return span{}, fmt.Errorf("unrecognized operator %q", op)
	}
	lo.setTail(infinity, infinity)
	hi.setTail(infinity, infinity)
	if lo.sys == Maven || lo.sys == RubyGems || lo.sys == PyPI {
		// Must rebuild the extensions.
		// TODO: Can this be more efficient?
		if err := lo.rebuildExtension(); err != nil {
			return span{}, err
		}
		if err := hi.rebuildExtension(); err != nil {
			return span{}, err
		}
	}
	return newSpan(lo, minOpen, hi, maxOpen)
}

func (v *Version) rebuildExtension() error {
	if v.ext == nil || v.ext.empty() {
		return nil
	}
	var err error
	v.ext, err = v.newExtension(v.Canon(true))
	return err
}

func excludeToSpans(v *Version) (span, span, error) {
	if len(v.num) == 0 {
		return span{}, span{}, fmt.Errorf("no numbers: %s", v)
	}
	// Any wildcards must be at the end.
	for _, val := range v.num[:len(v.num)-1] {
		if val == wildcard || val == infinity {
			return span{}, span{}, fmt.Errorf("!=%s not implemented", v)
		}
	}
	var (
		lo, hi = v, v
	)
	switch val := v.num[len(v.num)-1]; val {
	case infinity:
		return span{}, span{}, fmt.Errorf("!=%s not implemented", v)
	case wildcard:
		// Build the span as if it was equality and then invert it.
		opp, err := opVersionToSpan(tokEmpty, "", v)
		if err != nil {
			return span{}, span{}, err
		}
		lo = opp.min
		hi = opp.max
	default:
	}
	zero := &Version{
		sys: v.sys,
		str: "0.0.0",
	}
	zero.num = zero.buf[:3]
	inf := &Version{
		sys: v.sys,
		str: "∞.∞.∞",
	}
	inf.num = inf.buf[:3]
	inf.setMajor(infinity)
	inf.setMinor(infinity)
	inf.setPatch(infinity)
	s1, err := newSpan(zero, closed, lo, open)
	if err != nil {
		return span{}, span{}, err
	}
	s2, err := newSpan(hi, open, inf, closed)
	return s1, s2, err
}
