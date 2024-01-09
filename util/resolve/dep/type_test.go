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

package dep

import (
	"fmt"
	"testing"
)

func TestAccessors(t *testing.T) {
	reg := Type{}
	dev := NewType(Dev)
	dev.AddAttr(-1, "a") // Short attr
	dev.AddAttr(1, "b")  // Big attr

	tests := []struct {
		ty            Type
		key           AttrKey
		wantIsRegular bool
		wantHas       bool
		wantGet       string
	}{
		{reg, -1, true /* wantIsRegular */, false, ""},
		{reg, 0, true /* wantIsRegular */, false, ""},
		{reg, 1, true /* wantIsRegular */, false, ""},
		{dev, -1, false /* wantIsRegular */, true, ""},
		{dev, 0, false /* wantIsRegular */, false, ""},
		{dev, 1, false /* wantIsRegular */, true, "b"},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s/%s", test.ty, test.key), func(t *testing.T) {
			if got, want := test.ty.IsRegular(), test.wantIsRegular; got != want {
				t.Errorf("'isRegular' got %v, want %v", got, want)
			}
			if got, want := test.ty.HasAttr(test.key), test.wantHas; got != want {
				t.Errorf("'has' got %v, want %v", got, want)
			}
			v, ok := test.ty.GetAttr(test.key)
			if ok != test.wantHas {
				t.Errorf("'get' got %v, want %v", ok, test.wantHas)
			} else if v != test.wantGet {
				t.Errorf("'get' got %v, want %q", ok, test.wantGet)
			}
		})
	}
}
