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

package resolvetest

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/version"
)

func TestParseData(t *testing.T) {
	ctx := context.Background()
	// Parse input.
	a, err := ParseFiles(resolve.NPM, "testdata/resolvetest_test.data")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Some comparisons.
	if got, want := len(a.Universe), 4; got != want {
		t.Fatalf("got %d universes, want %d", got, want)
	}
	for _, name := range []string{"alice", "bob", "alice2", "alice3"} {
		if a.Universe[name] == nil {
			t.Fatalf("Missing %s universe", name)
		}
	}

	if got, want := len(a.Graph), 2; got != want {
		t.Fatalf("got %d resolved graphs, want %d", got, want)
	}
	for _, name := range []string{"alice", "alice2"} {
		if a.Graph[name] == nil {
			t.Fatalf("Missing %s resolved graph", name)
		}
		if err := a.Graph[name].Canon(); err != nil {
			t.Fatalf("Canon %s resolved graph", name)
		}
	}
	g1, g2 := a.Graph["alice"], a.Graph["alice2"]
	if d := cmp.Diff(g1, g2); d != "" {
		t.Logf("Mismatching parsed graphs:(- alice, + alice1):\n%s", d)
	}

	if got, want := len(a.Test), 2; got != want {
		t.Fatalf("got %d tests, want %d", got, want)
	}
	var (
		aliceVK = resolve.VersionKey{
			PackageKey: resolve.PackageKey{
				System: resolve.NPM,
				Name:   "alice",
			},
			VersionType: resolve.Concrete,
			Version:     "1.0.0",
		}
		bobVK = resolve.VersionKey{
			PackageKey: resolve.PackageKey{
				System: resolve.NPM,
				Name:   "bob",
			},
			VersionType: resolve.Concrete,
			Version:     "1.0.0",
		}
	)
	for _, vk := range []resolve.VersionKey{aliceVK, bobVK} {
		if _, err := a.Test[0].Universe.Requirements(ctx, vk); err != nil {
			t.Fatalf("Missing %s in test universe: %v", vk, err)
		}
	}

	if got, want := a.Test[0].VK, aliceVK; got != want {
		t.Fatalf("Unexpected test resolve,\n got %s\nwant %s", got, want)
	}

	if a.Test[0].Graph != a.Graph["alice"] {
		t.Fatal("Unexpected test graph")
	}
	if got, want := a.Test[0].GraphName, "alice"; got != want {
		t.Fatalf("Unexpected graph name: got %q, want %q", got, want)
	}

	got, want := a.Test[0].Flags, map[string]bool{"flag1": true, "flag2": true}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Unexpected test flags:\n(- got, + want):\n%s", diff)
	}

	// Multiverse with overlap.
	u := a.Test[1].Universe
	avs, err := u.Versions(ctx, aliceVK.PackageKey)
	if err != nil {
		t.Fatalf("Versions(%v): %v", aliceVK.PackageKey, err)
	}
	found := false
	for _, av := range avs {
		if av.VersionKey == aliceVK {
			found = true
			got, _ := av.GetAttr(version.Registries)
			want := ",a3,dep:bob"
			if got != want {
				t.Fatalf("Unexpected attributes, got %q, want %q", got, want)
			}
		}
	}
	if !found {
		t.Fatalf("Missing version %v", aliceVK)
	}
}
