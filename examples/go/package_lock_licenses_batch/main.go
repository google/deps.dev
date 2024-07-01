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

/*
package_lock_licenses_batch is a simple example application that reads
dependencies from an npm package-lock.json file and fetches their licenses from
the deps.dev HTTP API.

The output from this application is the same as
examples/go/package_lock_licences, but it retrieves licenses by calling the
GetVersionBatch endpoint rather than by making concurrent calls to GetVersion.

It assumes well-formed input and is not meant as an example of how to write a
robust lockfile parser.
*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

// Version is an internal representation of a package version.
type Version struct {
	Name    string
	Version string
}

// Result holds the license details for a version.
type Result struct {
	LicenseDetails []License
}

// License corresponds to the v3alpha API definition of Version.License.
type License struct {
	License string `json:"license"`
	SPDX    string `json:"spdx"`
}

// VersionKey corresponds to the v3alpha API definition of a VersionKey.
type VersionKey struct {
	System  string `json:"system"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GetVersionRequest corresponds to the v3alpha API definition of GetVersionRequest.
type GetVersionRequest struct {
	VersionKey VersionKey `json:"versionKey"`
}

// GetVersionBatchRequest corresponds to the v3alpha API definition of GetVersionRequest.
type GetVersionBatchRequest struct {
	Requests  []GetVersionRequest `json:"requests"`
	PageToken string              `json:"pageToken,omitempty"`
}

// VersionResponse corresponds to the v3alpha API definition of VersionBatch.Response.
type VersionResponse struct {
	Request GetVersionRequest `json:"request"`
	Version struct {
		VersionKey     VersionKey `json:"versionKey"`
		LicenseDetails []License  `json:"licenseDetails"`
	} `json:"version"`
}

// VersionBatch corresponds to the v3alpha API definition of VersionBatch.
type VersionBatch struct {
	Responses     []VersionResponse `json:"responses"`
	NextPageToken string            `json:"nextPageToken"`
}

var (
	includeDevDeps      = flag.Bool("dev", false, "whether to include dev dependencies")
	includeOptionalDeps = flag.Bool("optional", false, "whether to include optional dependencies")
)

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: package_lock_licenses_batch [flags] package-lock.json\n\nFlags:\n")
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
	versions := map[Version]*Result{Version{pl.Name, pl.Version}: new(Result)}
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
			versions[Version{name, dep.Version}] = new(Result)
			toVisit = append(toVisit, dep)
		}
	}

	// Construct the batch request from the unique package versions
	// collected earlier.
	var req GetVersionBatchRequest
	for v := range versions {
		req.Requests = append(req.Requests, GetVersionRequest{
			VersionKey: VersionKey{
				System:  "NPM",
				Name:    v.Name,
				Version: v.Version,
			},
		})
	}

	// Keep making requests until we have received responses for all
	// versions.
	for {
		// Make the request.
		b, err := json.Marshal(req)
		if err != nil {
			log.Fatalf("marshalling POST body: %v", err)
		}
		url := "https://api.deps.dev/v3alpha/versionbatch"
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
		if err != nil {
			log.Fatalf("creating POST request: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("POST request return status: %v", resp.StatusCode)
		}

		// Collect licenses from the response.
		var batch VersionBatch
		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("reading reponse: %v", err)
		}
		resp.Body.Close()
		if err := json.Unmarshal(b, &batch); err != nil {
			log.Fatalf("unmarshalling response: %v", err)
		}
		for _, response := range batch.Responses {
			v := Version{
				Name:    response.Request.VersionKey.Name,
				Version: response.Request.VersionKey.Version,
			}
			if (response.Version.VersionKey == VersionKey{}) {
				// An empty Version field means that the requested
				// version was not found.
				versions[v] = nil
			} else {
				versions[v].LicenseDetails = response.Version.LicenseDetails
			}
		}

		// An empty page token means there are no more repsonses to
		// fetch.
		if batch.NextPageToken == "" {
			break
		}
		// We haven't received responses for all requests yet, populate
		// the NextPageToken field in preparation for the next request.
		req.PageToken = batch.NextPageToken
	}

	// Print each package version and its license details on stdout.
	for v, r := range versions {
		fmt.Printf("%s@%s:", v.Name, v.Version)
		if r == nil {
			fmt.Printf(" error: version not found")
		} else {
			for _, l := range r.LicenseDetails {
				fmt.Printf(" %s", l.SPDX)
				if l.SPDX == "non-standard" {
					fmt.Printf(" (%s)", l.License)
				}
			}
		}
		fmt.Printf("\n")
	}
}
