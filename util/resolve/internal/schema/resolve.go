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
	"fmt"
	"strings"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/internal/deptest"
)

// resolveSchema represents a parsed description of a resolution graph.
type resolveSchema struct {
	// errs holds the error lines from the input not linked to a node.
	errs []string
	// rows holds the processed lines from the input.
	rows []resolveRow
	// labels maps labels to rows.
	labels map[string]int
}

// row represents a processed line from the schema.
type resolveRow struct {
	// line holds the line number, for error reporting.
	line int
	// depth holds the indentation level that defines the edge.
	// If depth is 0, the row defines the root.
	depth int
	// label, name, requirement, and version hold the label of the node,
	// the name of the package, the requirement yielding to the concrete.
	// If name and concrete are empty, label must not be empty, and refers to
	// another node's label.
	// If name and concrete are not empty, the row defines a node. The label,
	// then, if not empty, is associated to this node.
	label, name, requirement, concrete string
	// dt holds the dependency type of the edge.
	dt dep.Type
	// err holds the node error parsed by [ERROR: ].
	err string
}

/*
ParseResolve parses the given schema text and creates a resolve.Graph.

The schema describes a resolution graph of concrete versions (nodes) connected
by requirement versions (edges) that is used to construct a resolve.Graph
suitable for comparison to graphs returned by resolvers.

The schema may be declared using a simple, tab-sensitive grammar, with each
level of indentation representing an edge and each line representing an edge
specifier (a requirement) and a node (a concrete).
Lines are trimmed, and empty lines or lines starting with a `#` are skipped.

Items (label, requirement, concrete) on a line are space separated. In the
current implementation, names, requirements, concretes and labels must not
contain spaces.

	# name1 v1 --- name2@* ---> v2 -- name3@v1-4 --> v3
	#          \
	#           +- name4@v4 --> v4
	name1 v1
		name2@* v2
			name3@v1-4 v3
		name4@v4 v4

Each node can be labeled, so that it can be referenced in another line (in
such case the referring line does not create a node, and simply represents an
edge)

	# name1 v1 --- name2@* ---> v2 -- name3@v1-4 ----> v3
	#          \                                    /
	#           +- name4@v4 --> v4 -- name3@v2-5 --+
	name1 v1
		name2@* v2
			label: [DepType|]name3@v1-4 v3
		name4@v4 v4
			[DepType|]$label@v2-5

The first line defines the graph root. It may or may not have a label:

	[label: ]name concrete

Pattern of other lines, defining nodes, contains tabulation to implicitly
create an edge:

	tabs [label: ]name@requirement concrete

Pattern for rows referring a node by label:

	tabs $label@requirement

Pattern for rows defining an error linked to the parent node:

	tabs name@requirement ERROR: error

The indentation of a line is used to implicitly create an edge between nodes.
If the current line has an indentation of n, an edge will be created from the
closest preceding line that has an indentation of n-1 to the current line.
Furthermore, if a line has an indentation of n, the line immediately after must
have an indentation <= n+1 (it can reduce, stay, or increase by 1).
*/
func ParseResolve(text string, sys resolve.System) (*resolve.Graph, error) {
	s, err := parseResolve(text)
	if err != nil {
		return nil, err
	}

	g := &resolve.Graph{
		Error: strings.Join(s.errs, "\n"),
	}
	// Create nodes.
	nodes := make([]resolve.NodeID, len(s.rows))
	for i, r := range s.rows {
		// Skip labels.
		if r.name == "" {
			continue
		}

		// Do not create node for errors not linked to a concrete node.
		// They will be added to the parent.
		if r.err != "" && r.concrete == "" {
			continue
		}

		// Create node; the first node in the text must necessarily be
		// the root, and therefore the zeroth node.
		vk := resolve.VersionKey{
			PackageKey: resolve.PackageKey{
				System: sys,
				Name:   r.name,
			},
			VersionType: resolve.Concrete,
			Version:     r.concrete,
		}
		nodes[i] = g.AddNode(vk)
	}

	// Create edges.
	sources := make([]resolve.NodeID, len(g.Nodes)+1)
	for i, r := range s.rows {
		// Record the current index as the source at this indentation level.
		sources[r.depth] = nodes[i]

		// The root has no implicit ancestor.
		if r.depth == 0 {
			continue
		}

		src := sources[r.depth-1]
		if r.err != "" {
			vk := resolve.VersionKey{
				PackageKey: resolve.PackageKey{
					System: sys,
					Name:   r.name,
				},
				VersionType: resolve.Requirement,
				Version:     r.requirement,
			}
			if err := g.AddError(src, vk, r.err); err != nil {
				return nil, fmt.Errorf("cannot add an error to %s", g.Nodes[src].Version)
			}
			continue
		}
		dst := nodes[i]
		// Use the referred node if the row is a label.
		if r.name == "" {
			dst = nodes[s.labels[r.label]]
		}

		if err := g.AddEdge(src, dst, r.requirement, r.dt); err != nil {
			return nil, fmt.Errorf("cannot create edge from %s to %s", g.Nodes[src].Version, g.Nodes[dst].Version)
		}
	}

	if err := g.Canon(); err != nil {
		return nil, fmt.Errorf("canonicalizing graph: %v", err)
	}
	return g, nil
}

// parseResolve parses the given text and create a schema.
func parseResolve(text string) (*resolveSchema, error) {
	s := &resolveSchema{
		rows:   make([]resolveRow, 0, strings.Count(text, "\n")),
		labels: make(map[string]int),
	}

	// Extract the rows from the text.
	for i, line := range strings.Split(text, "\n") {
		// Skip comments and empty rows.
		tl := strings.TrimSpace(line)
		if i := strings.Index(tl, "#"); i == 0 || tl == "" {
			continue
		}
		// Extract errors not linked to a node.
		if strings.HasPrefix(tl, "ERROR:") {
			s.errs = append(s.errs, strings.TrimSpace(tl[6:]))
			continue
		}

		r := resolveRow{
			line: i + 1,
		}

		line = replaceArt(line)
		tl = strings.TrimSpace(line)

		// Infer node depth from tabs indentation.
		for _, c := range line {
			if c != '\t' {
				break
			}
			r.depth++
		}

		// Extract the node error.
		if i := strings.Index(tl, " ERROR: "); i != -1 {
			r.err = tl[i+8:]
			tl = tl[:i]
		}

		// The optional defining label comes first.
		if i := strings.Index(tl, ": "); i != -1 {
			r.label = tl[:i]
			tl = strings.TrimSpace(tl[i+2:])
		}

		// The optional dep type of the edge comes second.
		if i := strings.Index(tl, "|"); i != -1 {
			var err error
			r.dt, err = deptest.ParseString(tl[:i])
			if err != nil {
				return nil, err
			}
			tl = strings.TrimSpace(tl[i+1:])
		}

		// The rest is a labeled requirement or a requirement and a concrete.
		switch items := strings.Split(tl, " "); len(items) {
		case 1: // This is a labeled requirement or an error.
			requirement := items[0]
			if requirement[0] != '$' && r.err == "" {
				return nil, fmt.Errorf("line %d: expected a label, got %q", r.line, requirement)
			}
			if requirement[0] == '$' && r.err != "" {
				return nil, fmt.Errorf("line %d: didn't expect a label, got %q", r.line, requirement)
			}
			i := strings.Index(requirement, "@")
			if i < 0 {
				return nil, fmt.Errorf("line %d: expected a requirement, got %q", r.line, requirement)
			}
			if r.err == "" {
				r.label = requirement[1:i]
			} else {
				r.name = requirement[:i]
			}
			r.requirement = requirement[i+1:]
		case 2: // This is node defining line.
			if r.label != "" {
				s.labels[r.label] = len(s.rows)
			}
			requirement, concrete := items[0], items[1]
			i := strings.Index(requirement, "@")
			if i < 0 && r.depth == 0 { // This is the root.
				r.name = requirement
				r.concrete = concrete
				break
			}
			if i < 0 {
				return nil, fmt.Errorf("line %d: expected a requirement, got %q", r.line, requirement)
			}
			r.name = requirement[:i]
			r.requirement = requirement[i+1:]
			r.concrete = concrete
		default:
			return nil, fmt.Errorf("line %d: unexpected number of items (%d)", r.line, len(items))
		}

		s.rows = append(s.rows, r)
	}

	// Validate the schema.
	for i, r := range s.rows {
		// Not root?
		if i == 0 && r.depth > 0 {
			return nil, fmt.Errorf("line %d: row should be root (found %d depth for %s@%s)", r.line, r.depth, r.name, r.concrete)
		}
		// Several root?
		if i > 0 && r.depth == 0 {
			return nil, fmt.Errorf("line %d: only one row can be root (found %d depth for %s@%s)", r.line, r.depth, r.name, r.concrete)
		}
		// Skipped indentation level?
		if i > 0 && r.depth > s.rows[i-1].depth+1 {
			return nil, fmt.Errorf("line %d: skipped indentation level (%d > %d)", r.line, r.depth, s.rows[i-1].depth+1)
		}
		// Undefined labels?
		if _, ok := s.labels[r.label]; !ok && r.name == "" && r.err == "" {
			return nil, fmt.Errorf("line %d: undefined label %q", r.line, r.label)
		}
	}

	return s, nil
}

func replaceArt(s string) string {
	for _, p := range []string{"   ", "├─ ", "│  ", "└─ "} {
		if strings.HasPrefix(s, p) {
			return "\t" + replaceArt(s[len(p):])
		}
	}
	return s
}
