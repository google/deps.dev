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

package npm

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/internal/resolvetest"
	"deps.dev/util/resolve/version"
)

func TestResolver(t *testing.T) {
	a, err := resolvetest.ParseFiles(resolve.NPM,
		"testdata/resolve_test.data", "testdata/resolve_test.want",
		"testdata/derivedfrom_test.data", "testdata/derivedfrom_test.want",
		"testdata/deleted_test.data", "testdata/deleted_test.want",
		"testdata/alias.data", "testdata/alias.want",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range a.Test {
		t.Run(tst.Name, func(t *testing.T) {
			if tst.Universe == nil {
				t.Fatalf("no universe for %s", tst.Name)
			}
			if tst.Graph == nil {
				t.Fatalf("no graph for %s", tst.Name)
			}

			looseErrors := tst.Flags["loose_errors"]
			cleanGraph(t, nil, tst.Graph, looseErrors, false)

			r := NewResolver(tst.Universe)
			g, err := r.Resolve(context.Background(), tst.VK)
			if err != nil {
				t.Fatalf("cannot resolve %s: %v", tst.VK, err)
			}
			demangle := tst.Flags["demangle_names"]
			cleanGraph(t, tst.Universe, g, looseErrors, demangle)

			if diff := cmp.Diff(tst.Graph, g); diff != "" {
				t.Fatalf("Unexpected resolution (- want, + got):\n%s", diff)
			}
		})
	}
}

// cleanGraph cleans up the given graph so that it is suitable for comparison.
// If flagErrors is set and the given graph contains errors, returns a simple
// empty "HAS ERROR" graph.
// If flagDemangle is set, replaces all the mangled names by their original
// names. This is useful to compare sos vs. native, as native resolvers don't
// use mangled names.
func cleanGraph(t *testing.T, c resolve.Client, g *resolve.Graph, flagErrors, flagDemangle bool) {
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

	if flagDemangle {
		// Take the derived package version's original name.
		for i, n := range g.Nodes {
			v, err := c.Version(context.Background(), n.Version)
			if err != nil {
				t.Fatalf("VersionAttrs(%s): %v", n.Version, err)
			}
			if name, ok := v.GetAttr(version.DerivedFrom); ok {
				g.Nodes[i].Version.Name = name
			}
		}
	}

	if err := g.Canon(); err != nil {
		fmt.Println(g.String())
		t.Fatalf("Canon: %v", err)
	}

	g.Duration = 0
}
