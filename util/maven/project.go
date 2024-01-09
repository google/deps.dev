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

type ProjectKey struct {
	GroupID    String `xml:"groupId,omitempty"`
	ArtifactID String `xml:"artifactId,omitempty"`
	Version    String `xml:"version,omitempty"`
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
	Inherited    BoolString   `xml:"inherited,omitempty"`
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

type DependencyManagement struct {
	Dependencies []Dependency `xml:"dependencies>dependency,omitempty"`
}

func (dm *DependencyManagement) merge(parent DependencyManagement) {
	dm.Dependencies = append(dm.Dependencies, parent.Dependencies...)
}

// Dependency contains relevant information about a Maven dependency.
// https://maven.apache.org/guides/introduction/introduction-to-dependency-mechanism.html
type Dependency struct {
	GroupID    String      `xml:"groupId,omitempty"`
	ArtifactID String      `xml:"artifactId,omitempty"`
	Version    String      `xml:"version,omitempty"`
	Type       String      `xml:"type,omitempty"`
	Classifier String      `xml:"classifier,omitempty"`
	Scope      String      `xml:"scope,omitempty"`
	Exclusions []Exclusion `xml:"exclusions>exclusion,omitempty"`
	Optional   BoolString  `xml:"optional,omitempty"`
}

func (d *Dependency) interpolate(properties map[string]string) bool {
	ok1 := d.GroupID.interpolate(properties)
	ok2 := d.ArtifactID.interpolate(properties)
	ok3 := d.Version.interpolate(properties)
	ok4 := d.Scope.interpolate(properties)
	ok5 := d.Type.interpolate(properties)
	ok6 := d.Classifier.interpolate(properties)
	ok7 := d.Optional.interpolate(properties)
	return ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7
}

type Exclusion struct {
	GroupID    String `xml:"groupId,omitempty"`
	ArtifactID String `xml:"artifactId,omitempty"`
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
	Enabled String `xml:"enabled"`
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
	p.Profiles = append(p.Profiles, parent.Profiles...)
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

// MaxImports defines the maximum number of dependency management imports allowed
const MaxImports = 300

// ProcessDependencies takes the following actions for Maven dependencies:
//   - dedupe dependencies and dependency management
//   - import dependency management (not yet transitively)
//   - fill in missing dependency version requirement
//
// A function to get dependency management from another project is needed
// since dependency management is imported transitively.
func (p *Project) ProcessDependencies(getDependencyManagement func(String, String, String) (DependencyManagement, error)) {
	// depKey uniquely identifies a Maven dependency.
	type depKey struct {
		groupID    String
		artifactID String
		typ        String
		classifier String
	}
	makeDepKey := func(dep Dependency) depKey {
		if dep.Type == "" {
			dep.Type = "jar"
		}
		return depKey{
			groupID:    dep.GroupID,
			artifactID: dep.ArtifactID,
			typ:        dep.Type,
			classifier: dep.Classifier,
		}
	}
	// addDepManagement adds dependency management in deps to m and returns:
	//  - a slice of keys of dependency management in deps that have been added to m;
	//  - a slice containing dependency management to be imported.
	addDepManagement := func(deps []Dependency, m map[depKey]Dependency) (keys []depKey, depImports []Dependency) {
		for _, dep := range deps {
			if dep.Scope == "import" {
				depImports = append(depImports, dep)
				continue
			}
			dk := makeDepKey(dep)
			if _, ok := m[dk]; !ok {
				m[dk] = dep
				keys = append(keys, dk)
			}
		}
		return
	}
	deps := make(map[depKey]Dependency, len(p.Dependencies))
	depKeys := make([]depKey, 0, len(p.Dependencies))
	for _, dep := range p.Dependencies {
		dk := makeDepKey(dep)
		if _, ok := deps[dk]; !ok {
			deps[dk] = dep
			depKeys = append(depKeys, dk)
		}
	}
	depManagement := make(map[depKey]Dependency, len(p.DependencyManagement.Dependencies))
	depManagementKeys, depManagementImports := addDepManagement(p.DependencyManagement.Dependencies, depManagement)
	// Append dependency management imports.
	depImportKeys := make(map[depKey]bool, len(depManagementImports))
	for _, dep := range depManagementImports {
		dk := makeDepKey(dep)
		if _, ok := depImportKeys[dk]; !ok {
			depImportKeys[dk] = true
		}
	}
	n := 0
	imported := make(map[depKey]bool)
	for ; n < MaxImports && len(depManagementImports) > 0; n++ {
		dep := depManagementImports[0]
		depManagementImports = depManagementImports[1:]
		dk := makeDepKey(dep)
		if imported[dk] {
			continue
		}
		imported[dk] = true
		if dep.Type != "pom" {
			continue
		}
		dm, err := getDependencyManagement(dep.GroupID, dep.ArtifactID, dep.Version)
		if err != nil {
			// Failed to fetch dependency management to import.
			continue
		}
		dependencyKeys, depImports := addDepManagement(dm.Dependencies, depManagement)
		depManagementKeys = append(depManagementKeys, dependencyKeys...)
		depManagementImports = append(depImports, depManagementImports...)
	}

	// There were dependencies and dependency management.
	p.Dependencies = []Dependency{}
	p.DependencyManagement.Dependencies = []Dependency{}
	for _, dk := range depKeys {
		dep := deps[dk]
		// Copy dependency info from dependency management if available.
		//  - Version: only copy when the field is empty;
		//  - Scope: only copy when the field is empty;
		//  - Exclusions: only copy when the field is empty;
		//  - Optional: dep always takes precedence.
		if dm, ok := depManagement[dk]; ok {
			if dep.Version == "" {
				dep.Version = dm.Version
			}
			if dep.Scope == "" {
				dep.Scope = dm.Scope
			}
			if len(dep.Exclusions) == 0 {
				dep.Exclusions = dm.Exclusions
			}
		}
		p.Dependencies = append(p.Dependencies, dep)
	}
	for _, dk := range depManagementKeys {
		dm := depManagement[dk]
		p.DependencyManagement.Dependencies = append(p.DependencyManagement.Dependencies, dm)
	}
}
