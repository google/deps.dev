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

func TestProfile(t *testing.T) {
	input, err := os.ReadFile("testdata/profiles.xml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	want := []Profile{{
		ID: "my-profile-1",
		Activation: Activation{
			ActiveByDefault: "false",
			JDK:             "1.8",
			OS: ActivationOS{
				Name:    "linux",
				Family:  "unix",
				Arch:    "amd64",
				Version: "5.10.0-26-cloud-amd64",
			},
			Property: ActivationProperty{
				Name:  "debug",
				Value: "true",
			},
			File: ActivationFile{
				Missing: "/missing/file/path",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "abc.version", Value: "1.0.0"},
				{Name: "def.version", Value: "2.0.0"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.profile",
			ArtifactID: "abc",
			Version:    "${abc.version}",
		}, {
			GroupID:    "org.profile",
			ArtifactID: "def",
			Version:    "${def.version}",
		}},
	}, {
		ID: "my-profile-2",
		Activation: Activation{
			ActiveByDefault: "true",
			File: ActivationFile{
				Exists: "/exists/file/path",
			},
		},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "xyz",
				Version:    "3.0.0",
				Scope:      "import",
				Type:       "pom",
			}, {
				GroupID:    "org.dep",
				ArtifactID: "management",
				Version:    "4.0.0",
			}},
		},
		Repositories: []Repository{
			{
				ID:  "profile-repo",
				URL: "https://www.profile-repo.example.com",
				Snapshots: RepositoryPolicy{
					Enabled: "true",
				},
			},
		},
	}}
	var project struct {
		Profiles []Profile `xml:"profiles>profile"`
	}
	if err := xml.Unmarshal(input, &project); err != nil {
		t.Fatalf("failed to unmarshal input: %v", err)
	}
	if diff := cmp.Diff(project.Profiles, want); diff != "" {
		t.Errorf("unmarshal profiles: got %v, want %v", project.Profiles, want)
	}
}

func TestBuildProfileActivation(t *testing.T) {
	tests := []struct {
		prof Profile
		want bool
	}{
		{
			prof: Profile{
				Activation: Activation{
					ActiveByDefault: "true",
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					ActiveByDefault: "not-bool",
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name: "!any-name",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name: "any-name",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name:  "any-name",
						Value: "any-value",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name:  "any-name",
						Value: "!any-value",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name:  "!any-name",
						Value: "any-value",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					Property: ActivationProperty{
						Name:  "!any-name",
						Value: "!any-value",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "[1.3,1.6)",
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "[1.3,)",
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "1.3",
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "999",
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "[1.3,1.6)",
					Property: ActivationProperty{
						Name: "!any-name",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "[1.3,)",
					Property: ActivationProperty{
						Name: "!any-name",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					JDK: "[1.3,)",
					Property: ActivationProperty{
						Name: "any-name",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name:    "Windows XP",
						Family:  "Windows",
						Arch:    "x86",
						Version: "5.1.2600",
					},
				},
			},
			want: false,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name:    "Linux",
						Family:  "Unix",
						Arch:    "amd64",
						Version: "5.10.0-26-cloud-amd64",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name:   "Linux",
						Family: "Unix",
						Arch:   "amd64",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name: "Linux",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name:   "!Windows",
						Family: "Unix",
						Arch:   "!darwin",
					},
				},
			},
			want: true,
		},
		{
			prof: Profile{
				Activation: Activation{
					OS: ActivationOS{
						Name:   "Linux",
						Family: "!Unix",
					},
				},
			},
			want: false,
		},
	}
	for _, test := range tests {
		got, err := test.prof.activated(JDKProfileActivation, OSProfileActivation)
		if err != nil {
			t.Errorf("profile.activated() on %s: %v", test.prof, err)
		}
		if got != test.want {
			t.Errorf("profile.activated() on %s got: %v, want: %v", test.prof, got, test.want)
		}
	}
}

func TestMergeProfiles(t *testing.T) {
	proj := Project{
		Dependencies: []Dependency{
			{GroupID: "org.dep", ArtifactID: "xyz", Version: "1.1.1"},
		},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{
				{GroupID: "org.management", ArtifactID: "xyz", Version: "2.2.2"},
			},
		},
		Repositories: []Repository{
			{ID: "default-repo", URL: "https://www.example.com"},
		},
		Profiles: []Profile{{
			// Not activated
			Activation: Activation{
				JDK: "[1.3,1.5)",
			},
			Dependencies: []Dependency{
				{GroupID: "org.dep", ArtifactID: "not-activated", Version: "1.0.0"},
			},
			Repositories: []Repository{
				{ID: "repo-not-activated", URL: "https://www.example.com"},
			},
		}, {
			// Activated
			Activation: Activation{
				JDK: "[1.5,)",
			},
			Dependencies: []Dependency{
				{GroupID: "org.dep", ArtifactID: "abc", Version: "1.0.0"},
				{GroupID: "org.dep", ArtifactID: "def", Version: "2.0.0"},
			},
			Repositories: []Repository{
				{ID: "profile-repo-1", URL: "https://www.profile.repo-1.example.com"},
			},
		}, {
			// Activated
			Activation: Activation{
				OS: ActivationOS{
					Name:    "Linux",
					Family:  "Unix",
					Arch:    "amd64",
					Version: "5.10.0-26-cloud-amd64",
				},
			},
			DependencyManagement: DependencyManagement{
				Dependencies: []Dependency{
					{GroupID: "org.management", ArtifactID: "xxx", Version: "3.0.0"},
					{GroupID: "org.management", ArtifactID: "yyy", Version: "4.0.0"},
				},
			},
			Repositories: []Repository{
				{ID: "profile-repo-2", URL: "https://www.profile.repo-2.example.com"},
			},
		}},
	}
	proj.MergeProfiles(JDKProfileActivation, OSProfileActivation)
	want := Project{
		Dependencies: []Dependency{
			{GroupID: "org.dep", ArtifactID: "xyz", Version: "1.1.1"},
			{GroupID: "org.dep", ArtifactID: "abc", Version: "1.0.0"},
			{GroupID: "org.dep", ArtifactID: "def", Version: "2.0.0"},
		},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{
				{GroupID: "org.management", ArtifactID: "xyz", Version: "2.2.2"},
				{GroupID: "org.management", ArtifactID: "xxx", Version: "3.0.0"},
				{GroupID: "org.management", ArtifactID: "yyy", Version: "4.0.0"},
			},
		},
		Repositories: []Repository{
			{ID: "default-repo", URL: "https://www.example.com"},
			{ID: "profile-repo-1", URL: "https://www.profile.repo-1.example.com"},
			{ID: "profile-repo-2", URL: "https://www.profile.repo-2.example.com"},
		},
	}
	proj.Profiles = nil
	if diff := cmp.Diff(proj, want); diff != "" {
		t.Fatalf("mergeProfiles does not have match result:\n got %v\n want%v\n", proj, want)
	}

	// Test no activated profiles.
	proj = Project{
		Dependencies: []Dependency{
			{GroupID: "org.dep", ArtifactID: "xyz", Version: "1.1.1"},
		},
		Profiles: []Profile{{
			Activation: Activation{
				Property: ActivationProperty{
					Name:  "any-name",
					Value: "any-value",
				},
			},
			Dependencies: []Dependency{
				{GroupID: "org.activation", ArtifactID: "not-activated", Version: "1.0.0"},
			},
		}, {
			Activation: Activation{
				ActiveByDefault: "true",
			},
			Dependencies: []Dependency{
				{GroupID: "org.activation", ArtifactID: "activated", Version: "2.0.0"},
			},
		}},
	}
	proj.MergeProfiles(JDKProfileActivation, OSProfileActivation)
	want = Project{
		Dependencies: []Dependency{
			{GroupID: "org.dep", ArtifactID: "xyz", Version: "1.1.1"},
			{GroupID: "org.activation", ArtifactID: "activated", Version: "2.0.0"},
		},
	}
	proj.Profiles = nil
	if diff := cmp.Diff(proj, want); diff != "" {
		t.Fatalf("mergeProfiles does not have match result:\n got %v\n want%v\n", proj, want)
	}
}
