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

	"deps.dev/util/semver"
)

// Profile contains information of a build profile.
// https://maven.apache.org/guides/introduction/introduction-to-profiles.html
type Profile struct {
	ID                   String               `xml:"id,omitempty"`
	Activation           Activation           `xml:"activation,omitempty"`
	Properties           Properties           `xml:"properties,omitempty"`
	DependencyManagement DependencyManagement `xml:"dependencyManagement,omitempty"`
	Dependencies         []Dependency         `xml:"dependencies>dependency,omitempty"`
	Repositories         []Repository         `xml:"repositories>repository,omitempty"`
}

// Activation contains information to decide if a build profile is activated or not.
// https://maven.apache.org/guides/introduction/introduction-to-profiles.html#details-on-profile-activation=
type Activation struct {
	ActiveByDefault FalsyBool          `xml:"activeByDefault,omitempty"`
	JDK             String             `xml:"jdk,omitempty"`
	OS              ActivationOS       `xml:"os,omitempty"`
	Property        ActivationProperty `xml:"property,omitempty"`
	File            ActivationFile     `xml:"file,omitempty"`
}

type ActivationOS struct {
	Name    String `xml:"name,omitempty"`
	Family  String `xml:"family,omitempty"`
	Arch    String `xml:"arch,omitempty"`
	Version String `xml:"version,omitempty"`
}

func (ao ActivationOS) blank() bool {
	return ao.Name == "" && ao.Family == "" && ao.Arch == "" && ao.Version == ""
}

type ActivationProperty struct {
	Name  String `xml:"name,omitempty"`
	Value String `xml:"value,omitempty"`
}

func (ap ActivationProperty) blank() bool {
	return ap.Name == "" && ap.Value == ""
}

type ActivationFile struct {
	Missing String `xml:"missing,omitempty"`
	Exists  String `xml:"exists,omitempty"`
}

func (af ActivationFile) blank() bool {
	return af.Exists == "" && af.Missing == ""
}

// activated returns if a Maven build profile is activated or not.
// If no JDK or OS information is provided, the profile is considered
// as not activated.
// Since Maven 3.2.2 Activation occurs when all of the specified criteria have
// been met: https://maven.apache.org/pom.html#activation
// TODO: support profile activation on File.
func (p *Profile) activated(jdk string, os ActivationOS) (bool, error) {
	if jdk == "" && os.blank() {
		return false, nil
	}

	act := p.Activation
	res := false
	if act.JDK != "" {
		c, err := semver.Maven.ParseConstraint(string(act.JDK))
		if err != nil {
			return false, err
		}
		if c.IsSimple() {
			// A profile should be active when the JDK version is of
			// the same major and minor number.
			// https://maven.apache.org/guides/introduction/introduction-to-profiles.html#jdk
			cmp, diff, err := semver.Maven.Difference(string(act.JDK), jdk)
			if err != nil {
				return false, err
			}
			if cmp > 0 || (cmp < 0 && (diff == semver.DiffMajor || diff == semver.DiffMinor)) {
				return false, nil
			}
		} else {
			if !c.Match(jdk) {
				return false, nil
			}
		}
		res = true
	}
	if !act.OS.blank() {
		// isAllowed reports whether the given value is compatible with the
		// expected value. The comparison is not case sensitive and is negated
		// if the value is prefixed by !.
		// https://maven.apache.org/enforcer/enforcer-rules/requireOS.html
		isAllowed := func(value, expected String) bool {
			got, want := string(value), string(expected)
			if got == "" {
				return true
			}
			negate := false
			if strings.HasPrefix(got, "!") {
				got = strings.TrimPrefix(got, "!")
				negate = true
			}
			got = strings.ToLower(got)
			return negate && got != want || !negate && got == want
		}
		if !isAllowed(act.OS.Family, os.Family) ||
			!isAllowed(act.OS.Name, os.Name) ||
			!isAllowed(act.OS.Version, os.Version) ||
			!isAllowed(act.OS.Arch, os.Arch) {
			return false, nil
		}
		res = true
	}
	if name := string(act.Property.Name); name != "" {
		if value := string(act.Property.Value); value == "" {
			if !strings.HasPrefix(name, "!") {
				return false, nil
			}
		} else {
			if !strings.HasPrefix(value, "!") {
				return false, nil
			}
		}
		res = true
	}
	return res, nil
}

const (
	// JDKProfileActivation holds the JDK version used for profile activation.
	// This is arbitrary for now.
	// TODO: this should be abstracted and set as an option.
	JDKProfileActivation = "11.0.8"
)

var (
	// OSProfileActivation holds the OS settings used for profile
	// activation. This is arbitrary, it was obtained by running `mvn
	// enforcer:display-info` in an amd64 Google Compute Engine VM, running
	// Debian 11 and Maven 3.6.3.
	// TODO: this should be abstracted and set as an option.
	OSProfileActivation = ActivationOS{
		Name:    "linux",
		Family:  "unix",
		Arch:    "amd64",
		Version: "5.10.0-26-cloud-amd64",
	}
)

// MergeProfiles merge the data in activated profiles to the project.
// If there is no active profile, merge the data from default profiles.
// If no JDK or OS information is provided, default profiles are merged.
// The activation is based on the constants specified above.
func (p *Project) MergeProfiles(jdk string, os ActivationOS) (err error) {
	activeProfiles := make([]Profile, 0, len(p.Profiles))
	defaultProfiles := make([]Profile, 0, len(p.Profiles))
	for _, prof := range p.Profiles {
		act, actErr := prof.activated(jdk, os)
		if actErr != nil {
			// Keep the error for later, and try other profiles.
			err = appendError(err, actErr)
		}
		if act {
			activeProfiles = append(activeProfiles, prof)
		}
		if prof.Activation.ActiveByDefault.Boolean() {
			defaultProfiles = append(defaultProfiles, prof)
		}
	}
	// Merge default active profiles if no other profile is active.
	if len(activeProfiles) == 0 {
		activeProfiles = defaultProfiles
	}
	for _, prof := range activeProfiles {
		// Properties in active profiles should overwrite global properties.
		prof.Properties.merge(p.Properties)
		p.Properties = prof.Properties

		p.DependencyManagement.merge(prof.DependencyManagement)
		p.Dependencies = append(p.Dependencies, prof.Dependencies...)
		p.Repositories = append(p.Repositories, prof.Repositories...)
	}
	return
}

func appendError(e1, e2 error) error {
	if e1 == nil {
		return e2
	}
	return fmt.Errorf("%w, %w", e1, e2)
}
