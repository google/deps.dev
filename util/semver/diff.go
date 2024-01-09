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

// Diff characterizes the most significant manner in which two versions differ:
// by major number, by minor number, and so on.
type Diff int

//go:generate stringer -type Diff -trimprefix Diff

// Possible differences between versions, from most to least significant.
const (
	Same           Diff = iota // No difference.
	DiffOther                  // Unqualifiable difference.
	DiffMajor                  // Difference in Major number.
	DiffMinor                  // Difference in Minor number.
	DiffPatch                  // Difference in Patch number.
	DiffPrerelease             // Difference in prerelease information.
	DiffBuild                  // Difference in build tag.
)

// Difference parses a and b and, if there is no error, returns the result of
// a.Difference(b).
func (sys System) Difference(a, b string) (int, Diff, error) {
	av, err := sys.Parse(a)
	if err != nil {
		return 0, DiffOther, err
	}
	bv, err := sys.Parse(b)
	if err != nil {
		return 0, DiffOther, err
	}
	c, d := av.Difference(bv)
	return c, d, nil
}

// Difference reports the level of the most significant difference between u and
// v. The return values are the result of v.Compare(u) and the type of
// difference. For example, the difference between 1.2.3 and 1.3.4 is MinorDiff.
// If the difference is not well characterized by the definition of Semver 2.0,
// Difference returns OtherDiff.
// Note that since build tags are ignored by Compare, Difference can return
// (0, BuildDiff).
func (v *Version) Difference(u *Version) (int, Diff) {
	c := v.Compare(u)
	if c == 0 && v.build == u.build { // Build differences are ignored in Compare.
		return c, Same
	}

	// Special case, varies too much from Semver 2.0, but often works,
	// so let Maven-specific code try.
	switch v.sys {
	case Maven:
		return c, mavenDifference(v, u)
	}

	switch {
	case v.major() != u.major():
		return c, DiffMajor
	case v.minor() != u.minor():
		return c, DiffMinor
	case v.patch() != u.patch():
		return c, DiffPatch
	case len(v.num) != 3 || len(u.num) != 3:
		// Too messy, give up.
		return c, DiffOther
	case comparePrerelease(u, v) != 0:
		return c, DiffPrerelease
	case u.build != v.build:
		return c, DiffBuild
	}

	return c, DiffOther // We know they're not the same, but not how.
}
