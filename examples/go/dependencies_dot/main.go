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
dependencies_dot is a simple example application that fetches a resolved
dependency graph from the deps.dev HTTP API and renders it in the DOT language
used by Graphviz.

With Graphviz installed on your system, you can use it to create a visual
representation of a resolved dependency graph like so:

	dependencies_dot npm react 15.0.0 > deps.dot
	dot -Tpng deps.dot > deps.png

For more information about Graphviz and DOT, see https://graphviz.org/
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

type Dependencies struct {
	Nodes []Node
	Edges []Edge
	Error string
}

type Node struct {
	VersionKey VersionKey
	Errors     []string
}

type Edge struct {
	FromNode    int
	ToNode      int
	Requirement string
}

type VersionKey struct {
	System  string
	Name    string
	Version string
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dependencies_dot <system> <package> <version>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(1)
	}
	system := flag.Arg(0)
	name := flag.Arg(1)
	version := flag.Arg(2)

	// Fetch a resolved dependency graph from the deps.dev API.
	// Request parameters passed as path segments must be escaped,
	// as they may contain characters like '/'.
	url := "https://api.deps.dev/v3alpha/systems/" + url.PathEscape(system) + "/packages/" + url.PathEscape(name) + "/versions/" + url.PathEscape(version) + ":dependencies"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Response: %v", resp.Status)
	}
	var deps Dependencies
	err = json.NewDecoder(resp.Body).Decode(&deps)
	if err != nil {
		log.Fatalf("Decoding response body: %v", err)
	}

	// Check the graph for resolution errors, and print on stderr.
	if deps.Error != "" {
		log.Printf("Warning: %s", deps.Error)
	}
	for _, n := range deps.Nodes {
		for _, e := range n.Errors {
			log.Printf("Warning: %s", e)
		}
	}

	// Print the resolved dependency graph in DOT format on stdout.
	fmt.Printf("digraph {\n")
	for i, n := range deps.Nodes {
		fmt.Printf("  %d [label=%q];\n", i, n.VersionKey.Name+"@"+n.VersionKey.Version)
	}
	for _, e := range deps.Edges {
		fmt.Printf("  %d -> %d [label=%q];\n", e.FromNode, e.ToNode, e.Requirement)
	}
	fmt.Printf("}\n")
}
