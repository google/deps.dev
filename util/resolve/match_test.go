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

package resolve

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/version"
)

func TestSortVersions(t *testing.T) {
	v := func(v string) Version {
		return Version{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: NPM,
					Name:   "test",
				},
				VersionType: Concrete,
				Version:     v,
			},
		}
	}
	for _, vs := range [][]Version{{
		v("0.0.0"), v("0.0.1"), v("0.2.0-a"), v("0.2.0"),
	}, {
		v("1.0.0"), v("2.0.0"), v("invalid version"),
	}, {
		v("0.0.0"), v("not numbers"), v("not numbers, but longer"),
	}, {
		v("1.0.0-0"), v("1.0.0-a"), v("1.0.0-a.0"), v("1.0.0-a.a"),
	}} {
		got := append(make([]Version, 0, len(vs)), vs...)
		for i := 0; i < 10; i++ {
			rand.Shuffle(len(got), func(i, j int) {
				got[i], got[j] = got[j], got[i]
			})
			SortVersions(got)
			if diff := cmp.Diff(got, vs); diff != "" {
				t.Errorf("SortVersions:\n(-got, +want):\n%s", diff)
			}
		}
	}
}

func TestSortNPMDependencies(t *testing.T) {
	buildImport := func(dt dep.Type, name string) RequirementVersion {
		return RequirementVersion{
			Type: dt,
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: NPM,
					Name:   name,
				},
				VersionType: Requirement,
				Version:     "*",
			},
		}
	}

	var (
		reg    = dep.Type{}
		dev    = dep.NewType(dep.Dev)
		opt    = dep.NewType(dep.Opt)
		devopt = dep.NewType(dep.Dev, dep.Opt)
		alias  = dep.NewType()
		zoe    = dep.NewType()
	)
	alias.AddAttr(dep.KnownAs, "alias")
	zoe.AddAttr(dep.KnownAs, "zoe")
	cases := [][]RequirementVersion{
		{
			buildImport(alias, "zoe"),
			buildImport(reg, "alice"),
			buildImport(reg, "bob"),
			buildImport(reg, "BOB"),
			buildImport(reg, "chuck"),
			buildImport(zoe, "alias"),
			buildImport(reg, "zone"),
		},
		{
			buildImport(reg, "BOB"),
			buildImport(dev, "alice"),
			buildImport(dev, "bob"),
		},
		{
			buildImport(reg, "alice"),
			buildImport(devopt, "bob"),
			buildImport(opt, "BOB"),
			buildImport(reg, "chuck"),
		},
	}

	for _, want := range cases {
		got := append(make([]RequirementVersion, 0, len(want)), want...)
		// Test different shuffled imports.
		for i := 0; i < 10; i++ {
			rand.Shuffle(len(got), func(i, j int) {
				got[i], got[j] = got[j], got[i]
			})
			SortDependencies(got)
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("SortDependencies:\n(-got, +want):\n%s", diff)
			}
		}
	}

}

func TestMatchNPMRequirement(t *testing.T) {
	vk := func(v string) VersionKey {
		return VersionKey{
			PackageKey: PackageKey{
				System: NPM,
				Name:   "test",
			},
			VersionType: Concrete,
			Version:     v,
		}
	}
	tags := func(s ...string) (attr version.AttrSet) {
		attr.SetAttr(version.Tags, strings.Join(s, ","))
		return attr
	}
	v := func(ver string, ts ...string) Version {
		v := Version{VersionKey: vk(ver)}
		if len(ts) > 0 {
			v.AttrSet = tags(ts...)
		}
		return v
	}

	versions := []Version{
		v("0.0.0"),
		v("0.1.0"),
		v("0.1.1", "special"),
		v("1.0.1-a"),
		v("1.0.1"),
		v("1.0.2", "latest"),
		v("2.0.0"),
	}

	for _, c := range []struct {
		in   []Version
		req  VersionKey
		want []Version
	}{{
		req:  vk("=0.1.0"),
		want: []Version{v("0.1.0")},
	}, {
		req:  vk("^0.0.0"),
		want: []Version{v("0.0.0")},
	}, {
		req: vk("^1.0.1-0"),
		want: []Version{
			v("1.0.1-a"),
			v("1.0.1"),
			v("1.0.2", "latest"),
		},
	}, {
		// match everything except prerelease.
		req: vk("*"),
		want: []Version{
			v("0.0.0"),
			v("0.1.0"),
			v("0.1.1", "special"),
			v("1.0.1"),
			v("2.0.0"),
			v("1.0.2", "latest"), // latest goes last.
		},
	}, {
		req: vk(""),
		want: []Version{
			v("0.0.0"),
			v("0.1.0"),
			v("0.1.1", "special"),
			v("1.0.1"),
			v("2.0.0"),
			v("1.0.2", "latest"),
		},
	}, {
		req: vk(">1.0.0"),
		want: []Version{
			v("1.0.1"),
			v("2.0.0"),
			v("1.0.2", "latest"),
		},
	}, {
		req: vk(">2.0.0"),
	}, {
		req: vk("invalid"),
	}, {
		req:  vk("special"),
		want: []Version{v("0.1.1", "special")},
	}, {
		req:  vk("latest"),
		want: []Version{v("1.0.2", "latest")},
	}, {
		req: vk("<1.0.1||>=2.0.0"),
		want: []Version{
			v("0.0.0"),
			v("0.1.0"),
			v("0.1.1", "special"),
			v("2.0.0"),
		},
	}, {
		in: []Version{
			v("1.0.0-a", "latest"),
			v("1.0.0-b"),
			v("1.0.0-c"),
		},
		req: vk(">=1.0.0-0"),
		want: []Version{
			v("1.0.0-b"),
			v("1.0.0-c"),
			v("1.0.0-a", "latest"),
		},
	}, {
		in: []Version{
			v("1.0.0-a", "latest"),
			v("1.0.0-c"),
			v("1.0.0"),
		},
		req: vk(">=1.0.0-a"),
		want: []Version{
			v("1.0.0-a", "latest"),
			v("1.0.0-c"),
			v("1.0.0"),
		},
	}, {
		in: []Version{
			v("abc", "latest"),
			v("1.0.0"),
		},
		req: vk("*"),
		want: []Version{
			v("1.0.0"),
		},
	}} {
		if c.in == nil {
			c.in = versions
		}
		got := matchNPMRequirement(c.req, c.in)
		if c.want == nil {
			c.want = []Version{}
		}
		if got == nil {
			got = []Version{}
		}
		if diff := cmp.Diff(got, c.want); diff != "" {
			t.Errorf("matchNPMRequirement(%v):\n(-got, +want):\n%s", c.req, diff)
		}
	}
}
