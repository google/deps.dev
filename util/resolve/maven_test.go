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

package resolve

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	pb "deps.dev/api/v3"
	"deps.dev/util/maven"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/version"
)

type fakeInsightsClient struct {
	pb.InsightsClient
}

func (c *fakeInsightsClient) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.Version, error) {
	if vk := in.VersionKey; vk.System == pb.System_MAVEN && vk.Name == "org.version:aaa" && vk.Version == "1.1.1" {
		return &pb.Version{
			Registries: []string{"https://repo.maven.apache.org/maven2"},
		}, nil
	}
	if vk := in.VersionKey; vk.System == pb.System_MAVEN && vk.Name == "org.version:bbb" && vk.Version == "2.2.2" {
		return &pb.Version{
			Registries: []string{"https://www.another-registry.com"},
		}, nil
	}
	return nil, ErrNotFound
}

func (c *fakeInsightsClient) GetRequirements(ctx context.Context, in *pb.GetRequirementsRequest, opts ...grpc.CallOption) (*pb.Requirements, error) {
	vk := in.VersionKey
	if vk.System == pb.System_MAVEN && vk.Name == "org.version:aaa" && vk.Version == "1.1.1" {
		return &pb.Requirements{
			Maven: &pb.Requirements_Maven{
				Repositories: []*pb.Requirements_Maven_Repository{
					{Id: "my-repo", Url: "https://www.my-repo.example.com"},
				},
			},
		}, nil
	}
	if vk.System == pb.System_MAVEN && vk.Name == "org.version:bbb" && vk.Version == "2.2.2" {
		return &pb.Requirements{}, nil
	}
	if vk.System == pb.System_MAVEN && vk.Name == "org.parent:parent-pom" && vk.Version == "1.2.3" {
		return &pb.Requirements{
			Maven: &pb.Requirements_Maven{
				Dependencies: []*pb.Requirements_Maven_Dependency{
					{Name: "org.dependency:ggg", Version: "7.0.0"},
				},
				DependencyManagement: []*pb.Requirements_Maven_Dependency{
					{Name: "org.dependency:hhh", Version: "8.0.0"},
				},
			},
		}, nil
	}
	if vk.System == pb.System_MAVEN && vk.Name == "org.dependency:eee" && vk.Version == "5.0.0" {
		return &pb.Requirements{
			Maven: &pb.Requirements_Maven{
				DependencyManagement: []*pb.Requirements_Maven_Dependency{
					{Name: "org.import:aaa", Version: "9.9.9"},
					{Name: "org.import:bbb", Version: "8.8.8"},
					{Name: "org.import:ccc", Version: "7.7.7"},
				},
			},
		}, nil
	}
	return nil, ErrNotFound
}

func TestMavenVersion(t *testing.T) {
	ctx := context.Background()
	client := APIClient{
		c: &fakeInsightsClient{},
	}

	var attr1 version.AttrSet
	attr1.SetAttr(version.Registries, "https://repo.maven.apache.org/maven2|dep:https://www.my-repo.example.com")
	var attr2 version.AttrSet
	attr2.SetAttr(version.Registries, "https://www.another-registry.com")

	for _, test := range []struct {
		name, version string
		attr          version.AttrSet
	}{
		{
			name:    "org.version:aaa",
			version: "1.1.1",
			attr:    attr1,
		},
		{
			name:    "org.version:bbb",
			version: "2.2.2",
			attr:    attr2,
		},
	} {
		vk := VersionKey{
			PackageKey: PackageKey{
				System: Maven,
				Name:   test.name,
			},
			VersionType: Concrete,
			Version:     test.version,
		}

		got, err := client.Version(ctx, vk)
		if err != nil {
			t.Fatalf("Version(%v): %v ", vk, err)
		}

		want := Version{
			VersionKey: vk,
			AttrSet:    test.attr,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Version(%v):\ngot: %v\nwant: %v\n", vk, got, want)
		}
	}
}

func TestMavenRequirements(t *testing.T) {
	client := APIClient{
		c: &fakeInsightsClient{},
	}

	ctx := context.Background()
	vk := VersionKey{
		PackageKey: PackageKey{
			System: Maven,
			Name:   "abc:xyz",
		},
		Version: "3.2.1",
	}
	reqs := &pb.Requirements_Maven{
		Parent: &pb.VersionKey{
			System:  pb.System_MAVEN,
			Name:    "org.parent:parent-pom",
			Version: "1.2.3",
		},
		Dependencies: []*pb.Requirements_Maven_Dependency{
			{Name: "org.dependency:aaa", Version: "1.0.0"},
			{Name: "org.dependency:bbb", Version: "[2.0.0,)"},
			{Name: "org.dependency:ccc", Version: "${ccc.version}"},
			{Name: "org.import:aaa"},
		},
		DependencyManagement: []*pb.Requirements_Maven_Dependency{
			{Name: "org.dependency:ddd", Version: "4.0.0"},
			{Name: "org.dependency:eee", Version: "5.0.0", Type: "pom", Scope: "import"},
		},
		Properties: []*pb.Requirements_Maven_Property{
			{Name: "ccc.version", Value: "3.0.0"},
		},
		Profiles: []*pb.Requirements_Maven_Profile{
			{
				Id: "profile-one",
				Activation: &pb.Requirements_Maven_Profile_Activation{
					ActiveByDefault: "true",
				},
				Dependencies: []*pb.Requirements_Maven_Dependency{
					{Name: "org.dependency:fff", Version: "${fff.version}"},
				},
				Properties: []*pb.Requirements_Maven_Property{
					{Name: "fff.version", Value: "6.0.0"},
				},
			},
		},
	}
	got, err := client.mavenRequirements(ctx, vk, reqs)
	if err != nil {
		t.Fatalf("mavenRequirements: %v", err)
	}
	want := []RequirementVersion{
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.dependency:aaa",
				},
				VersionType: Requirement,
				Version:     "1.0.0",
			},
		},
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.dependency:bbb",
				},
				VersionType: Requirement,
				Version:     "[2.0.0,)",
			},
		},
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.dependency:ccc",
				},
				VersionType: Requirement,
				Version:     "3.0.0",
			},
		},
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.import:aaa",
				},
				VersionType: Requirement,
				Version:     "9.9.9",
			},
		},
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.dependency:fff",
				},
				VersionType: Requirement,
				Version:     "6.0.0",
			},
		},
		{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   "org.dependency:ggg",
				},
				VersionType: Requirement,
				Version:     "7.0.0",
			},
		},
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("mavenRequirements:\n(- want, + got):\n%s", d)
	}
}

func TestDepType(t *testing.T) {
	// Test the conversion from Dependency to dep.Type
	var type1 dep.Type
	type1.AddAttr(dep.Scope, "import")
	var type2 dep.Type
	type2.AddAttr(dep.MavenArtifactType, "pom")
	type2.AddAttr(dep.MavenExclusions, "org.example:abc|org.exclude:*")

	tests := []struct {
		typ, scope maven.String
		optional   maven.FalsyBool
		exclusions []maven.Exclusion
		want       dep.Type
	}{
		{"jar", "test", "true", nil, dep.NewType(dep.Test, dep.Opt)},
		{"", "import", "", nil, type1},
		{"pom", "compile", "", []maven.Exclusion{
			{GroupID: "org.example", ArtifactID: "abc"},
			{GroupID: "org.exclude", ArtifactID: "*"},
		}, type2},
	}

	for _, test := range tests {
		d := maven.Dependency{
			GroupID:    "org.example",
			ArtifactID: "xyz",
			Version:    "1.2.3",
			Type:       test.typ,
			Scope:      test.scope,
			Optional:   test.optional,
			Exclusions: test.exclusions,
		}
		got := MavenDepType(d, "")
		if !got.Equal(test.want) {
			t.Errorf("MavenDepType: got %v, want %v", got, test.want)
		}
	}

	// Test the conversion from dep.Type to Dependency
	var type3 dep.Type
	type3.AddAttr(dep.Scope, "provided")
	type4 := dep.NewType(dep.Opt, dep.Test)
	type4.AddAttr(dep.MavenExclusions, "org.exclude:dep-one|org.exclude:dep-two")
	type4.AddAttr(dep.MavenDependencyOrigin, "management")

	moreTests := []struct {
		typ        dep.Type
		wantDep    maven.Dependency
		wantOrigin string
	}{
		{type3, maven.Dependency{Scope: "provided"}, ""},
		{type4, maven.Dependency{
			Optional: "true",
			Scope:    "test",
			Exclusions: []maven.Exclusion{
				{GroupID: "org.exclude", ArtifactID: "dep-one"},
				{GroupID: "org.exclude", ArtifactID: "dep-two"},
			}}, "management"},
	}

	for _, test := range moreTests {
		got, o, err := MavenDepTypeToDependency(test.typ)
		if err != nil {
			t.Errorf("MavenDepTypeToDependency: %v", err)
		}
		if diff := cmp.Diff(got, test.wantDep); diff != "" || o != test.wantOrigin {
			t.Errorf("MavenDepTypeToDependency: got %v %s, want %v %s", got, o, test.wantDep, test.wantOrigin)
		}
	}
}
