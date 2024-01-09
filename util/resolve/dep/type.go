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
Package dep provides data structures for representing dependency types.
*/
package dep

import (
	"fmt"
	"strings"

	"deps.dev/util/resolve/internal/attr"
)

// Type indicates the type of a dependency edge.
//
// The zero value of Type is a regular dependency. Attributes may be added to
// a Type to annotate it with extra details or restrictions.
type Type struct {
	set attr.Set
}

// NewType constructs a Type with the given attributes set.
// This is a convenience constructor for Types with value-less attributes.
func NewType(attrs ...AttrKey) Type {
	var t Type
	for _, a := range attrs {
		t.AddAttr(a, "")
	}
	return t
}

// Clone returns a clone of the given Type.
func (t *Type) Clone() Type {
	return Type{set: t.set.Clone()}
}

// AddAttr adds an attribute to the Type.
func (t *Type) AddAttr(key AttrKey, value string) {
	// Handle special cases first.
	if key < 0 {
		t.set.Mask |= attr.Mask(-key)
		return
	}
	t.set.SetAttr(uint8(key), value)
}

// GetAttr gets an attribute from the Type.
func (t *Type) GetAttr(key AttrKey) (value string, ok bool) {
	// Handle special cases first.
	if key < 0 {
		return "", t.set.Mask&attr.Mask(-key) != 0
	}
	return t.set.GetAttr(uint8(key))
}

// HasAttr reports whether the type has the given attribute.
// This is a convenience method when the key is used as a flag.
func (t *Type) HasAttr(key AttrKey) bool {
	_, ok := t.GetAttr(key)
	return ok
}

// IsRegular reports whether the Type is a regular, unattributed Type.
func (t Type) IsRegular() bool { return t.set.IsRegular() }

// Equal reports whether the Type is identical to other.
func (t Type) Equal(other Type) bool { return t.Compare(other) == 0 }

// Compare returns -1, 0 or 1 depending on whether the Type is ordered
// before, equal to or after the other Type.
func (t Type) Compare(other Type) int { return t.set.Compare(other.set) }

func (t Type) String() string {
	s := "reg"
	if t.set.Mask != 0 {
		var ss []string
		if t.set.Mask&attr.Mask(-Dev) != 0 {
			ss = append(ss, "dev")
		}
		if t.set.Mask&attr.Mask(-Opt) != 0 {
			ss = append(ss, "opt")
		}
		if t.set.Mask&attr.Mask(-Test) != 0 {
			ss = append(ss, "test")
		}
		s = strings.Join(ss, "|")
	}
	t.set.ForEachAttr(func(key uint8, value string) {
		k := AttrKey(key)
		s += fmt.Sprintf("|%s=%q", k, value)
	})
	return s
}
