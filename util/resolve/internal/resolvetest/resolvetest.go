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
Package resolvetest provides a way to define test data for resolvers.

Test data follows a simple format that describes universes (entire package
ecosystems), resolved graphs, and test cases.

	Below is the definition of two universes, one named sample, the other
	named other, specified as a graph/schema definition.

	-- Universe sample
	alice
		1.0.0
			bob@1
	bob
		1.0.0
		2.0.0
	-- END

	-- Universe other
	eve
		1.0.0
			bob@1
	-- END

	Below is the definition of a test. It links a universe, a resolve root, a
	graph for the expected resolution, and optionally sets flags.

	-- Test alice
	Resolve alice 1.0.0
	Universe sample
	Graph alice
	Flag flag1 flag2
	-- END

	Below is the definition of a resolve.Graph, named alice, specified as a
	graph/schema definition.
	-- Graph alice
	alice 1.0.0
	└─ bob@1 1.0.0
	-- END

	Below is the definition of a test. It links two universes, a resolve root, a
	graph for the expected resolution, and optionally sets flags. The first
	universe, other, has no ID and is the default universe. The second has an
	ID that is typically used in version attributes to refer to a registry.

	-- Test eve
	Resolve eve 1.0.0
	Universe other, id_sample:sample
	Graph eve
	Flag flag1 flag2
	-- END


	Below is the definition of a resolve.Graph, named alice2, specified as a
	prototext message.
	alice 1.0.0
	└─ bob@1 1.0.0
	-- Graph prototext alice2
	node: {
		version: {
			system: NPM
			package_name: "alice"
			version_type: CONCRETE
			version: "1.0.0"
		}
	}
	node: {
		version: {
			system: NPM
			package_name: "bob"
			version_type: CONCRETE
			version: "1.0.0"
		}
	}
	edge: {
		to: 1
		requirement: "1"
	}
	-- END
*/
package resolvetest

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/internal/schema"
	"deps.dev/util/resolve/version"
)

const (
	startBlockUniverse = "-- universe "
	startBlockGraph    = "-- graph "
	startBlockTest     = "-- test "
	endBlock           = "-- end"
	optionPrototext    = "prototext"
	prefixTestUniverse = "universe "
	prefixTestResolve  = "resolve "
	prefixTestGraph    = "graph "
	prefixTestFlag     = "flag "
)

// Artifact describes the parsed content from a test data file.
type Artifact struct {
	// Universe holds the defined universes, indexed by name.
	Universe map[string]*resolve.LocalClient
	// Graph holds the defined resolved graphs, indexed by name.
	Graph map[string]*resolve.Graph
	// Test holds the defined tests in the order in which they were defined.
	Test []*Test
}

// Test describes a parsed test.
type Test struct {
	// Name of the name of the test.
	Name string
	// VK holds the concrete version from the universe to resolve.
	VK resolve.VersionKey
	// Universe holds the universe to use for resolution.
	Universe *resolve.LocalClient
	// Graph holds the resolved graph.
	Graph *resolve.Graph
	// GraphName holds the resolved graph name.
	GraphName string
	// Flags holds the defined flags
	Flags map[string]bool
}

// parsedTest describes a test during the parsing phase of the data.
// It contains identifiers instead of objects that may be parsed latter.
type parsedTest struct {
	name     string
	resolve  resolve.VersionKey
	universe string
	graph    string
	flags    map[string]bool
}

// ParseFiles parses the data from the given files and creates for the given
// system test artifacts: universes, resolved graphs, and tests.
func ParseFiles(sys resolve.System, files ...string) (*Artifact, error) {
	var b bytes.Buffer
	for _, file := range files {
		p, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		b.Write(p)
		b.WriteRune('\n')
	}

	return Parse(&b, sys)
}

// Parse parses the data from the given reader and creates for the given
// system test artifacts: universes, resolved graphs, and tests.
func Parse(r io.Reader, sys resolve.System) (*Artifact, error) {
	a := &Artifact{
		Universe: make(map[string]*resolve.LocalClient),
		Graph:    make(map[string]*resolve.Graph),
	}
	sc := bufio.NewScanner(r)
	var parsedTests []*parsedTest
	seenTest := make(map[string]bool)
	for line := 1; sc.Scan(); line++ {
		curLine := line
		l := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(strings.ToLower(l), startBlockUniverse):
			name, err := parseName(l[len(startBlockUniverse):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", curLine, err)
			}
			if a.Universe[name] != nil {
				return nil, fmt.Errorf("line %d: duplicate universe name: %q", curLine, name)
			}
			a.Universe[name], err = parseUniverse(sc, &line, sys)
			if err != nil {
				return nil, fmt.Errorf("line %d: parsing universe: %w", curLine, err)
			}

		case strings.HasPrefix(strings.ToLower(l), startBlockGraph):
			name, err := parseName(l[len(startBlockGraph):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", curLine, err)
			}
			if a.Graph[name] != nil {
				return nil, fmt.Errorf("line %d: duplicate graph name: %q", curLine, name)
			}
			a.Graph[name], err = parseGraph(sc, &line, sys)
			if err != nil {
				return nil, fmt.Errorf("line %d: parsing graph %s: %v", curLine, name, err)
			}

		case strings.HasPrefix(strings.ToLower(l), startBlockTest):
			name, err := parseName(l[len(startBlockTest):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", curLine, err)
			}
			if seenTest[name] {
				return nil, fmt.Errorf("line %d: duplicate test name: %q", curLine, name)
			}
			t, err := parseTest(sc, &line, sys, name)
			if err != nil {
				return nil, fmt.Errorf("line %d: cannot parse test: %w", curLine, err)
			}
			parsedTests = append(parsedTests, t)
			seenTest[name] = true
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	// Convert parsed tests into artifact tests.
	a.Test = make([]*Test, len(parsedTests))
	for i, pt := range parsedTests {
		t := &Test{
			Name:      pt.name,
			VK:        pt.resolve,
			Graph:     a.Graph[pt.graph],
			GraphName: pt.graph,
			Flags:     pt.flags,
		}
		us := make(map[string]*resolve.LocalClient)
		for _, u := range strings.Split(pt.universe, ",") {
			s := strings.Split(u, ":")
			switch n := len(s); n {
			case 1:
				us[""] = a.Universe[strings.TrimSpace(s[0])]
			case 2:
				us[strings.TrimSpace(s[0])] = a.Universe[strings.TrimSpace(s[1])]
			default:
				return nil, fmt.Errorf("test %s: cannot parse universe: %d colons, want 0 or 1", pt.name, n)
			}
		}
		u, err := newMultiverse(us)
		if err != nil {
			return nil, fmt.Errorf("test %s: cannot generate multiverse: %w", pt.name, err)
		}
		t.Universe = u
		a.Test[i] = t
	}

	return a, nil
}

// newMultiverse gathers content from all the given universes and returns
// a graph containing all the gathered versions. The registry attribute
// of each concrete version is set to a comma-separated list of ids of the
// given universes in which the version was found.
func newMultiverse(clients map[string]*resolve.LocalClient) (*resolve.LocalClient, error) {
	type ver struct {
		v          resolve.Version
		imports    []resolve.RequirementVersion
		registries []string
	}
	ctx := context.Background()
	pks := make(map[resolve.PackageKey]map[resolve.VersionKey]ver)
	for id, c := range clients {
		for _, vs := range c.PackageVersions {
			for _, v := range vs {
				if pks[v.PackageKey] == nil {
					pks[v.PackageKey] = make(map[resolve.VersionKey]ver)
				}
				ver := pks[v.PackageKey][v.VersionKey]
				// Overwrite the imports and attributes
				ver.v = v
				reqs, err := c.Requirements(ctx, v.VersionKey)
				if err != nil {
					return nil, err
				}
				ver.imports = reqs
				ver.registries = append(ver.registries, id)

				pks[v.PackageKey][v.VersionKey] = ver
			}
		}
	}
	// Populate a new client.
	cl := resolve.NewLocalClient()
	for _, vs := range pks {
		for _, v := range vs {
			if len(v.registries) > 1 || v.registries[0] != "" {
				sort.Strings(v.registries)
				regs := strings.Join(v.registries, ",")
				a, ok := v.v.GetAttr(version.Registries)
				if ok {
					regs += "," + a
				}
				v.v.SetAttr(version.Registries, regs)
			}
			cl.AddVersion(v.v, v.imports)
		}
	}
	return cl, nil
}

func parseName(s string) (string, error) {
	ts := strings.TrimSpace(s)
	if ts == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	return ts, nil
}

func parseUniverse(sc *bufio.Scanner, line *int, sys resolve.System) (*resolve.LocalClient, error) {
	var lines []string
	for sc.Scan() {
		*line++
		l := sc.Text()
		if strings.TrimSpace(strings.ToLower(l)) == endBlock {
			return buildUniverse(strings.Join(lines, "\n"), sys)
		}
		lines = append(lines, l)
	}
	return nil, fmt.Errorf("%w, want %q", io.ErrUnexpectedEOF, endBlock)
}

func buildUniverse(s string, sys resolve.System) (*resolve.LocalClient, error) {
	sch, err := schema.New(s, sys)
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	return sch.NewClient(), nil
}

func parseGraph(sc *bufio.Scanner, line *int, sys resolve.System) (*resolve.Graph, error) {
	var lines []string
	for sc.Scan() {
		*line++
		l := sc.Text()
		if strings.TrimSpace(strings.ToLower(l)) == endBlock {
			return schema.ParseResolve(strings.Join(lines, "\n"), sys)
		}
		lines = append(lines, l)
	}
	return nil, fmt.Errorf("%w, want %q", io.ErrUnexpectedEOF, endBlock)
}

func parseTest(sc *bufio.Scanner, line *int, sys resolve.System, name string) (*parsedTest, error) {
	t := &parsedTest{
		name: name,
	}
	for sc.Scan() {
		*line++
		l := strings.TrimSpace(sc.Text())

		switch {
		case strings.ToLower(l) == endBlock:
			return t, nil

		case strings.HasPrefix(strings.ToLower(l), prefixTestUniverse):
			n, err := parseName(l[len(prefixTestUniverse):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			t.universe = n

		case strings.HasPrefix(strings.ToLower(l), prefixTestGraph):
			n, err := parseName(l[len(prefixTestGraph):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			t.graph = n

		case strings.HasPrefix(strings.ToLower(l), prefixTestResolve):
			fields := strings.Fields(l[len(prefixTestResolve):])
			if len(fields) != 2 {
				return nil, fmt.Errorf("line %d: invalid version string %q", line, l)
			}
			t.resolve = resolve.VersionKey{
				PackageKey: resolve.PackageKey{
					System: sys,
					Name:   fields[0],
				},
				VersionType: resolve.Concrete,
				Version:     fields[1],
			}

		case strings.HasPrefix(strings.ToLower(l), prefixTestFlag):
			for _, flag := range strings.Fields(l[len(prefixTestFlag):]) {
				if t.flags == nil {
					t.flags = make(map[string]bool)
				}
				t.flags[flag] = true
			}
		}
	}
	return nil, fmt.Errorf("%w, want %q", io.ErrUnexpectedEOF, endBlock)
}
