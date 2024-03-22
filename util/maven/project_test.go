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

func TestProject(t *testing.T) {
	input, err := os.ReadFile("testdata/basic-1.2.3.xml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	want := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "com.example",
				ArtifactID: "parent-pom",
				Version:    "1.2.3",
			},
			RelativePath: "../parent-pom.xml",
		},
		Packaging:   "war",
		Name:        "Basic",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "MIT"},
			{Name: "Apache 2.0"},
		},
		Developers: []Developer{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/basic",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/basic/issues",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "com.example",
				ArtifactID: "relocation",
				Version:    "1.2.3",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "name", Value: "value"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
			Type:       "pom",
			Classifier: "sources",
			Exclusions: []Exclusion{
				{GroupID: "org.exclude", ArtifactID: "exclusion"},
				{GroupID: "org.zzz", ArtifactID: "*"},
			},
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "test",
			Version:    "3.0.0",
			Scope:      "test",
			Optional:   "true",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "dep",
				Version:    "1.0.0",
				Type:       "pom",
				Scope:      "import",
			}, {
				GroupID:    "org.ddd",
				ArtifactID: "dep-management",
				Version:    "2.0.0",
			}},
		},
		Repositories: []Repository{{
			ID:  "my-repo",
			URL: "https://www.my-repo.example.com",
			Releases: RepositoryPolicy{
				Enabled: "true",
			},
			Snapshots: RepositoryPolicy{
				Enabled: "false",
			},
		}, {
			ID:  "another-repo",
			URL: "https://www.another-repo.example.com",
		}},
		Profiles: []Profile{{
			ID: "my-profile-1",
			Activation: Activation{
				ActiveByDefault: "false",
				JDK:             "1.8",
				OS: ActivationOS{
					Name:    "linux",
					Family:  "unix",
					Arch:    "amd64",
					Version: "5.7.17-1rodete4-amd64",
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
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin", Version: "1.2.3",
						},
						Inherited: "false",
						Dependencies: []Dependency{
							{GroupID: "org.plugin", ArtifactID: "dep", Version: "1.0.0"},
						},
					},
				},
			},
		},
	}
	var got Project
	if err := xml.Unmarshal(input, &got); err != nil {
		t.Fatalf("failed to unmarshal input: %v", err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unmarshal input: got %v\n, want %v", got, want)
	}
}

func TestMergeParent(t *testing.T) {
	parent := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "parent-pom",
			Version:    "1.2.3",
		},
		Packaging:   "pom",
		Name:        "Parent POM",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "MIT"},
			{Name: "Apache 2.0"},
		},
		Developers: []Developer{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/parent",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/parent/issues",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "com.example",
				ArtifactID: "parent-relocation",
				Version:    "1.2.3",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "name", Value: "value"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "dep",
			Version:    "3.0.0",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "dep",
				Version:    "1.0.0",
				Type:       "pom",
				Scope:      "import",
			}, {
				GroupID:    "org.ddd",
				ArtifactID: "dep-management",
				Version:    "2.0.0",
			}},
		},
		Repositories: []Repository{{
			ID:  "repo-1",
			URL: "https://www.repo-1.example.com",
		}, {
			ID:  "repo-2",
			URL: "https://www.repo-2.example.com",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin-in-parent", Version: "1.2.3",
						},
					},
				},
			},
		},
	}

	proj1 := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "com.example",
				ArtifactID: "parent-pom",
				Version:    "1.2.3",
			},
			RelativePath: "../parent-pom.xml",
		},
	}
	want1 := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "com.example",
				ArtifactID: "parent-pom",
				Version:    "1.2.3",
			},
			RelativePath: "../parent-pom.xml",
		},
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "MIT"},
			{Name: "Apache 2.0"},
		},
		Developers: []Developer{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/parent",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/parent/issues",
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "name", Value: "value"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "dep",
			Version:    "3.0.0",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "dep",
				Version:    "1.0.0",
				Type:       "pom",
				Scope:      "import",
			}, {
				GroupID:    "org.ddd",
				ArtifactID: "dep-management",
				Version:    "2.0.0",
			}},
		},
		Repositories: []Repository{{
			ID:  "repo-1",
			URL: "https://www.repo-1.example.com",
		}, {
			ID:  "repo-2",
			URL: "https://www.repo-2.example.com",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin-in-parent", Version: "1.2.3",
						},
					},
				},
			},
		},
	}
	proj1.MergeParent(parent)
	if diff := cmp.Diff(proj1, want1); diff != "" {
		t.Errorf("mergeParent: got %v\n, want %v", proj1, want1)
	}

	proj2 := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "com.example",
				ArtifactID: "parent-pom",
				Version:    "1.2.3",
			},
			RelativePath: "../parent-pom.xml",
		},
		Packaging:   "war",
		Name:        "Basic",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "Apache 2.0"},
			{Name: "BSD License"},
		},
		Developers: []Developer{
			{Name: "David", Email: "david@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/basic",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/basic/issues",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "com.example",
				ArtifactID: "relocation",
				Version:    "1.2.3",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "bar", Value: "bar-value"},
				{Name: "foo", Value: "foo-value"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.xxx",
			ArtifactID: "dep",
			Version:    "1.1.1",
		}, {
			GroupID:    "org.yyy",
			ArtifactID: "dep",
			Version:    "2.2.2",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.zzz",
				ArtifactID: "dep",
				Version:    "3.3.3",
			}},
		},
		Repositories: []Repository{{
			ID:  "repo-3",
			URL: "https://www.repo-3.example.com",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin", Version: "1.1.1",
						},
						Dependencies: []Dependency{
							{GroupID: "org.plugin", ArtifactID: "dep", Version: "1.0.0"},
						},
					},
				},
			},
		},
	}
	want2 := Project{
		Parent: Parent{
			ProjectKey: ProjectKey{
				GroupID:    "com.example",
				ArtifactID: "parent-pom",
				Version:    "1.2.3",
			},
			RelativePath: "../parent-pom.xml",
		},
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Packaging:   "war",
		Name:        "Basic",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "Apache 2.0"},
			{Name: "BSD License"},
		},
		Developers: []Developer{
			{Name: "David", Email: "david@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/basic",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/basic/issues",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "com.example",
				ArtifactID: "relocation",
				Version:    "1.2.3",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "name", Value: "value"},
				{Name: "bar", Value: "bar-value"},
				{Name: "foo", Value: "foo-value"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.xxx",
			ArtifactID: "dep",
			Version:    "1.1.1",
		}, {
			GroupID:    "org.yyy",
			ArtifactID: "dep",
			Version:    "2.2.2",
		}, {
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
		}, {
			GroupID:    "org.ccc",
			ArtifactID: "dep",
			Version:    "3.0.0",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.zzz",
				ArtifactID: "dep",
				Version:    "3.3.3",
			}, {
				GroupID:    "org.import",
				ArtifactID: "dep",
				Version:    "1.0.0",
				Type:       "pom",
				Scope:      "import",
			}, {
				GroupID:    "org.ddd",
				ArtifactID: "dep-management",
				Version:    "2.0.0",
			}},
		},
		Repositories: []Repository{{
			ID:  "repo-3",
			URL: "https://www.repo-3.example.com",
		}, {
			ID:  "repo-1",
			URL: "https://www.repo-1.example.com",
		}, {
			ID:  "repo-2",
			URL: "https://www.repo-2.example.com",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin", Version: "1.1.1",
						},
						Dependencies: []Dependency{
							{GroupID: "org.plugin", ArtifactID: "dep", Version: "1.0.0"},
						},
					},
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin-in-parent", Version: "1.2.3",
						},
					},
				},
			},
		},
	}
	proj2.MergeParent(parent)
	if diff := cmp.Diff(proj2, want2); diff != "" {
		t.Errorf("mergeParent: got %v\n, want %v", proj2, want2)
	}
}

func TestInterpolate(t *testing.T) {
	proj := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Packaging:   "${packaging}",
		Name:        "Basic",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "${license}"},
		},
		Developers: []Developer{
			{Name: "${dev.name}", Email: "${dev.email}"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "${scm.url}",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "${issue.url}",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "${relocation.groupId}",
				ArtifactID: "${relocation.artifactId}",
				Version:    "${relocation.version}",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "packaging", Value: "ear"},
				{Name: "license", Value: "Apache 2.0"},
				{Name: "dev.name", Value: "Alice"},
				{Name: "dev.email", Value: "alice@example.com"},
				{Name: "scm.url", Value: "https://git.example.com/example/basic"},
				{Name: "issue.url", Value: "https://git.example.com/example/basic/issues"},
				{Name: "relocation.groupId", Value: "${pom.groupId}"},
				{Name: "relocation.artifactId", Value: "relocation"},
				{Name: "relocation.version", Value: "${project.version}"},
				{Name: "core.optional", Value: "true"},
				{Name: "dep.version", Value: "2.0.0"},
				{Name: "import.version", Value: "3.0.0"},
				{Name: "repo.url", Value: "https://www.my-repo.example.com"},
				{Name: "plugin.version", Value: "1.2.3"},
				{Name: "plugin.dependency.version", Value: "1.0.0"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
			Optional:   "${core.optional}",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "${dep.version}",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{
				{
					GroupID:    "org.import",
					ArtifactID: "dep",
					Version:    "${import.version}",
					Type:       "pom",
					Scope:      "import",
				},
			},
		},
		Repositories: []Repository{{
			ID:  "my-repo",
			URL: "${repo.url}",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin", Version: "${plugin.version}",
						},
						Dependencies: []Dependency{
							{GroupID: "org.plugins", ArtifactID: "dep", Version: "${plugin.dependency.version}"},
						},
					},
				},
			},
		},
	}
	want := Project{
		ProjectKey: ProjectKey{
			GroupID:    "com.example",
			ArtifactID: "basic",
			Version:    "1.2.3",
		},
		Packaging:   "ear",
		Name:        "Basic",
		Description: "A fantastic package!",
		URL:         "https://www.example.com",
		Licenses: []License{
			{Name: "Apache 2.0"},
		},
		Developers: []Developer{
			{Name: "Alice", Email: "alice@example.com"},
		},
		SCM: SCM{
			Tag: "r1.2.3",
			URL: "https://git.example.com/example/basic",
		},
		IssueManagement: IssueManagement{
			System: "github",
			URL:    "https://git.example.com/example/basic/issues",
		},
		DistributionManagement: DistributionManagement{
			Relocation: Relocation{
				GroupID:    "com.example",
				ArtifactID: "relocation",
				Version:    "1.2.3",
			},
		},
		Properties: Properties{
			Properties: []Property{
				{Name: "packaging", Value: "ear"},
				{Name: "license", Value: "Apache 2.0"},
				{Name: "dev.name", Value: "Alice"},
				{Name: "dev.email", Value: "alice@example.com"},
				{Name: "scm.url", Value: "https://git.example.com/example/basic"},
				{Name: "issue.url", Value: "https://git.example.com/example/basic/issues"},
				{Name: "relocation.groupId", Value: "${pom.groupId}"},
				{Name: "relocation.artifactId", Value: "relocation"},
				{Name: "relocation.version", Value: "${project.version}"},
				{Name: "core.optional", Value: "true"},
				{Name: "dep.version", Value: "2.0.0"},
				{Name: "import.version", Value: "3.0.0"},
				{Name: "repo.url", Value: "https://www.my-repo.example.com"},
				{Name: "plugin.version", Value: "1.2.3"},
				{Name: "plugin.dependency.version", Value: "1.0.0"},
			},
		},
		Dependencies: []Dependency{{
			GroupID:    "org.aaa",
			ArtifactID: "core",
			Version:    "1.0.0",
			Optional:   "true",
		}, {
			GroupID:    "org.bbb",
			ArtifactID: "dep",
			Version:    "2.0.0",
		}},
		DependencyManagement: DependencyManagement{
			Dependencies: []Dependency{{
				GroupID:    "org.import",
				ArtifactID: "dep",
				Version:    "3.0.0",
				Type:       "pom",
				Scope:      "import",
			}},
		},
		Repositories: []Repository{{
			ID:  "my-repo",
			URL: "https://www.my-repo.example.com",
		}},
		Build: Build{
			PluginManagement: PluginManagement{
				Plugins: []Plugin{
					{
						ProjectKey: ProjectKey{
							GroupID: "org.apache.maven.plugins", ArtifactID: "plugin", Version: "1.2.3",
						},
						Dependencies: []Dependency{
							{GroupID: "org.plugins", ArtifactID: "dep", Version: "1.0.0"},
						},
					},
				},
			},
		},
	}
	proj.Interpolate()
	if diff := cmp.Diff(proj, want); diff != "" {
		t.Errorf("interpolate: got %v\n, want %v", proj, want)
	}
}
