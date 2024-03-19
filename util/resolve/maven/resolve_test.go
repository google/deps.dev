// Copyright 2024 Google LLC
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

package maven

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/internal/resolvetest"
)

func TestMavenResolver(t *testing.T) {
	a, err := resolvetest.ParseFiles(resolve.Maven,
		"testdata/resolve_test.data", "testdata/resolve_test.want",
		"testdata/exclusions_test.data", "testdata/exclusions_test.want",
		"testdata/multiverse_test.data", "testdata/multiverse_test.want",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range a.Test {
		t.Run(tst.Name, func(t *testing.T) {
			r := NewResolver(tst.Universe)
			g, err := r.Resolve(context.Background(), tst.VK)
			if err != nil {
				t.Fatalf("cannot resolve %s: %v", tst.VK, err)
			}

			looseErrors := tst.Flags["loose_errors"]
			cleanGraph(t, g, looseErrors)
			cleanGraph(t, tst.Graph, looseErrors)
			if diff := cmp.Diff(tst.Graph, g); diff != "" {
				t.Fatalf("Unexpected resolution (- want, + got):\n%s", diff)
			}
		})
	}
}

// cleanGraph cleans up the given graph so that it is suitable for comparison.
// If flagErrors is set and the given graph contains errors, returns a simple
// empty "HAS ERROR" graph.
func cleanGraph(t *testing.T, g *resolve.Graph, flagErrors bool) {
	t.Helper()

	if flagErrors {
		hasError := g.Error != ""
		for _, n := range g.Nodes {
			if n.Errors != nil {
				hasError = true
				break
			}
		}
		if hasError {
			*g = resolve.Graph{
				Error: "HAS ERROR",
			}
			return
		}
	}

	if err := g.Canon(); err != nil {
		t.Fatalf("Canon: %v", err)
	}

	// Only keep the creating edge and remove the extra.
	edges := make([]resolve.Edge, 0, len(g.Edges))
	for _, e := range g.Edges {
		if !e.Type.HasAttr(dep.Selector) {
			continue
		}
		edges = append(edges, e)
	}
	copy(g.Edges, edges)
	g.Edges = g.Edges[:len(edges)]

	// Maven worker does not report requirements nor types.
	for i := range g.Edges {
		g.Edges[i].Requirement = ""
		g.Edges[i].Type = dep.Type{}
	}

	g.Duration = 0
}
