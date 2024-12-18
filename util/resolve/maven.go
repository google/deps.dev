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
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "deps.dev/api/v3"
	"deps.dev/util/maven"
	"deps.dev/util/resolve/dep"
)

func MavenDepType(d maven.Dependency, origin string) dep.Type {
	var dt dep.Type
	if d.Optional == "true" {
		dt.AddAttr(dep.Opt, "")
	}
	if d.Scope == "test" {
		dt.AddAttr(dep.Test, "")
	} else if d.Scope != "" && d.Scope != "compile" {
		dt.AddAttr(dep.Scope, string(d.Scope))
	}
	if d.Type != "" && d.Type != "jar" {
		dt.AddAttr(dep.MavenArtifactType, string(d.Type))
	}
	if d.Classifier != "" {
		dt.AddAttr(dep.MavenClassifier, string(d.Classifier))
	}
	if len(d.Exclusions) > 0 {
		dt.AddAttr(dep.MavenExclusions, d.ExclusionsString())
	}
	// Only add Maven dependency origin when it is not direct dependency.
	if origin != "" {
		dt.AddAttr(dep.MavenDependencyOrigin, origin)
	}
	return dt
}

func MavenDepTypeToDependency(typ dep.Type) (maven.Dependency, string, error) {
	result := maven.Dependency{}
	if _, ok := typ.GetAttr(dep.Opt); ok {
		result.Optional = "true"
	}
	if _, ok := typ.GetAttr(dep.Test); ok {
		result.Scope = "test"
	}
	if s, ok := typ.GetAttr(dep.Scope); ok {
		if result.Scope != "" {
			return maven.Dependency{}, "", errors.New("invalid Maven dep.Type")
		}
		result.Scope = maven.String(s)
	}
	if c, ok := typ.GetAttr(dep.MavenClassifier); ok {
		result.Classifier = maven.String(c)
	}
	if t, ok := typ.GetAttr(dep.MavenArtifactType); ok {
		result.Type = maven.String(t)
	}
	if e, ok := typ.GetAttr(dep.MavenExclusions); ok {
		exs := strings.Split(e, "|")
		for _, ex := range exs {
			i := strings.Index(ex, ":")
			result.Exclusions = append(result.Exclusions, maven.Exclusion{
				GroupID:    maven.String(ex[:i]),
				ArtifactID: maven.String(ex[i+1:]),
			})
		}

	}
	if o, ok := typ.GetAttr(dep.MavenDependencyOrigin); ok {
		return result, o, nil
	}
	return result, "", nil
}

const MaxMavenParent = 100

func (a *APIClient) fetchMavenParents(ctx context.Context, current maven.ProjectKey, project *maven.Project) error {
	visited := make(map[maven.ProjectKey]bool, MaxMavenParent)
	for n := 0; n < MaxMavenParent; n++ {
		if current.GroupID == "" || current.ArtifactID == "" || current.Version == "" {
			break
		}
		if visited[current] {
			// A cycle of parents is detected.
			return errors.New("a cycle of Maven parents is detected")
		}
		visited[current] = true

		resp, err := a.c.GetRequirements(ctx, &pb.GetRequirementsRequest{
			VersionKey: &pb.VersionKey{
				System:  pb.System(Maven),
				Name:    current.Name(),
				Version: string(current.Version),
			},
		})
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("requirements %v: %w", current, ErrNotFound)
		}
		if err != nil {
			return err
		}

		proj := mavenRequirementsToProject(current, resp.Maven)
		// Only merge default profiles by passing empty JDK and OS information.
		if err := proj.MergeProfiles("", maven.ActivationOS{}); err != nil {
			return err
		}
		project.MergeParent(proj)
		current = proj.Parent.ProjectKey
	}
	return project.Interpolate()
}

func (a *APIClient) mavenRequirements(ctx context.Context, vk VersionKey, reqs *pb.Requirements_Maven) ([]RequirementVersion, error) {
	projKey, err := maven.MakeProjectKey(vk.Name, vk.Version)
	if err != nil {
		return nil, err
	}
	project := mavenRequirementsToProject(projKey, reqs)
	// Only merge default profiles by passing empty JDK and OS information.
	if err := project.MergeProfiles("", maven.ActivationOS{}); err != nil {
		return nil, err
	}
	if err := a.fetchMavenParents(ctx, project.Parent.ProjectKey, &project); err != nil {
		return nil, err
	}
	project.ProcessDependencies(func(group, artifact, v maven.String) (maven.DependencyManagement, error) {
		pk := maven.ProjectKey{
			GroupID:    group,
			ArtifactID: artifact,
			Version:    v,
		}
		result := maven.Project{ProjectKey: pk}
		if err := a.fetchMavenParents(ctx, pk, &result); err != nil {
			return maven.DependencyManagement{}, err
		}
		return result.DependencyManagement, nil
	})

	var result []RequirementVersion
	for _, d := range project.Dependencies {
		result = append(result, RequirementVersion{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: Maven,
					Name:   string(d.GroupID + ":" + d.ArtifactID),
				},
				VersionType: Requirement,
				Version:     string(d.Version),
			},
			Type: MavenDepType(d, ""),
		})
	}
	return result, nil
}

func mavenRequirementsToProject(pk maven.ProjectKey, req *pb.Requirements_Maven) maven.Project {
	if req == nil {
		return maven.Project{}
	}

	getDependencies := func(deps []*pb.Requirements_Maven_Dependency) []maven.Dependency {
		var result []maven.Dependency
		for _, d := range deps {
			var exs []maven.Exclusion
			for _, ex := range d.Exclusions {
				exKey, err := maven.MakeProjectKey(ex, "")
				if err != nil {
					continue
				}
				exs = append(exs, maven.Exclusion{GroupID: exKey.GroupID, ArtifactID: exKey.ArtifactID})
			}

			dk, err := maven.MakeProjectKey(d.Name, "")
			if err != nil {
				continue
			}
			result = append(result, maven.Dependency{
				GroupID:    dk.GroupID,
				ArtifactID: dk.ArtifactID,
				Version:    maven.String(d.Version),
				Type:       maven.String(d.Type),
				Classifier: maven.String(d.Classifier),
				Scope:      maven.String(d.Scope),
				Optional:   maven.FalsyBool(d.Optional),
				Exclusions: exs,
			})
		}
		return result
	}

	getProperties := func(props []*pb.Requirements_Maven_Property) []maven.Property {
		var result []maven.Property
		for _, prop := range props {
			result = append(result, maven.Property{
				Name:  prop.Name,
				Value: prop.Value,
			})
		}
		return result
	}

	getRepositories := func(repos []*pb.Requirements_Maven_Repository) []maven.Repository {
		var result []maven.Repository
		for _, r := range repos {
			result = append(result, maven.Repository{
				ID:        maven.String(r.Id),
				URL:       maven.String(r.Url),
				Layout:    maven.String(r.Layout),
				Releases:  maven.RepositoryPolicy{Enabled: maven.TruthyBool(r.ReleasesEnabled)},
				Snapshots: maven.RepositoryPolicy{Enabled: maven.TruthyBool(r.SnapshotsEnabled)},
			})
		}
		return result
	}

	var profiles []maven.Profile
	for _, p := range req.Profiles {
		activation := maven.Activation{
			ActiveByDefault: maven.FalsyBool(p.Activation.ActiveByDefault),
		}
		if p.Activation.Jdk != nil {
			activation.JDK = maven.String(p.Activation.Jdk.Jdk)
		}
		if p.Activation.Os != nil {
			activation.OS = maven.ActivationOS{
				Name:    maven.String(p.Activation.Os.Name),
				Family:  maven.String(p.Activation.Os.Family),
				Arch:    maven.String(p.Activation.Os.Arch),
				Version: maven.String(p.Activation.Os.Version),
			}
		}
		if p.Activation.Property != nil {
			activation.Property = maven.ActivationProperty{
				Name:  maven.String(p.Activation.Property.Property.Name),
				Value: maven.String(p.Activation.Property.Property.Value),
			}
		}
		if p.Activation.File != nil {
			activation.File = maven.ActivationFile{
				Missing: maven.String(p.Activation.File.Missing),
				Exists:  maven.String(p.Activation.File.Exists),
			}
		}
		profiles = append(profiles, maven.Profile{
			ID:                   maven.String(p.Id),
			Activation:           activation,
			Properties:           maven.Properties{Properties: getProperties(p.Properties)},
			Dependencies:         getDependencies(p.Dependencies),
			DependencyManagement: maven.DependencyManagement{Dependencies: getDependencies(p.DependencyManagement)},
			Repositories:         getRepositories(p.Repositories),
		})
	}

	var parent maven.Parent
	if req.Parent != nil {
		parent.ProjectKey, _ = maven.MakeProjectKey(req.Parent.Name, req.Parent.Version)

	}

	return maven.Project{
		ProjectKey:           pk,
		Parent:               parent,
		Dependencies:         getDependencies(req.Dependencies),
		DependencyManagement: maven.DependencyManagement{Dependencies: getDependencies(req.DependencyManagement)},
		Properties:           maven.Properties{Properties: getProperties(req.Properties)},
		Repositories:         getRepositories(req.Repositories),
		Profiles:             profiles,
	}
}
