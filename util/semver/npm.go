// Copyright 2023 Google LLC
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

package semver

import (
	"fmt"
	"strings"
)

// NPM-specific support for some functionality.

// CalculateMinVersion returns the minimum version that satisfies the constraint. It is parallel to
// https://github.com/npm/node-semver/blob/main/ranges/min-version.js.
// This currently only works for NPM.
func (c *Constraint) CalculateMinVersion() (*Version, error) {
	minNonBuildVersion, _ := NPM.Parse("0.0.0")
	if c.sys != NPM {
		return nil, fmt.Errorf("calculateMinVersion is only supported by NPM")
	}
	if c.Set().Empty() {
		// An empty set of spans indicates an unsatisfiable constraint.
		return nil, fmt.Errorf("constraint is unsatisfiable")
	}

	// The empty string constraint "" means "any version", for which the lowest
	// is "0.0.0".
	if c.String() == "" {
		return minNonBuildVersion, nil
	}

	// This assumes canon was called somewhere, to set span[0] to the minimum version.
	s := c.set.span[0]
	v := s.min.copy()

	// The lower bound is inclusive, so it's the minimum version.
	// Use 0.0.0 instead of 0.0.0-0 as the minimum value, to be consistent with
	// NPM's implementation.
	// See https://github.com/npm/node-semver/blob/main/ranges/min-version.js and
	// https://github.com/npm/node-semver/blob/main/ranges/min-version.js.
	if !s.minOpen {
		if v.String() == "0.0.0-0" {
			return minNonBuildVersion, nil
		}
		return v, nil
	}

	// Otherwise, calculate the next version after it.
	if !v.IsPrerelease() {
		// For non-prereleases, the next version is the next patch.
		v.incN(nPatch)
		return v, nil
	}

	// For pre-releases, we must increment the pre-release identifier by adding a ".0".
	// Build metadata (after "+") is not considered in version comparison, so it is stripped.
	fullStr := v.String()
	if buildIndex := strings.Index(fullStr, "+"); buildIndex != -1 {
		fullStr = fullStr[:buildIndex]
	}
	v, err := NPM.Parse(fullStr + ".0")
	if err != nil {
		return nil, err
	}
	return v, nil
}
