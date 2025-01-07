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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestProcessDependencies(t *testing.T) {
	proj := Project{
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "dep",
			Version:    "1.0.0",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "dep",
			Version:    "3.0.0",
		}, {
			GroupID:    "org.ddd",
			ArtifactID: "dep",
			Version:    "4.0.0",
		}, {
			GroupID:    "org.eee",
			ArtifactID: "dep",
			Version:    "5.0.0",
		}, {
			GroupID:    "org.aaa",
			ArtifactID: "dep",
			Version:    "5.0.0",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "import1",
				Version:    "6.0.0",
				Type:       "pom",
				Scope:      "import",
			}, {
				GroupID:    "org.fff",
				ArtifactID: "dep",
				Version:    "7.0.0",
			}},
		},
	}
	getDependencyManagement := func(groupID, artifactID, version String) (DependencyManagement, error) {
		if groupID == "org.import" && artifactID == "import1" && version == "6.0.0" {
			return DependencyManagement{
				Dependencies: []Dependency{{
					GroupID:    "org.bbb",
					ArtifactID: "dep",
					Version:    "2.0.0",
				}, {
					GroupID:    "org.ccc",
					ArtifactID: "dep",
					Version:    "3.0.0",
					Scope:      "test",
				}, {
					GroupID:    "org.ggg",
					ArtifactID: "dep",
					Version:    "8.0.0",
				}, {
					GroupID:    "org.import",
					ArtifactID: "import2",
					Version:    "9.0.0",
					Type:       "pom",
					Scope:      "import",
				}},
			}, nil
		}
		if groupID == "org.import" && artifactID == "import2" && version == "9.0.0" {
			return DependencyManagement{
				Dependencies: []Dependency{{
					GroupID:    "org.ddd",
					ArtifactID: "dep",
					Version:    "4.0.0",
					Exclusions: []Exclusion{
						{GroupID: "org.exclude", ArtifactID: "*"},
					},
				}, {
					GroupID:    "org.hhh",
					ArtifactID: "dep",
					Version:    "10.0.0",
				}},
			}, nil
		}
		return DependencyManagement{}, fmt.Errorf("cannot find project %s:%s:%s", groupID, artifactID, version)
	}
	want := Project{
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "dep",
			Version:    "1.0.0",
			Type:       "jar",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
			Type:       "jar",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "dep",
			Version:    "3.0.0",
			Type:       "jar",
			Scope:      "test",
		}, {
			GroupID:    "org.ddd",
			ArtifactID: "dep",
			Version:    "4.0.0",
			Type:       "jar",
			Exclusions: []Exclusion{
				{GroupID: "org.exclude", ArtifactID: "*"},
			},
		}, {
			GroupID:    "org.eee",
			ArtifactID: "dep",
			Version:    "5.0.0",
			Type:       "jar",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.fff",
				ArtifactID: "dep",
				Version:    "7.0.0",
				Type:       "jar",
			}, {
				GroupID:    "org.bbb",
				ArtifactID: "dep",
				Version:    "2.0.0",
				Type:       "jar",
			}, {
				GroupID:    "org.ccc",
				ArtifactID: "dep",
				Version:    "3.0.0",
				Type:       "jar",
				Scope:      "test",
			}, {
				GroupID:    "org.ggg",
				ArtifactID: "dep",
				Version:    "8.0.0",
				Type:       "jar",
			}, {
				GroupID:    "org.ddd",
				ArtifactID: "dep",
				Version:    "4.0.0",
				Type:       "jar",
				Exclusions: []Exclusion{
					{GroupID: "org.exclude", ArtifactID: "*"},
				},
			}, {
				GroupID:    "org.hhh",
				ArtifactID: "dep",
				Version:    "10.0.0",
				Type:       "jar",
			}},
		},
	}
	proj.ProcessDependencies(getDependencyManagement)
	if diff := cmp.Diff(proj, want); diff != "" {
		t.Errorf("processDependencies:\n(-got, +want):\n%s", diff)
	}
}
