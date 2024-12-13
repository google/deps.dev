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
	"sort"
	"strings"
)

// A Set represents a complete constraint specification as a slice of spans
// defining ranges of valid concrete versions that would satisfy the constraint.
// The slice is stored in increasing span.min order.
// A Set is created by parsing a valid constraint specification.
type Set struct {
	sys  System
	span []span // Make this a field to hide the slice itself from callers.
}

// String returns a textual representation of the Set acceptable by
// ParseSetConstraint.
func (s Set) String() string {
	var b strings.Builder
	b.WriteByte('{')
	for i, span := range s.span {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprint(&b, span)
	}
	b.WriteByte('}')
	return b.String()
}

// parseSet parses the string and returns the set it represents.
// The text must have no extraneous characters.
// The boolean reports whether the set is created from a single version
// with no operators.
func (sys System) parseSet(s string) (Set, bool, error) {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return Set{}, false, fmt.Errorf("missing or misplaced braces: %#q", s)
	}
	if s == "{}" {
		// Special case: Convert this to an explicit empty set, which
		// is a set with one span of rank 'empty'.
		// Our code doesn't generate this form but a user might.
		return Set{
			sys:  sys,
			span: []span{{rank: empty}},
		}, false, nil
	}
	strs := strings.Split(s[1:len(s)-1], ",")
	spans := make([]span, 0, len(strs))
	weight := 0
	for _, str := range strs {
		span, simple, err := sys.parseSpan(str)
		if err != nil {
			return Set{}, false, err
		}
		spans = append(spans, span)
		weight++
		if !simple {
			weight++
		}
	}
	return Set{
		sys:  sys,
		span: spans,
	}, weight == 1, nil
}

// canon returns the canonicalization of the slice:
// - elements are sorted in increasing min order.
// - abutting elements are unified.
// The slice (its underlying array, that is) is overwritten in place.
func canon(s []span) ([]span, error) {
	// Common case; avoid the expense.
	if len(s) <= 1 {
		return s, nil
	}
	// If it's Maven, don't bother. This algorithm is relevant only to semver.
	// TODO: Should we do a Maven canonicalizer?
	if sysOfSpan(s) == Maven {
		return s, nil
	}
	sort.Slice(s, func(i, j int) bool {
		if c := s[i].min.Compare(s[j].min); c != 0 {
			return c < 0
		}
		if s[i].minOpen != s[j].minOpen {
			return !s[i].minOpen
		}
		if c := s[i].max.Compare(s[j].max); c != 0 {
			return c < 0
		}
		if s[i].maxOpen != s[j].maxOpen {
			return s[i].maxOpen
		}
		return false
	})
	allEmpty := true
	// Merge overlapping/adjoining elements.
	out := s[:0]
	for i := 0; i < len(s); i++ {
		this := s[i]
		if this.rank == empty {
			continue
		}
		allEmpty = false
		// Merge as many as possible into this element.
		for j := i + 1; j < len(s); j++ {
			next := s[j]
			if !this.max.equal(next.min) { // If equal, we can merge unless both are open (handled below)
				if len(this.max.pre) == 0 {
					maxPlusOne := this.max.copy()
					err := maxPlusOne.inc()
					if err != nil {
						return nil, err
					}
					if maxPlusOne.lessThan(next.min) {
						// There is a gap; cannot merge.
						break
					}
				} else {
					continue // Too difficult for now, but may be covered by another span. TODO?
				}
			}
			// Max equals min, but don't merge if both open.
			if this.maxOpen && next.minOpen {
				continue
			}
			// Merging prereleases and non-preleases is tricky, so avoid it.
			// Three tests to make.
			if !equalPrerelease(this.min, this.max) || !equalPrerelease(this.min, next.min) || !equalPrerelease(this.min, next.max) {
				continue
			}
			// We'll process the element now, so on the next outer loop, skip it.
			i++
			if next.rank == empty {
				continue
			}
			if next.max.lessThanOrEqual(this.max) {
				// Already covered; just close the end correctly.
				if this.max.equal(next.max) {
					this.maxOpen = this.maxOpen && next.maxOpen
				}
				continue
			}
			// Merge. We know that the maxes aren't equal, so it's a span.
			this.rank = vector
			this.max = next.max
			this.maxOpen = next.maxOpen
		}
		out = append(out, this)
	}
	if allEmpty {
		return s[:1], nil
	}
	return out, nil
}

// sysOfSpan returns the System of the versions in the spans. This requires unpacking
// the Versions.
func sysOfSpan(s []span) System {
	for _, span := range s {
		if span.min != nil {
			return span.min.sys
		}
	}
	return DefaultSystem
}

// Union replaces the receiver with the set union of the receiver and the argument.
func (s *Set) Union(t Set) error {
	var err error
	s.span, err = canon(append(s.span, t.span...))
	return err
}

// Intersect replaces the receiver with the set intersection of the receiver and the argument.
func (s *Set) Intersect(t Set) error {
	out := []span{} // TODO: Avoid allocation?
	for _, selem := range s.span {
		if selem.rank == empty { // Shouldn't happen.
			continue
		}
		// Intersect selem with each member of t. Since they're
		// canonical, so sorted in Min order, we can stop once there's
		// no chance of overlap.
		for _, telem := range t.span {
			if telem.rank == empty { // Shouldn't happen.
				continue
			}
			// TODO: Avoid the n^2 here.
			if telem.max.lessThan(selem.min) || (telem.max.equal(selem.min) && telem.maxOpen) {
				continue // Not there yet.
			}
			if telem.min.greaterThan(selem.max) {
				// No need to check further.
				break
			}
			// We know they overlap. Choose the larger min and the lesser max.
			min, max := selem.min, selem.max
			minOpen, maxOpen := selem.minOpen, selem.maxOpen
			if telem.min.greaterThan(selem.min) || (telem.min.equal(selem.min) && telem.minOpen) {
				min = telem.min
				minOpen = telem.minOpen
			}
			if telem.max.lessThan(selem.max) || (telem.max.equal(selem.max) && telem.maxOpen) {
				max = telem.max
				maxOpen = telem.maxOpen
			}
			span, err := newSpan(min, minOpen, max, maxOpen)
			if err != nil {
				return err
			}
			out = append(out, span)
		}
	}
	if len(out) == 0 {
		// An empty list means everything, so we need an explicitly empty span.
		out = []span{{rank: empty}}
	}
	var err error
	s.span, err = canon(out)
	return err
}

// Match reports whether the version defined by the argument string is contained
// in the set.
func (s Set) Match(version string) (bool, error) {
	v, err := s.sys.Parse(version)
	if err != nil {
		return false, err
	}
	return s.MatchVersion(v), nil
}

func (s Set) matchVersion(v *Version, includePrerelease bool) bool {
	// Empty constraint list is a special case: everything but pre-releases.
	if len(s.span) == 0 {
		return len(v.pre) == 0
	}

	// For RubyGems, prereleases are just another element.
	if v.sys == RubyGems {
		includePrerelease = true
	}

	for _, span := range s.span {
		pre := includePrerelease
		// PyPI has special checks.
		if v.sys == PyPI && span.rank == vector {
			// Prerelease and dev versions only match if the span is a prerelease or
			// dev (it doesn't seem to matter which is which).
			if !pre && (v.IsPrerelease() || v.isPyPIDev()) {
				anyPre := span.min.IsPrerelease() || span.max.IsPrerelease()
				anyDev := span.min.isPyPIDev() || span.max.isPyPIDev()
				if !(anyPre || anyDev) {
					continue
				}
				// Empirically we've also observed that
				// prerelease/dev matching is not enabled when
				// the lower bound is open, regardless of
				// whether the span is prerelease or dev.
				if span.minOpen {
					continue
				}
				pre = true
			}
			// TODO(pfcm): we will need a way to turn all of these
			// on to be able to match generously for security
			// advisories.

			// Postreleases are treated as normal versions, unless
			// the lower bound is open, not a postrelease and the
			// numbers exactly match.
			if v.isPyPIPost() && !span.min.isPyPIPost() && span.minOpen {
				if numsEqual(v, span.min) {
					continue
				}
			}
			// TODO: match locals properly
			if v.isPyPILocal() {
				continue
			}
		}
		if v.sys == NuGet {
			// In Nuget, ranges and floating constraints (*) don't
			// match prereleases unless one of the bounds is a
			// prerelease.
			if !pre && v.IsPrerelease() {
				if !span.min.IsPrerelease() && !span.max.IsPrerelease() {
					continue
				}
				pre = true
			}
		}
		if span.contains(v, pre) {
			return true
		}
	}
	return false
}

// numsEqual reports whether just the numeric components of the two versions are
// identical, with zero-padding if required.
func numsEqual(a, b *Version) bool {
	n := len(a.num)
	if nb := len(b.num); nb > n {
		n = nb
	}
	for i := 0; i < n; i++ {
		if s := sgnv(a.getNum(i), b.getNum(i)); s != 0 {
			return false
		}
	}
	return true
}

// MatchVersion reports whether the version is contained in the set.
func (s Set) MatchVersion(v *Version) bool {
	return s.matchVersion(v, false)
}

// Empty reports whether the set is empty, that is, can match no versions.
func (s Set) Empty() bool {
	for _, span := range s.span {
		if span.rank != empty {
			return false
		}
	}
	return true
}
