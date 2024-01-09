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

package attr

import (
	"testing"
)

func newSet(mask Mask) Set { return Set{Mask: mask} }

func newAttrSet(mask Mask, key uint8, v string) Set {
	set := newSet(mask)
	set.SetAttr(key, v)
	return set
}

func TestGet(t *testing.T) {
	set := Set{}

	if ok := set.IsRegular(); !ok {
		t.Errorf("got false, wanted true")
	}

	if got, ok := set.GetAttr(1); ok {
		t.Errorf("got %q %v, want false", got, ok)
	}

	want := "banana"
	set.SetAttr(1, want)
	if got, ok := set.GetAttr(1); !ok || got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	set2 := set.Clone()
	if got, ok := set2.GetAttr(1); !ok || got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, ok := set2.GetAttr(2); ok {
		t.Errorf("got %q %v, want false", got, ok)
	}
}

func TestCompare(t *testing.T) {
	// Sort order is Mask, AttrBits, Values
	// Has some duplicates, so that comparison is monotonic but not strictly increasing.
	ordered := []Set{
		newSet(0),
		newSet(1),
		newAttrSet(1, 0, "a"),
		newAttrSet(1, 0, "b"),
		newAttrSet(1, 0, "b"),
		newAttrSet(1, 1, "a"),
		newSet(2),
		newSet(2),
		newAttrSet(2, 0, "a"),
		newAttrSet(2, 1, "a"),
	}

	for i := 1; i < len(ordered); i++ {
		a := ordered[i-1]
		b := ordered[i]
		if comp := a.Compare(b); comp > 0 {
			t.Errorf("got %q not le than %q", a, b)
		}
		if comp := b.Compare(a); comp < 0 {
			t.Errorf("got %q not ge than %q", a, b)
		}
		// Try equality for all elements (may duplicate).
		c := a.Clone()
		d := b.Clone()
		if comp := a.Compare(c); comp != 0 {
			t.Errorf("got %q not equal to %q", a, b)
		}
		if comp := c.Compare(a); comp != 0 {
			t.Errorf("got %q not equal to %q", a, b)
		}
		if comp := b.Compare(d); comp != 0 {
			t.Errorf("got %q not equal to %q", a, b)
		}
		if comp := d.Compare(b); comp != 0 {
			t.Errorf("got %q not equal to %q", a, b)
		}
	}
}
