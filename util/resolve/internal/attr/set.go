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
Package attr provides data structures for representing sets of keyed attributes.

This package is an implementation detail of the dep and version packages.
*/
package attr

import (
	"math/bits"
	"strings"
)

// Set is a collection of attributes, represented as a Mask and a map of
// arbitrary uint8 keys to string values.
//
// The zero value of Set is an empty, default set.
type Set struct {
	// Mask is a bitmask that may be manipulated directly.
	// See the Mask type doc for details.
	Mask Mask

	attrs map[uint8]string

	// attrBits indicates which values are in attrs.
	// This is used to make Type comparisons and encoding fast;
	// it will need changing when we support keys >= 64.
	attrBits uint64
}

// A Mask is a bitmask of reserved, widely used, no-valued attributes whose
// presence in a Set is the indicator. All eight bits may be set; their
// semantics are defined by the user of this package.
type Mask uint8

// SetAttr adds an attribute to the Set, replacing any existing one of the same key.
// Keys >= 64 aren't supported yet, and will cause a panic.
func (s *Set) SetAttr(key uint8, value string) {
	if key >= 64 {
		panic("key too large")
	}
	if s.attrs == nil {
		s.attrs = make(map[uint8]string)
	}
	s.attrs[key] = value
	s.attrBits |= 1 << uint(key)
}

// GetAttr gets an attribute from the Set.
func (s Set) GetAttr(key uint8) (value string, ok bool) {
	value, ok = s.attrs[key]
	return
}

// Clone returns a clone of the given Set.
func (s Set) Clone() Set {
	c := Set{
		Mask:     s.Mask,
		attrs:    make(map[uint8]string, len(s.attrs)),
		attrBits: s.attrBits,
	}
	for k, v := range s.attrs {
		c.attrs[k] = v
	}
	return c
}

// IsRegular reports whether the Set is equivalent to its zero value.
func (s Set) IsRegular() bool {
	return s.Mask == 0 && len(s.attrs) == 0
}

// Compare returns -1, 0 or 1 depending on whether the Set is ordered
// before, equal to or after the other Set.
func (s Set) Compare(other Set) int {
	if s.Mask < other.Mask {
		return -1
	} else if s.Mask > other.Mask {
		return 1
	}

	if s.attrBits < other.attrBits {
		return -1
	} else if s.attrBits > other.attrBits {
		return 1
	}

	// Compare attributes themselves in encoding order.
	for remBits := s.attrBits; remBits != 0; {
		// Find lowest set bit.
		key := uint8(bits.TrailingZeros64(remBits))
		remBits &^= 1 << uint(key)

		if cmp := strings.Compare(s.attrs[key], other.attrs[key]); cmp != 0 {
			return cmp
		}
	}

	return 0
}

// ForEachAttr calls f for each attribute in ascending key order.
func (s Set) ForEachAttr(f func(key uint8, value string)) {
	for remBits := s.attrBits; remBits != 0; {
		// Find lowest set bit.
		key := uint8(bits.TrailingZeros64(remBits))
		remBits &^= 1 << uint(key)

		value := s.attrs[key]

		f(key, value)
	}
}
