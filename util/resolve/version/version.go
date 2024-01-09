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
Package version provides data structures for representing version attributes.
*/
package version

import (
	"math/bits"
	"strconv"
	"strings"

	"deps.dev/util/resolve/internal/attr"
)

// AttrSet represents a set of version attributes.
// The zero value of AttrSet is an empty set.
type AttrSet struct {
	set attr.Set
}

// SetAttr adds an attribute to the Set, replacing any existing one of the same key.
func (s *AttrSet) SetAttr(key AttrKey, value string) {
	// Handle special cases first.
	if key < 0 {
		s.set.Mask |= attr.Mask(-key)
		return
	}
	s.set.SetAttr(uint8(key), value)
}

// GetAttr gets an attribute.
func (s AttrSet) GetAttr(key AttrKey) (value string, ok bool) {
	// Handle special cases first.
	if key < 0 {
		return "", s.set.Mask&attr.Mask(-key) != 0
	}
	return s.set.GetAttr(uint8(key))
}

// HasAttr reports whether the set has the given attribute.
// This is a convenience method when the key is used as a flag.
func (s AttrSet) HasAttr(key AttrKey) bool {
	_, ok := s.GetAttr(key)
	return ok
}

// ForEachAttr calls f for each attribute.
func (s AttrSet) ForEachAttr(f func(key AttrKey, value string)) {
	for remBits := uint64(s.set.Mask); remBits != 0; {
		// Find lowest set bit.
		k := uint8(bits.TrailingZeros64(remBits))
		key := uint64(1) << k
		remBits &^= key
		f(AttrKey(-key), "")
	}
	s.set.ForEachAttr(func(key uint8, value string) {
		f(AttrKey(key), value)
	})
}

// Empty reports whether the AttrSet is equivalent to its zero value.
func (s AttrSet) Empty() bool { return s.set.IsRegular() }

func (s AttrSet) Equal(other AttrSet) bool { return s.set.Compare(other.set) == 0 }

func (s AttrSet) String() string {
	if s.Empty() {
		return "{}"
	}

	var sb strings.Builder
	any := false
	sb.WriteByte('{')
	for m, bit := s.set.Mask, 0; m != 0 && bit < maskLen; bit++ {
		if m&(1<<bit) == 0 {
			continue
		}
		key := AttrKey(-(1 << bit))
		if any {
			sb.WriteByte(',')
		}
		any = true
		sb.WriteString(key.String())
	}
	s.set.ForEachAttr(func(k uint8, value string) {
		if any {
			sb.WriteByte(',')
		}
		any = true
		sb.WriteString(AttrKey(k).String())
		if value != "" {
			sb.WriteByte('=')
			sb.WriteString(strconv.Quote(value))
		}
	})
	sb.WriteByte('}')
	return sb.String()
}

// Clone returns a copy of s.
func (s AttrSet) Clone() AttrSet {
	return AttrSet{
		set: s.set.Clone(),
	}
}
