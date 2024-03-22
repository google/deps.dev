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
package_lock_licenses is a simple example application that reads dependencies
from an npm package-lock.json file and fetches their licenses from the deps.dev
gRPC API.

The output from this application is the same as
examples/go/package_lock_licences_batch, but it retrieves licenses by making
concurrent calls to GetVersion rather than by making calls to GetVersionBatch.

It assumes well-formed input and is not meant as an example of how to write a
robust lockfile parser.
*/
package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "deps.dev/api/v3"
)

// NPMPackageLock represents a package-lock.json file used by the npm package
// management system.
// https://docs.npmjs.com/cli/v6/configuring-npm/package-lock-json
type NPMPackageLock struct {
	Name         string                   `json:"name"`
	Version      string                   `json:"version"`
	Dependencies map[string]NPMDependency `json:"dependencies"`
}

// NPMDependency represents a dependency read from a package-lock.json file.
// Note that this type is recursive. In npm, dependencies may have nested
// dependencies without limit.
type NPMDependency struct {
	Version      string                   `json:"version"`
	Bundled      bool                     `json:"bundled"`
	Dev          bool                     `json:"dev"`
	Optional     bool                     `json:"optional"`
	Dependencies map[string]NPMDependency `json:"dependencies"`
}

type Version struct {
	Name    string
	Version string
}

type VersionResponse struct {
	Licenses []string
	Error    error
}

var (
	includeDevDeps      = flag.Bool("dev", false, "whether to include dev dependencies")
	includeOptionalDeps = flag.Bool("optional", false, "whether to include optional dependencies")
)

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: package_lock_licenses [flags] package-lock.json\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	filename := flag.Arg(0)

	// Read and parse the lockfile.
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Reading file %q: %v", filename, err)
	}
	var pl NPMPackageLock
	if err := json.Unmarshal(data, &pl); err != nil {
		log.Fatalf("Parsing file %q: %v", filename, err)
	}

	// Traverse the dependency tree and find its set of unique package versions,
	// including the root.
	versions := map[Version]*VersionResponse{Version{pl.Name, pl.Version}: new(VersionResponse)}
	toVisit := []NPMDependency{{Version: pl.Version, Dependencies: pl.Dependencies}}
	for len(toVisit) > 0 {
		it := toVisit[0]
		toVisit = toVisit[1:]
		for name, dep := range it.Dependencies {
			if dep.Bundled {
				log.Printf("Skipping bundled dependency %s@%s", name, dep.Version)
				continue
			}
			if dep.Dev && !*includeDevDeps {
				continue
			}
			if dep.Optional && !*includeOptionalDeps {
				continue
			}
			versions[Version{name, dep.Version}] = new(VersionResponse)
			toVisit = append(toVisit, dep)
		}
	}

	// Create and configure a client for the gRPC API.
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

	// Fetch licenses from the deps.dev API.
	// To speed things up, use an error group to make many requests
	// concurrently, but limit the rate to 500 requests/second.
	// Note that gRPC will multiplex multiple requests over a single HTTP/2
	// connection.
	g, ctx := errgroup.WithContext(context.Background())
	limiter := rate.NewLimiter(500, 1)
	for v := range versions {
		r := versions[v]
		req := pb.GetVersionRequest{
			VersionKey: &pb.VersionKey{
				System:  pb.System_NPM,
				Name:    v.Name,
				Version: v.Version,
			},
		}
		g.Go(func() error {
			if err := limiter.Wait(ctx); err != nil {
				return err
			}
			resp, err := client.GetVersion(ctx, &req)
			switch status.Code(err) {
			case codes.OK:
				r.Licenses = resp.Licenses
			case codes.NotFound:
				r.Error = err
			default:
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("Fetching licenses: %v", err)
	}

	// Print each package version and its license on stdout.
	for v, r := range versions {
		fmt.Printf("%s@%s: ", v.Name, v.Version)
		if r.Error != nil {
			fmt.Printf("error: %v", r.Error)
		} else {
			fmt.Printf("%s", strings.Join(r.Licenses, " "))
		}
		fmt.Printf("\n")
	}
}
