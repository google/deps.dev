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
artifact_query is a simple example application that queries the deps.dev API by
file content hash.
*/
package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

type QueryResult struct {
	Versions []Version
}

type Version struct {
	VersionKey VersionKey
}

type VersionKey struct {
	System  string
	Name    string
	Version string
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: artifact_query <file>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	filename := flag.Arg(0)

	// Read the entire file into memory.
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Reading file: %v", err)
	}

	// Compute the SHA-1 hash of the file contents.
	hash := sha1.Sum(data)

	// Encode the hash as Base64, which is required when using the HTTP API.
	// When using the gRPC API, hash values are passed as bytes.
	hash64 := base64.StdEncoding.EncodeToString(hash[:])

	// Query the deps.dev API for package versions associated with artifacts matching the hash.
	url := "https://api.deps.dev/v3alpha/query?hash.type=SHA1&hash.value=" + url.QueryEscape(hash64)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Response: %v", resp.Status)
	}
	var result QueryResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Fatalf("Decoding response body: %v", err)
	}

	// Print all matching package versions.
	for _, v := range result.Versions {
		fmt.Printf("%s: %s@%s\n", v.VersionKey.System, v.VersionKey.Name, v.VersionKey.Version)
	}
}
