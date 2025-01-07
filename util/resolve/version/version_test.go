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

package version

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func newAttrSet(pairs ...any) AttrSet {
	var set AttrSet
	for len(pairs) > 0 {
		x, y := pairs[0], pairs[1]
		pairs = pairs[2:]
		set.SetAttr(x.(AttrKey), y.(string))
	}
	return set
}

func TestAttrSetString(t *testing.T) {
	tests := []struct {
		set  AttrSet
		want string
	}{
		{AttrSet{}, "{}"},
		{newAttrSet(AttrKey(-8), "", Blocked, ""), "{Blocked,AttrKey(-8)}"},
		{newAttrSet(AttrKey(12), "", AttrKey(23), "wowsa"), `{AttrKey(12),AttrKey(23)="wowsa"}`},
	}
	for _, test := range tests {
		if got := test.set.String(); got != test.want {
			t.Errorf("(%+v).String() = %q, want %q", test.set, got, test.want)
		}
	}
}

func TestAttrSetForEachAttr(t *testing.T) {
	tests := []map[AttrKey]string{
		{},
		{AttrKey(-4): "", Deleted: "", Blocked: ""},
		{AttrKey(6): "", AttrKey(23): "wowsa", Blocked: ""},
	}

	for _, test := range tests {
		var a AttrSet
		for k, v := range test {
			a.SetAttr(k, v)
		}
		got := make(map[AttrKey]string)
		a.ForEachAttr(func(key AttrKey, value string) {
			if _, ok := got[key]; ok {
				t.Errorf("(%+v).ForEachAttr: called twice on key %s", a, key)
			}
			got[key] = value
		})
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("(%+v).ForEachAttr:\n(- got, + want):\n%s", a, diff)
		}
	}
}

func TestAttrSetAccessors(t *testing.T) {
	tests := []map[AttrKey]string{
		{},
		{AttrKey(-4): "", Deleted: "ignored", Blocked: ""},
		{AttrKey(6): "", AttrKey(23): "wowsa", Blocked: ""},
	}

	for _, test := range tests {
		var a AttrSet
		for k, v := range test {
			for k, v := range test {
				a.SetAttr(k, v)
			}
			if ok := a.HasAttr(k); !ok {
				t.Errorf("(%+v).HasAttr: failed for key %s", a, k)
			}
			// Negative attributes do not get a value, just a presence test for 'ok'
			val, ok := a.GetAttr(k)
			if k < 0 {
				if val != "" {
					t.Errorf("(%+v).GetAttr: unexpected value for key %s: %q", a, k, val)
				}
			} else if !ok {
				t.Errorf("(%+v).GetAttr: failed for key %s", a, k)
			} else if val != v {
				t.Errorf("(%+v).GetAttr: key %s: got %q, want %q", a, k, val, v)
			}
		}
	}
}
