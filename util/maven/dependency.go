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
	"strings"
)

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
	Optional   FalsyBool   `xml:"optional,omitempty"`
}

type Exclusion struct {
	GroupID    String `xml:"groupId,omitempty"`
	ArtifactID String `xml:"artifactId,omitempty"`
}

func (d *Dependency) Name() string {
	return fmt.Sprintf("%s:%s", d.GroupID, d.ArtifactID)
}

func (d *Dependency) ExclusionsString() string {
	var exclusions strings.Builder
	first := true
	for _, ex := range d.Exclusions {
		if strings.Contains(string(ex.GroupID), "|") || strings.Contains(string(ex.ArtifactID), "|") {
			// Skip this exclusion if it contains a pipe.
			continue
		}
		if !first {
			exclusions.WriteString("|")
		}
		exclusions.WriteString(string(ex.GroupID) + ":" + string(ex.ArtifactID))
		first = false
	}
	return exclusions.String()
}

// DependencyKey uniquely identifies a Maven dependency.
type DependencyKey struct {
	GroupID    String
	ArtifactID String
	Type       String
	Classifier String
}

func (d *Dependency) Key() DependencyKey {
	if d.Type == "" {
		d.Type = "jar"
	}
	return DependencyKey{
		GroupID:    d.GroupID,
		ArtifactID: d.ArtifactID,
		Type:       d.Type,
		Classifier: d.Classifier,
	}
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

type DependencyManagement struct {
	Dependencies []Dependency `xml:"dependencies>dependency,omitempty"`
}

func (dm *DependencyManagement) merge(parent DependencyManagement) {
	dm.Dependencies = append(dm.Dependencies, parent.Dependencies...)
}

// MaxImports defines the maximum number of dependency management imports allowed
const MaxImports = 300

// ProcessDependencies takes the following actions for Maven dependencies:
//   - dedupe dependencies and dependency management
//   - import dependency management
//   - fill in missing dependency version requirement
//
// A function to get dependency management from another project is needed
// since dependency management is imported transitively.
func (p *Project) ProcessDependencies(getDependencyManagement func(String, String, String) (DependencyManagement, error)) {
	// addDepManagement adds dependency management in deps to m and returns:
	//  - a slice of keys of dependency management in deps that have been added to m;
	//  - a slice containing dependency management to be imported.
	addDepManagement := func(deps []Dependency, m map[DependencyKey]Dependency) (keys []DependencyKey, depImports []Dependency) {
		for _, dep := range deps {
			if dep.Scope == "import" {
				depImports = append(depImports, dep)
				continue
			}
			dk := dep.Key()
			if _, ok := m[dk]; !ok {
				m[dk] = dep
				keys = append(keys, dk)
			}
		}
		return
	}
	deps := make(map[DependencyKey]Dependency, len(p.Dependencies))
	depKeys := make([]DependencyKey, 0, len(p.Dependencies))
	for _, dep := range p.Dependencies {
		dk := dep.Key()
		if _, ok := deps[dk]; !ok {
			deps[dk] = dep
			depKeys = append(depKeys, dk)
		}
	}
	depManagement := make(map[DependencyKey]Dependency, len(p.DependencyManagement.Dependencies))
	depManagementKeys, depManagementImports := addDepManagement(p.DependencyManagement.Dependencies, depManagement)
	// Append dependency management imports.
	depImportKeys := make(map[DependencyKey]bool, len(depManagementImports))
	for _, dep := range depManagementImports {
		dk := dep.Key()
		if _, ok := depImportKeys[dk]; !ok {
			depImportKeys[dk] = true
		}
	}
	n := 0
	imported := make(map[DependencyKey]bool)
	for ; n < MaxImports && len(depManagementImports) > 0; n++ {
		dep := depManagementImports[0]
		depManagementImports = depManagementImports[1:]
		dk := dep.Key()
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
