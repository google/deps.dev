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

// rank describes the rank of the range: empty, a point (unit), or a full span (vector).
type rank uint8

const (
	empty rank = iota
	unit
	vector
)

const (
	closed = false // Whether this end of the interval is open/closed.
	open   = true
)

// A span represents a range of possible versions that could match a constraint.
// The zero value of a span is empty.
type span struct {
	rank    rank
	minOpen bool // Whether min is included; closed (included) is the default.
	maxOpen bool // Whether max is included; closed (included) is the default.
	min     *Version
	// If span is a unit, max==min. Invariant established during construction.
	max *Version
}

// String returns a textual representation of the span.
func (s span) String() string {
	// Use Canon, not String, when printing versions as we want to see the internal version,
	// not the string the user provided.
	switch s.rank {
	case empty:
		return "<empty>"
	case unit:
		return s.min.Canon(false)
	case vector:
		left := '['
		if s.minOpen {
			left = '('
		}
		right := ']'
		if s.maxOpen {
			right = ')'
		}
		return fmt.Sprintf("%c%s:%s%c", left, s.min.Canon(false), s.max.Canon(false), right)
	}
	return fmt.Sprintf("internal error: unrecognized rank %d in %s,%s", s.rank, s.min, s.max)
}

// parseSpan parses the string and returns the span it represents.
// The text must have no extraneous characters, including space.
// The returned boolean reports whether the span represents a
// simple version, not a wildcard, with no operators.
func (sys System) parseSpan(s string) (span, bool, error) {
	switch {
	case s == "":
		// Error returned at bottom of function.
	case s == "<empty>":
		return span{rank: empty}, false, nil
	case s[0] == '[', s[0] == '(':
		minOpen := s[0] == '('
		close := s[len(s)-1]
		if close != ']' && close != ')' {
			break
		}
		versions := strings.Split(s[1:len(s)-1], ":")
		if len(versions) != 2 {
			break
		}
		maxOpen := close == ')'
		min, err := sys.parse(versions[0], false)
		if err != nil {
			return span{}, false, err
		}
		max, err := sys.parse(versions[1], true)
		if err != nil {
			return span{}, false, err
		}
		return span{
			minOpen: minOpen,
			maxOpen: maxOpen,
			rank:    vector,
			min:     min,
			max:     max,
		}, false, nil
	default:
		v, err := sys.Parse(s)
		if err != nil {
			return span{}, false, err
		}
		return span{
			minOpen: false,
			maxOpen: false,
			rank:    unit,
			min:     v,
			max:     v,
		}, !v.IsWildcard(), nil
	}
	return span{}, false, fmt.Errorf("syntax error parsing span %#q", s)
}

// newSpan returns the span defined by the min and max versions.
// The Version structs should not be modified after calling newSpan.
func newSpan(min *Version, minOpen bool, max *Version, maxOpen bool) (span, error) {
	// Comparison won't work correctly with wildcards (they're not really
	// versions, they're ranges). Convert them into real values before proceeding.
	if min.major() == wildcard {
		// A plain "*" means everything.
		min = min.sys.MinVersion(min)
	} else {
		min.setTail(wildcard, 0)
	}
	max.setTail(wildcard, infinity)
	min.build = ""
	max.build = ""
	switch {
	case min.equal(max):
		return span{
			minOpen: minOpen,
			maxOpen: maxOpen,
			rank:    unit,
			min:     min,
			max:     min,
		}, nil
	case min.lessThan(max):
		return span{
			minOpen: minOpen,
			maxOpen: maxOpen,
			rank:    vector,
			min:     min,
			max:     max,
		}, nil
	default:
		return span{}, fmt.Errorf("newSpan: max less than min: %q < %q", max.Canon(true), min.Canon(true))
	}
}

// setTail changes v so that the last few numbers, starting with the marker, are
// replaced by the fill value. It is a no-op if the marker does not appear.
func (v *Version) setTail(marker, fill value) {
	n := v.atLeast3()
	i := 0
	for ; i < n && v.getNum(i) != marker; i++ {
	}
	for ; i < n; i++ {
		v.setNum(i, fill)
	}
}

// fill sets all unset numbers in the version to val, up to 3 max.
func (v *Version) fill(val value) {
	for i := len(v.num); i < 3; i++ {
		v.setNum(i, val)
	}
}

// contains reports whether v is within the range defined by the span.
// The boolean specifies whether to include prerelease versions when
// the constraint itself does not have them.
func (s span) contains(v *Version, includePrerelease bool) bool {
	switch s.rank {
	case empty:
		return false
	case unit:
		return compare(s.min, v) == 0
	}

	// Vector is more work. Need to see if it's between s.min and s.max.
	min := s.min
	max := s.max

	if c := v.Compare(min); c == 0 && s.minOpen || c < 0 {
		return false
	}
	if c := max.Compare(v); c == 0 && s.maxOpen || c < 0 {
		return false
	}

	if includePrerelease {
		// Numbers match and that's enough.
		return true
	}

	// Unless it's Maven, for which the compare method handles
	// everything, we need to check the prerelease tags.
	if v.sys != Maven && v.isPrerelease {
		// Numbers must match either min or max and pre must be in range.
		if min.isPrerelease && equalValues(v.num, min.num) && s.min.lessThanOrEqual(v) {
			return true
		}
		if max.isPrerelease && equalValues(v.num, max.num) { // Already know v <= max.
			return true
		}
		return false // There may be more at this min to check, such as 1.2.0-p1 || 1.2.0-p2.
	}
	return true
}

func equalValues(a, b []value) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if x != b[i] {
			return false
		}
	}
	return true
}
