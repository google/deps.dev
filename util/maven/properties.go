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
	"strings"
)

// Properties hold property pairs defined in a pom.xml.
type Properties struct {
	Properties []Property
}

type Property struct {
	Name  string
	Value string
}

// UnmarshalXML unmarshals properties defined in pom.xml and stores
// them in a slice of Property.
//
// The properties section should be follow the format below:
//
// <properties>
//
//	<name1>value1</name1>
//	<name2>value2</name2>
//	...
//
// </properties>
func (p *Properties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t1 := t.(type) {
		case xml.StartElement:
			var s string
			if err := d.DecodeElement(&s, &t1); err != nil {
				return err
			}
			p.Properties = append(p.Properties, Property{
				Name:  t1.Name.Local,
				Value: strings.TrimSpace(s),
			})
		case xml.EndElement:
			return nil
		}
	}
}

func (p *Properties) merge(parent Properties) {
	p.Properties = append(parent.Properties, p.Properties...)
}

// propertyMap returns the property map with project properties and
// properties defined in p.Properties.
//
// For Maven 3.9, project properties with additional prefix (pom.* and
// project.*) cannot be overwritten by explictly defined properties,
// Project properties without additional prefix can be overwritten.
//
// Also the properties without additional prefix and those with `pom.`
// prefix are deprecated, it is suggested to use the properties with
// `project.` prefix instead.
func (p *Project) propertyMap() (map[string]string, error) {
	m := make(map[string]string)
	for _, prop := range p.Properties.Properties {
		// Replace any value that was previously set.
		m[prop.Name] = prop.Value
	}

	addProjectProperty := func(k string, v String) {
		if v == "" {
			return
		}
		// Do not overwrite the project properties without additional prefix.
		if _, ok := m[k]; !ok {
			m[k] = string(v)
		}
		m["pom."+k] = string(v)
		m["project."+k] = string(v)
	}
	addProjectProperty("groupId", p.GroupID)
	addProjectProperty("version", p.Version)
	addProjectProperty("parent.groupId", p.Parent.GroupID)
	addProjectProperty("parent.version", p.Parent.Version)
	return m, nil
}
