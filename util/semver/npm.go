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
	"strconv"
	"strings"
)

// NPM-specific support for some functionality.

// CalculateMinVersion returns the minimum version that satisfies the constraint. It is parallel to
// https://github.com/npm/node-semver/blob/main/ranges/min-version.js.
func (c *Constraint) CalculateMinVersion() (*Version, error) {
	if c.sys != NPM {
		return nil, fmt.Errorf("calculateMinVersion is only supported by NPM")
	}
	if c.Set().Empty() {
		// An empty set of spans indicates an unsatisfiable constraint.
		return nil, fmt.Errorf("constraint is unsatisfiable")
	}

	// The empty string constraint "" means "any version", for which the lowest
	// is "0.0.0-0".
	if c.String() == "" {
		return NPM.Parse("0.0.0-0")
	}

	s := c.set.span[0]
	v := s.min.copy()

	// The lower bound is inclusive, so it's the minimum version.
	if !s.minOpen {
		return v, nil
	}

	// Otherwise, calculate the next version after it.
	if !v.IsPrerelease() {
		// For non-prereleases, the next version is the next patch.
		v.incN(nPatch)
		return v, nil
	}

	// For pre-releases, we must increment the pre-release identifier.
	// e.g., >2.0.0-alpha -> 2.0.0-alpha.0
	// e.g., >2.0.0-alpha.0 -> 2.0.0-alpha.1
	// Build metadata (after "+") is not considered in version comparison, so it is stripped.
	fullStr := v.String()
	if buildIndex := strings.Index(fullStr, "+"); buildIndex != -1 {
 		fullStr = fullStr[:buildIndex]
 	}
	preIndex := strings.Index(fullStr, "-")
	if preIndex == -1 {
		// This should not happen if IsPrerelease is true.
		return nil, fmt.Errorf("internal error: IsPrerelease is true but no '-' in version string %q", fullStr)
	}
	base := fullStr[:preIndex]
	pre := fullStr[preIndex+1:]

	parts := strings.Split(pre, ".")
	lastPart := parts[len(parts)-1]
	num, err := strconv.Atoi(lastPart)
	var newPre string
	if err == nil {
		// Last part is numeric, so we increment it.
		parts[len(parts)-1] = strconv.Itoa(num + 1)
		newPre = strings.Join(parts, ".")
	} else {
		// Last part is non-numeric, so we append ".0".
		newPre = pre + ".0"
	}
	v, err = NPM.Parse(base + "-" + newPre)
	if err != nil {
		return nil, err
	}
	return v, nil
}
