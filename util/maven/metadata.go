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

// Metadata contains repository information of a package.
// https://maven.apache.org/ref/3.9.3/maven-repository-metadata/repository-metadata.html
type Metadata struct {
	ModelVersion String           `xml:"modelVersion"`
	GroupID      String           `xml:"groupId"`
	ArtifactID   String           `xml:"artifactId"`
	Versioning   Versioning       `xml:"versioning"`
	Version      String           `xml:"version"`
	Plugins      []MetadataPlugin `xml:"plugins>plugin"`
}

type Versioning struct {
	Latest           String            `xml:"latest"`
	Release          String            `xml:"release"`
	Versions         []String          `xml:"versions>version"`
	LastUpdated      String            `xml:"lastUpdated"`
	Snapshot         Snapshot          `xml:"snapshot"`
	SnapshotVersions []SnapshotVersion `xml:"snapshotVersions>snapshotVersion"`
}

type Snapshot struct {
	Timestamp   String `xml:"timestamp"`
	BuildNumber int    `xml:"buildNumber"`
	LocalCopy   bool   `xml:"localCopy"`
}

type SnapshotVersion struct {
	Classifier String `xml:"classifier"`
	Extension  String `xml:"extension"`
	Value      String `xml:"value"`
	Updated    String `xml:"updated"`
}

type MetadataPlugin struct {
	Name       String `xml:"name"`
	Prefix     String `xml:"prefix"`
	ArtifactID String `xml:"artifactId"`
}
