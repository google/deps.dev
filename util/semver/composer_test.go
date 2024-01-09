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

// Composer-specific tests.
// TODO: Investigate Composer's version semantics further.

var composerVersionParseTests = []versionParseTest{
	// The following tests are taken from advisory ranges observed in the wild.
	v("6.3.5.1", "", "6.3.5.1"),   // GHSA-jvg4-9rc2-wvcr.
	v("6.13.8.2", "", "6.13.8.2"), // GHSA-mrvj-7q4f-5p42.
	v("1.2.5", "", "1.2.5"),       // GHSA-mrvj-7q4f-5p42.
	v("2022.07", "", "2022.7"),    // GHSA-m8gv-gvhf-7rhp.
	v("v1.1", "", "1.1"),          // GHSA-rr5c-69c9-gj9f.

	// The following test is taken from a real package seen in the wild
	v("V1.0.0", "", "1.0.0"), // h2o-work/laravel-permission-manager

	// The following is an artificial test case
	v("vv1.1", "invalid version `vv1.1`", ""), // too many 'v's.
}

func TestComposerVersionParse(t *testing.T) {
	testVersionParse(t, Composer, composerVersionParseTests)
}
