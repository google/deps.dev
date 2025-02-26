// Copyright 2025 Google LLC
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

package pypi

import (
	"context"
	"reflect"
	"testing"
	"time"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/internal/resolvetest"
)

// TODO: pip's test suite is missing cases with multiple valid answers. Add some.

func testResolution(ctx context.Context, t *testing.T, r resolve.Resolver, tst *resolvetest.Test) {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	if tst.Name == "with-without-extras" {
		// One test takes longer than a second.
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	}
	defer cancel()

	g, err := r.Resolve(ctx, tst.VK)
	if err != nil {
		t.Fatalf("Resolve(%v): %v", tst.VK, err)
	}
	if err := g.Canon(); err != nil {
		t.Fatalf("Canonicalizing: %v", err)
	}
	g.Duration = 0
	tst.Graph.Duration = 0

	// TODO: check the errors actually match somehow.
	if tst.Flags["error"] {
		if g.Error == "" {
			t.Errorf("Expected graph-level error %q, got:\n%v", tst.Graph.Error, g)
		}
		return
	}

	if !reflect.DeepEqual(g, tst.Graph) {
		t.Errorf("Graph mismatch:\ngot:\n%v\nwant:\n%v\n", g, tst.Graph)
	}
}

func TestResolve(t *testing.T) {
	a, err := resolvetest.ParseFiles(resolve.PyPI,
		"testdata/pip-tests.data",
		"testdata/additional-tests.data",
		"testdata/prerelease.data",
		"testdata/prerelease.want",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range a.Test {
		t.Run(tst.Name, func(t *testing.T) {
			testResolution(context.Background(), t, NewResolver(tst.Universe), tst)
		})
	}
}

func TestResolveLibResolve(t *testing.T) {
	a, err := resolvetest.ParseFiles(resolve.PyPI,
		"testdata/resolvelib/pypi-2020-02-13-chalice.data",
		"testdata/resolvelib/pypi-2020-11-17-cheroot.data",
		"testdata/resolvelib/pypi-2020-02-15-pandas.data",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range a.Test {
		t.Run(tst.Name, func(t *testing.T) {
			testResolution(context.Background(), t, NewResolver(tst.Universe), tst)
		})
	}
}

func TestResolveLoops(t *testing.T) {
	a, err := resolvetest.ParseFiles(resolve.PyPI, "testdata/loops.data", "testdata/loops.want")
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range a.Test {
		t.Run(tst.Name, func(t *testing.T) {
			testResolution(context.Background(), t, NewResolver(tst.Universe), tst)
		})
	}
}

func BenchmarkResolve(b *testing.B) {
	a, err := resolvetest.ParseFiles(resolve.PyPI,
		"testdata/resolvelib/pypi-2020-02-13-chalice.data",
		"testdata/resolvelib/pypi-2020-11-17-cheroot.data",
		"testdata/resolvelib/pypi-2020-02-15-pandas.data",
		"testdata/synthetic-backtracking.data",
		"testdata/prerelease.data",
	)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for _, tst := range a.Test {
		b.Run(tst.Name, func(b *testing.B) {
			r := NewResolver(tst.Universe)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				g, err := r.Resolve(ctx, tst.VK)
				if err != nil {
					b.Fatalf("Resolve(%v): %v", tst.VK, err)
				}
				_ = g
			}
		})
	}
}

func TestFilterSlice(t *testing.T) {
	count := func(in []int) map[int]int {
		out := make(map[int]int)
		for _, i := range in {
			out[i]++
		}
		return out
	}

	for _, c := range []struct {
		in  []int
		p   func(int) (bool, error)
		out []int
	}{{
		in: []int{0, 1, 2, 3, 4},
		p: func(i int) (bool, error) {
			return i == 0, nil
		},
		out: []int{0},
	}, {
		in: []int{0, 1, 2, 3, 4},
		p: func(i int) (bool, error) {
			return false, nil
		},
		out: []int{},
	}, {
		in: nil,
		p: func(i int) (bool, error) {
			return true, nil
		},
		out: nil,
	}, {
		in: []int{0, 1, 0},
		p: func(i int) (bool, error) {
			return i == 0, nil
		},
		out: []int{0, 0},
	}} {
		original := count(c.in)

		got, err := filterSlice(c.in, c.p)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.out, got) {
			t.Errorf("Result mismatch:\nwant: %v\n got: %v", c.out, got)
		}

		// Check the original slice still has the same elements,
		// although we don't mind if they're in a different order.
		gotCounts := count(c.in)
		if !reflect.DeepEqual(original, gotCounts) {
			t.Errorf("Original slice elements modified:\nwant: %v\n got: %v", original, gotCounts)
		}
	}
	// Some tests that filter the same slice repeatedly
	in := []int{1, 2, 3, 4, 5, 6, 7, 8}
	inCounts := count(in)
	for _, c := range []struct {
		p   func(int) (bool, error)
		out []int
	}{{
		p: func(i int) (bool, error) {
			return i == 8, nil
		},
		// after this in will be: [8 3 4 5 6 7 2 1]
		out: []int{8},
	}, {
		p: func(i int) (bool, error) {
			return i == 1, nil
		},
		// after this in will be: [1 4 5 6 7 2 3 8]
		out: []int{1},
	}, {
		p: func(i int) (bool, error) {
			return i%3 == 0, nil
		},
		// after this in will be: [3 6 5 7 2 4 8 1]
		out: []int{3, 6},
	}, {
		p: func(i int) (bool, error) {
			return i%2 == 0, nil
		},
		// after this in will be: [8 6 4 2 7 5 1 3]
		out: []int{8, 6, 4, 2},
	}} {
		got, err := filterSlice(in, c.p)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.out, got) {
			t.Errorf("Result mismatch:\nwant: %v\n got: %v", c.out, got)
		}

		// Everything should still be in the original slice.
		gotCounts := count(in)
		if !reflect.DeepEqual(inCounts, gotCounts) {
			t.Errorf("Original slice elements modified:\nwant: %v\n got: %v", inCounts, gotCounts)
		}
	}
}
