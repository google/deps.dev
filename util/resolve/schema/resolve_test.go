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

package schema

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
)

func TestSchema_GraphRoot(t *testing.T) {
	got, err := ParseResolve(`
name1 v1
`, resolve.NPM)
	if err != nil {
		t.Fatalf("cannot parse sample graph: %v", err)
	}
	want := &resolve.Graph{}
	want.AddNode(makeVK("name1", "v1"))
	if err := want.Canon(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected graph:\n(- got, + want):\n%s", diff)
	}
}

func TestSchema_CycleOnRoot(t *testing.T) {
	got, err := ParseResolve(`
label: name1 v1
	$label@*
`, resolve.NPM)
	if err != nil {
		t.Fatalf("cannot parse sample graph: %v", err)
	}
	want := &resolve.Graph{}
	want.AddNode(makeVK("name1", "v1"))
	if err := want.AddEdge(0, 0, "*", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.Canon(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected graph:\n(- got, + want):\n%s", diff)
	}
}

func TestSchema_GraphSimple(t *testing.T) {
	got, err := ParseResolve(`
name1 v1
	name2@r1 v2
		label: name3@r3 v3
	name4@r2 v4
		$label@r4
`, resolve.NPM)
	if err != nil {
		t.Fatalf("cannot parse sample graph: %v", err)
	}
	want := &resolve.Graph{}
	n1 := want.AddNode(makeVK("name1", "v1")) // Root
	n2 := want.AddNode(makeVK("name2", "v2"))
	n3 := want.AddNode(makeVK("name3", "v3"))
	n4 := want.AddNode(makeVK("name4", "v4"))
	if err := want.AddEdge(n1, n2, "r1", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n1, n4, "r2", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n2, n3, "r3", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n4, n3, "r4", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.Canon(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected graph:\n(- got, + want):\n%s", diff)
	}
}

func TestSchema_DepType(t *testing.T) {
	got, err := ParseResolve(`
name1 v1
	name2@r1 v2
		label: Framework .NETStandard1.0|name3@r3 v3
	Dev|name4@r2 v4
		XTest xtest|$label@r4
`, resolve.NPM)
	if err != nil {
		t.Fatalf("cannot parse sample graph: %v", err)
	}
	want := &resolve.Graph{}
	n1 := want.AddNode(makeVK("name1", "v1")) // Root
	n2 := want.AddNode(makeVK("name2", "v2"))
	n3 := want.AddNode(makeVK("name3", "v3"))
	n4 := want.AddNode(makeVK("name4", "v4"))
	if err := want.AddEdge(n1, n2, "r1", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	dt := dep.Type{}
	dt.AddAttr(dep.Dev, "")
	if err := want.AddEdge(n1, n4, "r2", dt); err != nil {
		t.Fatal(err)
	}
	dt = dep.Type{}
	dt.AddAttr(dep.Framework, ".NETStandard1.0")
	if err := want.AddEdge(n2, n3, "r3", dt); err != nil {
		t.Fatal(err)
	}
	dt = dep.Type{}
	dt.AddAttr(dep.XTest, "xtest")
	if err := want.AddEdge(n4, n3, "r4", dt); err != nil {
		t.Fatal(err)
	}
	if err := want.Canon(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected graph:\n(- got, + want):\n%s", diff)
	}
}

func TestSchema_ScopedNames(t *testing.T) {
	got, err := ParseResolve(`
@scope1/name1 v1
	@scope2/name2@r1 v2
		label: @scope3/name3@r3 v3
	KnownAs @alias/pkg|@scope4/name4@r2 v4
		$label@r4
		not/scope5@name5@r5 v5
`, resolve.NPM)
	if err != nil {
		t.Fatalf("cannot parse sample graph: %v", err)
	}
	want := &resolve.Graph{}
	n1 := want.AddNode(makeVK("@scope1/name1", "v1")) // Root
	n2 := want.AddNode(makeVK("@scope2/name2", "v2"))
	n3 := want.AddNode(makeVK("@scope3/name3", "v3"))
	n4 := want.AddNode(makeVK("@scope4/name4", "v4"))
	n5 := want.AddNode(makeVK("not/scope5", "v5"))
	if err := want.AddEdge(n1, n2, "r1", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	dt := dep.Type{}
	dt.AddAttr(dep.KnownAs, "@alias/pkg")
	if err := want.AddEdge(n1, n4, "r2", dt); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n2, n3, "r3", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n4, n3, "r4", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.AddEdge(n4, n5, "name5@r5", dep.Type{}); err != nil {
		t.Fatal(err)
	}
	if err := want.Canon(); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected graph:\n(- got, + want):\n%s", diff)
	}
}

func TestGraphString(t *testing.T) {
	cases := []struct {
		title     string
		schema    string
		want      string
		errNodeID resolve.NodeID
		errReq    resolve.VersionKey
		err       error
	}{
		{
			title:  "empty",
			schema: "",
			want:   "",
		},
		{
			title:  "single",
			schema: "alice 0",
			want: `alice 0
`,
		},
		{
			title: "simple",
			schema: `
alice 1
	bob@r1 2
		chuck@r2 3
		$dave@r3
	dave: dave@r4 4
`,
			want: `alice 1
├─ bob@r1 2
│  ├─ chuck@r2 3
│  └─ $1@r3
└─ 1: dave@r4 4
`,
		},
		{
			title: "cycle root",
			schema: `
alice: alice 1
	bob@r1 2
		chuck@r2 3
			$alice@r5
		$dave@r3
	dave: dave@r4 4
		$alice@r6
`,
			want: `1: alice 1
├─ bob@r1 2
│  ├─ chuck@r2 3
│  │  └─ $1@r5
│  └─ $2@r3
└─ 2: dave@r4 4
   └─ $1@r6
`,
		},
		{
			title: "direct cycle",
			schema: `
alice: alice 1
	$alice@
`,
			want: `1: alice 1
└─ $1@
`,
		},
		{
			title: "direct cycle with 2 deps",
			schema: `
alice: alice 1
	bob@1 1
	$alice@2
`,
			want: `1: alice 1
├─ $1@2
└─ bob@1 1
`,
		},
		{
			title: "inner direct cycle with 2 deps",
			schema: `
alice 1
	bob: bob@1 1
		$bob@
`,
			want: `alice 1
└─ 1: bob@1 1
   └─ $1@
`,
		},
		{
			title: "two direct incoming edges",
			schema: `
alice 1
	bob: bob@r1 2
		chuck@r2 3
	test|$bob@r3
`,
			want: `alice 1
├─ 1: bob@r1 2
│  └─ chuck@r2 3
└─ test | $1@r3
`,
		},
		{
			title: "cycle towards itself",
			schema: `
alice 1
	chuck_after_bob_after_canon@r1 2
		bob: bob@r2 3
			$bob@r3
`,
			want: `alice 1
└─ chuck_after_bob_after_canon@r1 2
   └─ 1: bob@r2 3
      └─ $1@r3
`,
		},
		{
			title: "simple with one error",
			schema: `
alice 1
	bob@r1 2
		chuck@r2 3
			franck@r5 ERROR: not found
		$dave@r3
	dave: dave@r4 4
`,
			want: `alice 1
├─ bob@r1 2
│  ├─ chuck@r2 3
│  │  └─ franck@r5 ERROR: not found
│  └─ $1@r3
└─ 1: dave@r4 4
`,
			errNodeID: 2,
			errReq: resolve.VersionKey{
				PackageKey: resolve.PackageKey{Name: "franck"},
				Version:    "r5",
			},
			err: errors.New("not found"),
		},
		{
			title: "simple with scopes",
			schema: `
@a/alice 1
	@b/bob@r1 2
		@c/chuck@r2 3
		$dave@r3
	dave: @d/dave@r4 4
`,
			want: `@a/alice 1
├─ @b/bob@r1 2
│  ├─ @c/chuck@r2 3
│  └─ $1@r3
└─ 1: @d/dave@r4 4
`,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			g, err := ParseResolve(c.schema, resolve.NPM)
			if err != nil {
				t.Fatal(err)
			}
			s := g.String()
			if s != c.want {
				t.Fatalf("wrong representation, got:\n%s\nwant:\n%s\n", s, c.want)
			}

			gg, err := ParseResolve(s, resolve.NPM)
			if err != nil {
				t.Fatalf("ParseResolve(%q): %v", s, err)
			}
			if diff := cmp.Diff(g, gg); diff != "" {
				t.Errorf("g != ParseResolve(g.String()):\n(- g, + ParseResolve):\n%s", diff)
			}
		})
	}
}

func makeVK(name string, v string) resolve.VersionKey {
	return resolve.VersionKey{
		PackageKey: resolve.PackageKey{
			System: resolve.NPM,
			Name:   name,
		},
		VersionType: resolve.Concrete,
		Version:     v,
	}
}
