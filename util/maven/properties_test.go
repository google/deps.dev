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

func TestProperties(t *testing.T) {
	input, err := os.ReadFile("testdata/properties.xml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	want := Properties{
		Properties: []Property{
			{Name: "name", Value: "value"},
			{Name: "foo.version", Value: "1.2.3"},
			{Name: "bar.version", Value: "${foo.version}"},
			{Name: "with.space", Value: "text"},
		},
	}
	var project struct {
		Properties Properties `xml:"properties"`
	}
	if err := xml.Unmarshal(input, &project); err != nil {
		t.Fatalf("failed to unmarshal input: %v", err)
	}
	if diff := cmp.Diff(project.Properties, want); diff != "" {
		t.Errorf("unmarshal properties: got %v, want %v", project.Properties, want)
	}
}

func TestPropertyMap(t *testing.T) {
	proj := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "core",
			Version:    "1.0.0",
		},
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "org.parent",
				ArtifactID: "parent-pom",
				Version:    "1.1.1",
			},
			RelativePath: "../parent-pom.xml",
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "foo", Value: "abc.xyz"},
				{Name: "bar", Value: "1.2.3"},
				{Name: "version", Value: "6.6.6"},
				{Name: "parent.version", Value: "9.9.9"},
			},
		},
	}
	want := map[string]string{
		"groupId":                "com.example",
		"version":                "6.6.6", // Overwritten by a defined property
		"pom.groupId":            "com.example",
		"pom.version":            "1.0.0",
		"project.groupId":        "com.example",
		"project.version":        "1.0.0",
		"parent.groupId":         "org.parent",
		"parent.version":         "9.9.9", // Overwritten by a defined property
		"pom.parent.groupId":     "org.parent",
		"pom.parent.version":     "1.1.1",
		"project.parent.groupId": "org.parent",
		"project.parent.version": "1.1.1",
		"foo":                    "abc.xyz",
		"bar":                    "1.2.3",
	}
	got, err := proj.propertyMap()
	if err != nil {
		t.Fatalf("failed to make property map: %v", err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("property map mistmatch:\n(-got, +want):\n%s", diff)
	}
}
