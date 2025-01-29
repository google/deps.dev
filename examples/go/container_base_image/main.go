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
container_base_image is a simple example application that queries the deps.dev
API for the base image(s) of an Open Container Image (OCI) compliant tarball.

The OCI spec is defined at
https://github.com/opencontainers/image-spec/blob/main/spec.md.

To produce an OCI-compliant tarball using the docker command, run `docker image
save <image id>` with a docker client v25.0 or above.
*/
package main

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
)

// response is used to unmarshal QueryContainerImage responseses.
type response struct {
	Results []struct {
		Repository string
	}
}

// https://github.com/opencontainers/image-spec/blob/main/image-layout.md#oci-layout-file
type ociLayout struct {
	ImageLayoutVersion string `json:"imageLayoutVersion"`
}

// https://github.com/opencontainers/image-spec/blob/main/image-index.md
type index struct {
	Manifests []struct {
		Digest string `json:"digest"`
	} `json:"manifests"`
}

// https://github.com/opencontainers/image-spec/blob/main/manifest.md
type manifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
}

// https://github.com/opencontainers/image-spec/blob/main/config.md
type config struct {
	RootFS struct {
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: container_base_image <image.tar>\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	tarArchive := flag.Arg(0)
	// Check that the tar declares itself to to be OCI 1.0.0 compliant.
	var layout ociLayout
	if err := unmarshalFile(tarArchive, "oci-layout", &layout); err != nil {
		log.Fatalf("%v\nAre you using a docker client version >=25.0 to save the image?", err)
	}
	if layout.ImageLayoutVersion != "1.0.0" {
		log.Fatalf("The oci-layout file lists version %v which is not supported.", layout.ImageLayoutVersion)
	}

	// Find the manifest(s) for the image. There may be multiple.
	var idx index
	if err := unmarshalFile(tarArchive, "index.json", &idx); err != nil {
		log.Fatalf("%v", err)
	}
	fmt.Printf("%d manifest(s) found\n", len(idx.Manifests))

	// For each manifest, look up the base image(s).
	for _, m := range idx.Manifests {
		// Read the manifest file, which contains the config digest.
		var mt manifest
		id, _ := strings.CutPrefix(m.Digest, "sha256:")
		if err := unmarshalFile(tarArchive, fmt.Sprintf("blobs/sha256/%s", id), &mt); err != nil {
			log.Fatalf("%v", err)
		}
		// Read the config file, which contains the diff IDs.
		var c config
		id, _ = strings.CutPrefix(mt.Config.Digest, "sha256:")
		if err := unmarshalFile(tarArchive, fmt.Sprintf("blobs/sha256/%s", id), &c); err != nil {
			log.Fatalf("%v", err)
		}
		// For each chain ID, query the deps.dev api to determine
		// whether it's a known base image.
		chainIDs := makeChainIDs(c.RootFS.DiffIDs)
		for i, chainID := range chainIDs {
			url := "https://api.deps.dev/v3alpha/querycontainerimages/" + chainID
			resp, err := http.Get(url)
			if err != nil {
				log.Fatalf("Request: %v", err)
			}
			switch resp.StatusCode {
			case http.StatusNotFound:
				fmt.Printf("Layer %d: no base image found\n", i)
			case http.StatusOK:
				var respBody response
				err = json.NewDecoder(resp.Body).Decode(&respBody)
				if err != nil {
					log.Fatalf("Decoding response body: %v", err)
				}
				var repos []string
				for _, r := range respBody.Results {
					repos = append(repos, r.Repository)
				}
				fmt.Printf("Layer %d: %v\n", i, strings.Join(repos, " "))
			default:
				log.Fatalf("Response: %v", resp.Status)
			}
			resp.Body.Close()
		}
	}
}

// unmarshalFile looks for the file with the specified filename in the specified
// tarArchive.
func unmarshalFile(tarArchive string, filename string, v any) error {
	f, err := os.Open(tarArchive)
	if err != nil {
		return err
	}
	tr := tar.NewReader(f)
	var b []byte
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if hdr.Name == filename {
			b, err = io.ReadAll(io.Reader(tr))
			if err != nil {
				return err
			}
			break
		}
	}
	if len(b) == 0 {
		return fmt.Errorf("No %s in tar archive", filename)
	}

	return json.Unmarshal(b, v)
}

// makeChainIDs computes the chain ID for each prefix of layers. An OCI chain
// ID refers to a sequence of layers with a single identifier.
// https://github.com/opencontainers/image-spec/blob/main/config.md#layer-chainid
func makeChainIDs(diffIDs []string) []string {
	if len(diffIDs) == 0 {
		return nil
	}
	diffDigests := make([]digest.Digest, len(diffIDs))
	for i, diffID := range diffIDs {
		diffDigests[i] = digest.Digest(diffID)
	}
	chainDigests := identity.ChainIDs(diffDigests)
	chainIDs := make([]string, len(chainDigests))
	for i, chainDigest := range chainDigests {
		chainIDs[i] = string(chainDigest)
	}
	return chainIDs
}
