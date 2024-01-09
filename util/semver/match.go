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

import "strings"

// This file contains the implementation of constraint matching. See
// https://docs.npmjs.com/misc/semver for the Node-specific subset, including a
// test evaluator at https://semver.npmjs.com/. RubyGems constraint
// documentation has gone walkabout, however the (non-NPM) ~>
// ("pessimistic", "bacon-eater", "tilde-wakka", "twiddle-wakka" or
// "compatible-with") operator is described a little in
// https://depfu.com/blog/2016/12/14/get-to-know-your-twiddle-wakka
// and the code helps:
// https://github.com/rubygems/rubygems/blob/master/lib/rubygems/version.rb

/*
Match reports whether the version represented by the argument string
satisfies the constraint. It returns false if the argument is
invalid or a wildcard.

A user's version U = x.y.z matches a simple constraint version V
(a.b.c), possibly with an operator, by applying these rules given
the operator:

	""  (nothing) U == V (using the rules of version.Compare)
	=   U == V
		This operator is spelled == in PyPI.
	>=  U >= V
	<   U < V
	<=  U <= V
	^   (major range operator) U >= a.b.c AND U < (a+1).0.0.
		• Cargo behaves differently when given exactly two zeros:
			- ^0.0  means >=0.0.0 AND < 0.1.0
		Otherwise it is the same as the others.
	~   (minor range operator)
		• If a.b.c are present, x == a AND y == b AND c >= z
		• If only a.b are present, x == a AND y == b
		• If only a is present, x == a
	~>  (pessimistic operator, a.k.a. compatible with and other fanciful names)
		This operator is spelled ~= in PyPI. In NPM, this token is supported
		but is just a variant spelling of ~.
		• If a.b.c are present, x == a AND y == b AND c >= z
		• If only a.b are present, x == a AND y >= b
		• If only a is present, x == a

For expressions involving lists separated by commas (",") ors ("||"), or
spaces (" "), the precedence order is in that order in increasing precedence:
Comma binds loosest, then ||, then spaces. Commas and spaces represent "AND"
(conjunction); ors represent "OR" (disjunction).
See the package comment for information about which packaging
systems support which operators.

Maven represents constraints as unions of ranges, not using the syntax
described above. For Maven, therefore, Match reports whether the
version is in any of the ranges.
*/
func (c *Constraint) Match(version string) bool {
	v, err := c.sys.Parse(version)
	if err != nil {
		return false
	}
	return c.match(v)
}

// MatchVersion is like Match but it takes a *Version.
func (c *Constraint) MatchVersion(v *Version) bool {
	if v.IsWildcard() {
		return false
	}
	return c.match(v)
}

// match is a helper that also tweaks prerelease in some cases.
func (c *Constraint) match(v *Version) bool {
	prerelease := false
	// NuGet matches prereleases if the constraint is a plain version.
	// Ranges and floating constraints (*) don't match prereleases unless
	// explicitly specified.
	if c.sys == NuGet && !strings.ContainsAny(c.str, "[(,]*") {
		prerelease = true
	}
	// The empty constraint in PyPI does not match dev versions.
	if c.sys == PyPI && c.str == "" && v.ext.(*pep440Extension).isDev() {
		return false
	}
	return c.set.matchVersion(v, prerelease)
}

// MatchVersionPrerelease is like Match but if v contains prereleases, they are
// ignored for the purpose of matching (assuming the constraint does not have
// them), and thus v matches if just the numbers match.
// It is used when matching vulnerabilities, not during constraint satisfaction
// for builds.
func (c *Constraint) MatchVersionPrerelease(v *Version) bool {
	if v.IsWildcard() {
		return false
	}
	return c.set.matchVersion(v, true)
}
