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
resolve is an example program that performs dependency resolution for
a single version of a published npm package, using the example resolver
implementation in deps.dev/resolve. It then compares the resulting graph
to the result from the GetDependencies endpoint.
*/
package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "deps.dev/api/v3alpha"
	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/npm"
)

const usage = "Usage: resolve <package-name> <package-version>"

func main() {
	log.SetFlags(0)
	if len(os.Args) != 3 {
		log.Fatal(usage)
	}

	root := resolve.VersionKey{
		PackageKey: resolve.PackageKey{
			System: resolve.NPM,
			Name:   os.Args[1],
		},
		VersionType: resolve.Concrete,
		Version:     os.Args[2],
	}

	// Set up gRPC API client.
	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Fatalf("Getting system cert pool: %v", err)
	}
	creds := credentials.NewClientTLSFromCert(certPool, "")
	conn, err := grpc.Dial("api.deps.dev:443", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("Dialing: %v", err)
	}
	client := pb.NewInsightsClient(conn)

	resolver := npm.NewResolver(resolve.NewAPIClient(client))
	ctx := context.Background()

	start := time.Now()
	log.Printf("Resolving: %v", root)
	g, err := resolver.Resolve(ctx, root)
	if err != nil {
		log.Fatal(err)
	}
	// Strip the dependency types for comparison.
	for i, e := range g.Edges {
		e.Type = dep.Type{}
		g.Edges[i] = e
	}
	if err := g.Canon(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Resolved in %v", time.Since(start))

	start = time.Now()
	log.Printf("GetDependencies(%v)", root)
	resp, err := client.GetDependencies(ctx, &pb.GetDependenciesRequest{
		VersionKey: &pb.VersionKey{
			System:  pb.System_NPM,
			Name:    root.Name,
			Version: root.Version,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	var g2 resolve.Graph
	for _, n := range resp.Nodes {
		g2.AddNode(resolve.VersionKey{
			PackageKey: resolve.PackageKey{
				System: resolve.NPM,
				Name:   n.VersionKey.Name,
			},
			VersionType: resolve.Concrete,
			Version:     n.VersionKey.Version,
		})
	}
	for _, e := range resp.Edges {
		g2.AddEdge(resolve.NodeID(e.FromNode), resolve.NodeID(e.ToNode), e.Requirement, dep.Type{})
	}
	if err := g2.Canon(); err != nil {
		log.Fatal(err)
	}
	log.Printf("GetDependencies in %v", time.Since(start))
	printGraphs(g, &g2)
}

// printGraphs prints the two resolved graphs side by side.
func printGraphs(local, remote *resolve.Graph) {
	s1 := strings.Split(local.String(), "\n")
	s2 := strings.Split(remote.String(), "\n")

	w := tabwriter.NewWriter(os.Stdout, 10, 2, 2, ' ', 0)
	fmt.Fprintf(w, "Local\tGetDependencies\n")
	for len(s1) > 0 && len(s2) > 0 {
		fmt.Fprintf(w, "%s\t%s\n", s1[0], s2[0])
		s1, s2 = s1[1:], s2[1:]
	}
	for _, l := range s1 {
		fmt.Fprintf(w, "%s\t\n", l)
	}
	for _, l := range s2 {
		fmt.Fprintf(w, "\t%s\t\n", l)
	}
	w.Flush()
	return
}
