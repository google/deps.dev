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

package maven

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMetadata(t *testing.T) {
	input, err := os.ReadFile("testdata/maven-metadata.xml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	want := Metadata{
		GroupID:    "com.example",
		ArtifactID: "basic",
		Versioning: Versioning{
			Latest:  "3.0.0",
			Release: "3.0.0",
			Versions: []String{
				"1.0.0",
				"2.0.0",
				"3.0.0",
			},
			LastUpdated: "20240101000000",
			Snapshot: Snapshot{
				Timestamp:   "20240101.000000",
				BuildNumber: 1,
				LocalCopy:   true,
			},
			SnapshotVersions: []SnapshotVersion{
				{
					Classifier: "sources",
					Extension:  "jar",
					Value:      "4.0.0-SNAPSHOT",
					Updated:    "20240101000000",
				},
			},
		},
		Version: "4.0.0-SNAPSHOT",
		Plugins: []MetadataPlugin{
			{
				Name:       "plugin",
				Prefix:     "plugin",
				ArtifactID: "plugin",
			},
		},
	}
	var got Metadata
	if err := xml.Unmarshal(input, &got); err != nil {
		t.Fatalf("failed to unmarshal input: %v", err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unmarshal input got: %v,\n want: %v", got, want)
	}
}
