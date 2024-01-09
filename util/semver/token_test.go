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

import "testing"

func TestToken(t *testing.T) {
	tests := []struct {
		sys   System
		str   string
		typ   tokType
		token string
		next  int
	}{
		{NPM, "", tokEOF, "", 0},
		{NPM, "     ", tokEOF, "", 5},

		// Versions.
		{NPM, "1", tokVersion, "1", 1},
		{NPM, "1 ", tokVersion, "1", 1},
		{NPM, "1, ", tokVersion, "1", 1},
		{NPM, "  1  ", tokVersion, "1", 3},
		{NPM, "  1.2.3-alpha  ", tokVersion, "1.2.3-alpha", 13},
		{NPM, "  1.2.3-alpha+beta.2  ", tokVersion, "1.2.3-alpha+beta.2", 20},
		{NPM, "  1.2.3.4.5.6.7  ", tokVersion, "1.2.3.4.5.6.7", 15}, // Invalid version, but the token function doesn't check that.

		// Wildcards.
		{NPM, "x", tokWildcard, "x", 1},
		{NPM, "X", tokWildcard, "X", 1},
		{NPM, "  1.*.0  ", tokWildcard, "1.*.0", 7},
		{NPM, "  1.0.*  ", tokWildcard, "1.0.*", 7},
		{NPM, "  1.x.0  ", tokWildcard, "1.x.0", 7},
		{NPM, "  1.X.0  ", tokWildcard, "1.X.0", 7},
		{NPM, "  1.X.0-rc1  ", tokWildcard, "1.X.0-rc1", 11},

		// Tricky confusion between wildcard and version.
		{NPM, "1-x", tokVersion, "1-x", 3},
		{NPM, "1-X", tokVersion, "1-X", 3},
		{NPM, "1.0-x", tokVersion, "1.0-x", 5},
		{NPM, "1.0-X", tokVersion, "1.0-X", 5},
		{NPM, "1.0.0-x", tokVersion, "1.0.0-x", 7},
		{NPM, "1.0.0-X", tokVersion, "1.0.0-X", 7},

		// Operators
		{NPM, "  ||  ", tokOr, "||", 4},
		{NPM, "  -  ", tokHyphen, "-", 3},
		{NPM, "  =  ", tokEqual, "=", 3},
		{NPM, "  >  ", tokGreater, ">", 3},
		{NPM, "  >=  ", tokGreaterEqual, ">=", 4},
		{NPM, "  <  ", tokLess, "<", 3},
		{NPM, "  <=  ", tokLessEqual, "<=", 4},
		{NPM, "  ~  ", tokTilde, "~", 3},
		{NPM, "  ~>  ", tokTilde, "~>", 4}, // See token.go
		{RubyGems, "  ~>  ", tokBacon, "~>", 4},
		{PyPI, "  ~=  ", tokBacon, "~=", 4},

		// Oddballs
		{RubyGems, "  !=  ", tokNotEqual, "!=", 4},
		{RubyGems, "  ,  ", tokComma, ",", 3},

		// Invalid things.
		{NPM, " ,,  ", tokInvalid, ",", 2},
		{NPM, " |  ", tokInvalid, "|", 2},
		{NPM, " [  ", tokInvalid, "[", 2},
		{NPM, " (  ", tokInvalid, "(", 2},
		{NPM, " ]  ", tokInvalid, "]", 2},
		{NPM, " )  ", tokInvalid, ")", 2},
	}
	for _, test := range tests {
		typ, token, next := test.sys.token(test.str)
		if typ != test.typ || token != test.token || next != test.next {
			t.Errorf("token(%q) = %s %q %d; expect %s %q %d", test.str,
				typ, token, next,
				test.typ, test.token, test.next)
		}
	}
}
