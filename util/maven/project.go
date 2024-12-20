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
	"errors"
	"fmt"
	"strings"
)

type ProjectKey struct {
	GroupID    String `xml:"groupId,omitempty"`
	ArtifactID String `xml:"artifactId,omitempty"`
	Version    String `xml:"version,omitempty"`
}

func (pk ProjectKey) Name() string {
	return fmt.Sprintf("%s:%s", pk.GroupID, pk.ArtifactID)
}

func MakeProjectKey(name, version string) (ProjectKey, error) {
	group, artifact, ok := strings.Cut(name, ":")
	if !ok {
		return ProjectKey{}, errors.New("invalid Maven package name")
	}
	return ProjectKey{
		GroupID:    String(group),
		ArtifactID: String(artifact),
		Version:    String(version),
	}, nil
}

type Parent struct {
	ProjectKey
	RelativePath String `xml:"relativePath,omitempty"`
}

// Project contains information of a package version.
// https://maven.apache.org/ref/3.9.3/maven-model/maven.html
type Project struct {
	ProjectKey

	Parent      Parent `xml:"parent,omitempty"`
	Packaging   String `xml:"packaging,omitempty"`
	Name        String `xml:"name,omitempty"`
	Description String `xml:"description,omitempty"`
	URL         String `xml:"url,omitempty"`

	Properties Properties `xml:"properties,omitempty"`

	Licenses               []License              `xml:"licenses>license,omitempty"`
	Developers             []Developer            `xml:"developers>developer,omitempty"`
	SCM                    SCM                    `xml:"scm,omitempty"`
	IssueManagement        IssueManagement        `xml:"issueManagement,omitempty"`
	DistributionManagement DistributionManagement `xml:"distributionManagement,omitempty"`
	DependencyManagement   DependencyManagement   `xml:"dependencyManagement,omitempty"`
	Dependencies           []Dependency           `xml:"dependencies>dependency,omitempty"`
	Repositories           []Repository           `xml:"repositories>repository,omitempty"`
	Profiles               []Profile              `xml:"profiles>profile,omitempty"`
	Build                  Build                  `xml:"build,omitempty"`
}

type Build struct {
	PluginManagement PluginManagement `xml:"pluginManagement,omitempty"`
}

func (b *Build) interpolate(properties map[string]string) bool {
	return b.PluginManagement.interpolate(properties)
}

func (b *Build) merge(parent Build) {
	b.PluginManagement.merge(parent.PluginManagement)
}

type PluginManagement struct {
	Plugins []Plugin `xml:"plugins>plugin,omitempty"`
}

func (p *PluginManagement) interpolate(properties map[string]string) bool {
	var plugins []Plugin
	for _, plugin := range p.Plugins {
		if plugin.interpolate(properties) {
			plugins = append(plugins, plugin)
		}
	}
	p.Plugins = plugins
	// Only interpolated plugins are appended.
	return true
}

func (p *PluginManagement) merge(parent PluginManagement) {
	p.Plugins = append(p.Plugins, parent.Plugins...)
}

type Plugin struct {
	ProjectKey
	Inherited    TruthyBool   `xml:"inherited,omitempty"`
	Dependencies []Dependency `xml:"dependencies>dependency,omitempty"`
}

func (p *Plugin) interpolate(properties map[string]string) bool {
	var deps []Dependency
	for _, dep := range p.Dependencies {
		if dep.interpolate(properties) {
			deps = append(deps, dep)
		}
	}
	p.Dependencies = deps
	return p.GroupID.interpolate(properties) && p.ArtifactID.interpolate(properties) && p.Version.interpolate(properties) && p.Inherited.interpolate(properties)
}

type License struct {
	Name String `xml:"name,omitempty"`
}

func (l *License) interpolate(properties map[string]string) bool {
	return l.Name.interpolate(properties)
}

type Developer struct {
	Name  String `xml:"name,omitempty"`
	Email String `xml:"email,omitempty"`
}

func (d *Developer) interpolate(properties map[string]string) bool {
	ok1 := d.Name.interpolate(properties)
	ok2 := d.Email.interpolate(properties)
	return ok1 && ok2
}

type SCM struct {
	Tag String `xml:"tag,omitempty"`
	URL String `xml:"url,omitempty"`
}

func (s *SCM) merge(parent SCM) {
	if s.Tag == "" && s.URL == "" {
		*s = parent
	}
}

func (s *SCM) interpolate(properties map[string]string) bool {
	ok1 := s.Tag.interpolate(properties)
	ok2 := s.URL.interpolate(properties)
	return ok1 && ok2
}

type IssueManagement struct {
	System String `xml:"system,omitempty"`
	URL    String `xml:"url,omitempty"`
}

func (im *IssueManagement) merge(parent IssueManagement) {
	if im.System == "" && im.URL == "" {
		*im = parent
	}
}

func (im *IssueManagement) interpolate(properties map[string]string) bool {
	ok1 := im.System.interpolate(properties)
	ok2 := im.URL.interpolate(properties)
	return ok1 && ok2
}

type DistributionManagement struct {
	Relocation Relocation `xml:"relocation,omitempty"`
}

func (dm *DistributionManagement) interpolate(properties map[string]string) bool {
	return dm.Relocation.interpolate(properties)
}

type Relocation struct {
	GroupID    String `xml:"groupId,omitempty"`
	ArtifactID String `xml:"artifactId,omitempty"`
	Version    String `xml:"version,omitempty"`
}

func (r *Relocation) interpolate(properties map[string]string) bool {
	ok1 := r.GroupID.interpolate(properties)
	ok2 := r.ArtifactID.interpolate(properties)
	ok3 := r.Version.interpolate(properties)
	return ok1 && ok2 && ok3
}

// Repository contains the information about a remote repository.
// https://maven.apache.org/ref/3.9.3/maven-model/maven.html#repository-1
type Repository struct {
	ID        String           `xml:"id,omitempty"`
	URL       String           `xml:"url,omitempty"`
	Layout    String           `xml:"layout,omitempty"`
	Releases  RepositoryPolicy `xml:"releases,omitempty"`
	Snapshots RepositoryPolicy `xml:"snapshots,omitempty"`
}

func (r *Repository) interpolate(properties map[string]string) bool {
	ok1 := r.ID.interpolate(properties)
	ok2 := r.URL.interpolate(properties)
	ok3 := r.Layout.interpolate(properties)
	ok4 := r.Releases.interpolate(properties)
	ok5 := r.Snapshots.interpolate(properties)
	return ok1 && ok2 && ok3 && ok4 && ok5
}

type RepositoryPolicy struct {
	Enabled TruthyBool `xml:"enabled"`
}

func (rp *RepositoryPolicy) interpolate(properties map[string]string) bool {
	return rp.Enabled.interpolate(properties)
}

// MergeParent merges data from the parent project.
// https://maven.apache.org/guides/introduction/introduction-to-the-pom.html#Project_Inheritance
func (p *Project) MergeParent(parent Project) {
	p.GroupID.merge(parent.GroupID)
	p.Version.merge(parent.Version)
	p.Description.merge(parent.Description)
	p.URL.merge(parent.URL)
	if len(p.Licenses) == 0 {
		p.Licenses = parent.Licenses
	}
	if len(p.Developers) == 0 {
		p.Developers = parent.Developers
	}
	p.SCM.merge(parent.SCM)
	p.IssueManagement.merge(parent.IssueManagement)
	p.Properties.merge(parent.Properties)
	p.DependencyManagement.merge(parent.DependencyManagement)
	p.Build.merge(parent.Build)
	p.Dependencies = append(p.Dependencies, parent.Dependencies...)
	p.Repositories = append(p.Repositories, parent.Repositories...)
}

// Interpolate resolves placeholders in Project if there exists.
// Metadata is only recorded if it is successfully resolved.
func (p *Project) Interpolate() error {
	properties, err := p.propertyMap()
	if err != nil {
		return err
	}

	p.Packaging.interpolate(properties)
	p.SCM.interpolate(properties)
	p.IssueManagement.interpolate(properties)
	p.DistributionManagement.interpolate(properties)
	p.Build.interpolate(properties)

	var licenses []License
	for _, l := range p.Licenses {
		if ok := l.interpolate(properties); ok {
			licenses = append(licenses, l)
		}
	}
	p.Licenses = licenses

	var developers []Developer
	for _, d := range p.Developers {
		if ok := d.interpolate(properties); ok {
			developers = append(developers, d)
		}
	}
	p.Developers = developers

	var deps []Dependency
	for _, dep := range p.Dependencies {
		if dep.GroupID == "" || dep.ArtifactID == "" {
			continue
		}
		if dep.interpolate(properties) {
			deps = append(deps, dep)
		}
	}
	p.Dependencies = deps

	deps = []Dependency{}
	for _, dm := range p.DependencyManagement.Dependencies {
		if dm.GroupID == "" || dm.ArtifactID == "" {
			continue
		}
		if dm.interpolate(properties) {
			deps = append(deps, dm)
		}
	}
	p.DependencyManagement = DependencyManagement{Dependencies: deps}

	var repos []Repository
	for _, r := range p.Repositories {
		if ok := r.interpolate(properties); ok {
			repos = append(repos, r)
		}
	}
	p.Repositories = repos

	return nil
}
