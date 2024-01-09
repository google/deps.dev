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

package resolve

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"deps.dev/util/resolve/dep"
)

// NodeID identifies a node in a Graph.
// It is always scoped to a specific Graph, and is an index of the Nodes slice
// in that Graph.
type NodeID int

// Node is a concrete version in a resolved dependency Graph.
type Node struct {
	Version VersionKey
	Errors  []NodeError
}

// NodeError holds error information for a Node's Requirement.
type NodeError struct {
	Req   VersionKey
	Error string
}

func (ne NodeError) Compare(other NodeError) int {
	if c := ne.Req.Compare(other.Req); c != 0 {
		return c
	}
	return strings.Compare(ne.Error, other.Error)
}

// Edge represents a resolution From an importer Node To an imported Node,
// satisfying the importer's Requirement for the given dependency Type.
type Edge struct {
	From        NodeID
	To          NodeID
	Requirement string
	Type        dep.Type
}

// Graph holds the result of a dependency resolution.
type Graph struct {
	// The first element in the slice is the root node.
	// NodeID is the index into this slice.
	Nodes []Node

	Edges []Edge

	// Error is a graph-wide resolution error that is set if the resolver
	// was not able to perform the resolution based on its input data; it
	// is not used for errors that are independent of the data such as a
	// network connection problem.
	Error string

	// Duration is the time it took to perform this resolution.
	Duration time.Duration
}

// AddNode inserts a node into the graph, not connected to anything. The
// returned ID is required to add edges.
func (g *Graph) AddNode(vk VersionKey) NodeID {
	g.Nodes = append(g.Nodes, Node{
		Version: vk,
	})
	return NodeID(len(g.Nodes) - 1)
}

// AddEdge inserts an edge in the graph between the two provided nodes.
func (g *Graph) AddEdge(from, to NodeID, req string, t dep.Type) error {
	if !g.contains(from) {
		return fmt.Errorf("node not in graph: %v", from)
	}
	if !g.contains(to) {
		return fmt.Errorf("node not in graph: %v", to)
	}
	g.Edges = append(g.Edges, Edge{
		From:        from,
		To:          to,
		Requirement: req,
		Type:        t,
	})
	return nil
}

// AddError associates a resolution error with a node and the requirement that
// caused the error.
func (g *Graph) AddError(n NodeID, req VersionKey, err string) error {
	if !g.contains(n) {
		return fmt.Errorf("node not in graph: %v", n)
	}
	g.Nodes[n].Errors = append(g.Nodes[n].Errors, NodeError{
		Req:   req,
		Error: err,
	})
	return nil
}

// contains checks if a provided NodeID is actually in the graph.
func (g *Graph) contains(n NodeID) bool {
	return n >= 0 && int(n) < len(g.Nodes)
}

// Canon converts the graph (in place) into a canonicalized representation,
// suitable for comparing with other graphs.
// If it fails then the graph is still valid but won't be a canonical form.
func (g *Graph) Canon() error {
	// Sort NodeErrors.
	for _, n := range g.Nodes {
		sort.Slice(n.Errors, func(i, j int) bool {
			return n.Errors[i].Compare(n.Errors[j]) < 0
		})
	}

	// See if the graph can be canonicalized purely based on sorting the nodes.
	on := newOrderedNodes(g.Nodes)
	on.KeepZero = true // maintain root
	sort.Sort(on)
	if on.Root != 0 {
		// This indicates a severe error in orderedNodes.
		panic("root " + g.Nodes[on.Root].Version.String() + " no longer at index 0")
	}
	g.renumber(on.Mapping(), false)

	if on.Dupe {
		// If there were duplicate nodes, the prior sort did not yield a
		// canonical ordering. Perform a more expensive BFS canonicalisation.
		// Unfortunately this needs to be done after the edge/root renumbering
		// because the initial orderedNodes sort rearranged g.Nodes.
		m, err := g.canonBFS()
		if err != nil {
			return err
		}
		g.renumber(m, true)
	}

	return nil
}

// renumber renumbers the graph's edges and root node based on the given mapping
// of old to new node IDs.
func (g *Graph) renumber(oldToNew []int, includeNodes bool) {
	if includeNodes {
		nn := make([]Node, len(g.Nodes))
		for i, j := range oldToNew {
			nn[j] = g.Nodes[i]
		}
		g.Nodes = nn
	}
	// Renumber the edges and sort them.
	for i, e := range g.Edges {
		e.From = NodeID(oldToNew[e.From])
		e.To = NodeID(oldToNew[e.To])
		g.Edges[i] = e
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		ei, ej := g.Edges[i], g.Edges[j]
		if ej.From != ei.From {
			return ei.From < ej.From
		}
		if ei.To != ej.To {
			return ei.To < ej.To
		}
		if ei.Requirement != ej.Requirement {
			return ei.Requirement < ej.Requirement
		}
		return ei.Type.Compare(ej.Type) < 0
	})
}

func (g *Graph) canonBFS() ([]int, error) {
	/*
		Algorithm: Perform a BFS of the graph, starting at the root.
		Label each node in sequence as encountered, and enqueue that
		node's adjacent nodes in the standard order.

		This implementation make some effort to avoid allocations, even
		at the cost of doing extra memory copies. The latter are
		significantly cheaper since most graphs are quite small.

		TODO: There might be a more clever way to do this by using on.IDs.
	*/

	// Build ragged adjacency matrix.
	edges := make([][]int, len(g.Nodes))
	for _, e := range g.Edges {
		edges[int(e.From)] = append(edges[int(e.From)], int(e.To))
	}

	oldToNew := make([]int, len(g.Nodes)) // Final result (-1 means unlabeled).
	nextLabel := 0
	queue := []int{int(0)}
	for i := range oldToNew {
		oldToNew[i] = -1
	}

	var onScratch orderedNodes // Scratch work for adjacent node sorting.
	for len(queue) > 0 {
		n := queue[0]
		copy(queue, queue[1:])
		queue = queue[:len(queue)-1]
		if oldToNew[n] > -1 {
			continue
		}

		// Process n.
		oldToNew[n] = nextLabel
		nextLabel++

		// Build a list of the adjacent nodes that are not yet labeled.
		onScratch.Nodes, onScratch.IDs = onScratch.Nodes[:0], onScratch.IDs[:0]
		for _, to := range edges[n] {
			if oldToNew[to] == -1 {
				onScratch.Nodes = append(onScratch.Nodes, g.Nodes[to])
				onScratch.IDs = append(onScratch.IDs, to)
			}
		}
		if len(onScratch.Nodes) > 1 {
			sort.Sort(&onScratch)
			if onScratch.Dupe {
				return nil, fmt.Errorf("graph node %v has duplicate direct dependency", g.Nodes[n].Version)
			}
		}
		queue = append(queue, onScratch.IDs...)
	}
	if rem := len(g.Nodes) - nextLabel; rem > 0 {
		return nil, fmt.Errorf("failed labeling all nodes; %d are unreachable from root", rem)
	}

	return oldToNew, nil
}

// orderedNodes is a sort.Interface for a slice of Node values.
// It rearranges the Nodes and IDs slices in parallel while sorting.
type orderedNodes struct {
	KeepZero bool // whether to keep the zero node in its place

	Nodes []Node
	IDs   []int // indexes into Nodes

	// Quicksort may move the root away from Nodes[0], even though Less(0, x) is true ∀x > 0.
	// Track any root changes; by the end of the sort this should be zero.
	Root int

	// Dupe is set to true if a duplicate Node is found during sorting.
	Dupe bool
}

func newOrderedNodes(nodes []Node) *orderedNodes {
	ids := make([]int, len(nodes))
	for i := range ids {
		ids[i] = i
	}
	return &orderedNodes{Nodes: nodes, IDs: ids}
}

// Mapping returns the mapping of old to new indexes.
// It is the inverse of n.IDs after sorting.
func (n *orderedNodes) Mapping() []int {
	m := make([]int, len(n.IDs))
	for i, j := range n.IDs {
		m[j] = i
	}
	return m
}

func (n *orderedNodes) Len() int { return len(n.IDs) }
func (n *orderedNodes) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
	n.IDs[i], n.IDs[j] = n.IDs[j], n.IDs[i]

	if i == n.Root {
		n.Root = j
	} else if j == n.Root {
		n.Root = i
	}
}
func (n *orderedNodes) Less(i, j int) bool {
	// Always compare so duplicates can be discovered, even duplicates of the root.
	ni, nj := n.Nodes[i], n.Nodes[j]
	c := ni.Compare(nj)
	if c == 0 {
		n.Dupe = true
	}
	if n.KeepZero && (i == n.Root || j == n.Root) {
		// The root is less than every other element.
		return i == n.Root
	}
	return c < 0
}

func (n Node) Compare(o Node) int {
	if c := n.Version.Compare(o.Version); c != 0 {
		return c
	}
	// They must have the same version, are the error slices different?
	if li, lj := len(n.Errors), len(o.Errors); li < lj {
		return -1
	} else if li > lj {
		return 1
	}
	for i := range n.Errors {
		if c := n.Errors[i].Compare(o.Errors[i]); c != 0 {
			return c
		}
	}
	return 0 // They must be equal.
}

// String produces a text representation of the graph.
// The graph is represented by a spanning tree computed using the creator
// relationship (when available, first edge otherwise).
// Extraneous (non creating) edges are represented using labels.
// The representation is recognized by the resolve graph schema.
func (g *Graph) String() string {
	var b strings.Builder
	if g.Error != "" {
		for _, l := range strings.Split(g.Error, "\n") {
			fmt.Fprintf(&b, "ERROR: %s\n", l)
		}
	}
	if len(g.Nodes) == 0 {
		return b.String()
	}

	// Get for each node its unique creator and count of dependents.
	creator := make(map[NodeID]NodeID, len(g.Nodes))
	dependents := make([]int, len(g.Nodes))
	// The root creates itself.
	creator[0] = 0
	// The root dependents is set to 1, itself, so that having a node
	// referring to it (creating a cycle) will make the dependents increment to
	// 2: that way, the root can be treated as any other node in term of
	// labeling.
	dependents[0] = 1
	for _, e := range g.Edges {
		dependents[e.To]++

		// Do not overwrite an already set creator, so that EdgeExtra are not
		// overwritten if that creator relationship came from such edge.
		// A node cannot be its own creator, look for another edge for
		// creation.
		if _, ok := creator[e.To]; !ok && e.To != e.From {
			creator[e.To] = e.From
		}
	}

	// node represents a node in the tree, and ultimately a line in the art.
	// Tree nodes coincide with graph nodes for all the created nodes.
	// Additional nodes are added to the tree, all leaves, that represent the
	// non-creating edges (in which case the resolve node is nil) and the
	// errors.
	type node struct {
		label    int
		nid      NodeID
		n        *Node
		req      string
		err      string
		children []*node
		dt       dep.Type
	}
	nodes := make([]*node, len(g.Nodes))
	label := 0
	nbChildren := 0
	// Create one tree node for each graph node.
	for i, n := range g.Nodes {
		id, n := NodeID(i), n
		nodes[id] = &node{
			nid: id,
			n:   &n,
		}
		// If there are dependents in addition to the creating node, label
		// the node.
		if dependents[id] > 1 {
			label++
			nodes[id].label = label
		}
	}
	// Add the edges as children in the tree. If this is a creating edge,
	// (and the first one, in case there are several direct edges with
	// different type for example) reuse the tree node of the created graph
	// node, otherwise create a tree leaf that contains the label.
	seen := make([]bool, len(g.Nodes))
	for _, e := range g.Edges {
		nf, nt := nodes[e.From], nodes[e.To]
		if e.From != creator[e.To] || seen[e.To] || e.From == e.To {
			nt = &node{label: nt.label}
		}
		if e.From == creator[e.To] {
			seen[e.To] = true
		}
		nt.req = e.Requirement
		nf.children = append(nf.children, nt)
		nt.dt = e.Type
		nbChildren++
	}
	// Add errors as leaves in the tree.
	for i, n := range g.Nodes {
		tn := nodes[i]
		for _, ne := range n.Errors {
			tn.children = append(tn.children, &node{
				n:   &Node{Version: ne.Req},
				req: ne.Req.Version,
				err: ne.Error,
			})
			nbChildren++
		}
	}

	// DFS the tree, add a line in art per tree node, using the schema format.
	// Because this is art, the node's line gets prefix1, and its children get
	// prefix2.
	seen = make([]bool, len(g.Nodes))
	var walk func(n *node, req, prefix1, prefix2 string)
	walk = func(n *node, req, prefix1, prefix2 string) {
		seen[n.nid] = true
		fmt.Fprint(&b, prefix1)
		if n.n == nil {
			if !n.dt.IsRegular() {
				fmt.Fprintf(&b, "%s | ", n.dt)
			}
			fmt.Fprintf(&b, "$%d@%s\n", n.label, req)
			return
		}
		if n.label > 0 {
			fmt.Fprintf(&b, "%d: ", n.label)
		}
		if !n.dt.IsRegular() {
			fmt.Fprintf(&b, "%s | ", n.dt)
		}
		pt := ""
		if prefix1 == "" {
			// Root has no requirement.
			fmt.Fprintf(&b, "%s%s ", pt, n.n.Version.Name)
		} else {
			fmt.Fprintf(&b, "%s%s@%s ", pt, n.n.Version.Name, req)
		}
		if n.err != "" {
			fmt.Fprintf(&b, "ERROR: %s\n", n.err)
		} else {
			fmt.Fprintf(&b, "%s\n", n.n.Version.Version)
		}
		for i, c := range n.children {
			p1 := "├─ "
			p2 := "│  "
			if i == len(n.children)-1 {
				p1 = "└─ "
				p2 = "   "
			}
			walk(c, c.req, prefix2+p1, prefix2+p2)
		}
	}
	walk(nodes[0], "", "", "")
	for i, ok := range seen {
		if !ok {
			fmt.Fprintf(&b, "ORPHAN: %s\n", g.Nodes[i])
		}
	}
	return b.String()
}
