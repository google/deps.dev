package resolve

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	pb "deps.dev/api/v3"
	"deps.dev/util/resolve/internal/deptest"
)

func TestNPMDependencies(t *testing.T) {
	ctx := context.Background()
	vk := func(name, version string) VersionKey {
		return VersionKey{
			PackageKey: PackageKey{
				System: NPM,
				Name:   name,
			},
			VersionType: Concrete,
			Version:     version,
		}
	}
	req := func(name, version, typ string) RequirementVersion {
		dt, err := deptest.ParseString(typ)
		if err != nil {
			t.Fatal(err)
		}
		return RequirementVersion{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: NPM,
					Name:   name,
				},
				VersionType: Requirement,
				Version:     version,
			},
			Type: dt,
		}
	}

	root := vk("test", "1.0.0")
	for _, c := range []struct {
		in  *pb.Requirements_NPM
		out map[VersionKey][]RequirementVersion
	}{{
		in:  &pb.Requirements_NPM{},
		out: nil,
	}, {
		in: &pb.Requirements_NPM{
			Dependencies: &pb.Requirements_NPM_Dependencies{
				Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "regular",
					Requirement: "^9.9.0",
				}},
				DevDependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "dev",
					Requirement: "^9.9.1",
				}},
				OptionalDependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "opt",
					Requirement: "^9.9.2",
				}},
				PeerDependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "peer",
					Requirement: "^9.9.3",
				}},
			},
		},
		out: map[VersionKey][]RequirementVersion{
			root: {
				req("opt", "^9.9.2", "opt"),
				req("peer", "^9.9.3", "Scope peer"),
				req("regular", "^9.9.0", ""),
				req("dev", "^9.9.1", "dev"),
			},
		},
	}, {
		in: &pb.Requirements_NPM{
			Dependencies: &pb.Requirements_NPM_Dependencies{
				Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "regular",
					Requirement: "^9.9.0",
				}},
				BundleDependencies: []string{"regular"},
			},
		},
		out: map[VersionKey][]RequirementVersion{
			root: {
				req("regular", "^9.9.0", ""),
				req("regular", "*", "Scope bundle"),
			},
		},
	}, {
		in: &pb.Requirements_NPM{
			Dependencies: &pb.Requirements_NPM_Dependencies{
				Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "regular",
					Requirement: "^9.9.0",
				}},
				BundleDependencies: []string{"regular"},
			},
			Bundled: []*pb.Requirements_NPM_Bundle{{
				Path:    "node_modules/regular",
				Name:    "regular",
				Version: "9.9.9",
			}},
		},
		out: map[VersionKey][]RequirementVersion{
			root: {
				req("regular", "^9.9.0", ""),
				req("regular", "*", "Scope bundle"),
				req("test>1.0.0>regular", "9.9.9", ""),
			},
			vk("test>1.0.0>regular", "9.9.9"): {},
		},
	}, {
		in: &pb.Requirements_NPM{
			Dependencies: &pb.Requirements_NPM_Dependencies{
				Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "regular",
					Requirement: "^9.9.0",
				}},
				BundleDependencies: []string{"regular"},
			},
			Bundled: []*pb.Requirements_NPM_Bundle{{
				Path:    "node_modules/regular",
				Name:    "regular",
				Version: "9.9.9",
				Dependencies: &pb.Requirements_NPM_Dependencies{
					Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
						Name:        "nested",
						Requirement: "^0.0.1",
					}},
				},
			}, {
				Path:    "node_modules/regular/node_modules/nested",
				Name:    "nested",
				Version: "0.0.1",
			}},
		},
		out: map[VersionKey][]RequirementVersion{
			root: {
				req("regular", "^9.9.0", ""),
				req("regular", "*", "Scope bundle"),
				req("test>1.0.0>regular", "9.9.9", ""),
			},
			vk("test>1.0.0>regular", "9.9.9"): {
				req("nested", "^0.0.1", ""),
				req("test>1.0.0>regular>nested", "0.0.1", ""),
			},
			vk("test>1.0.0>regular>nested", "0.0.1"): {},
		},
	}, {
		in: &pb.Requirements_NPM{
			Dependencies: &pb.Requirements_NPM_Dependencies{
				Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
					Name:        "a",
					Requirement: "^9.9.0",
				}, {
					Name:        "b",
					Requirement: "^9.9.0",
				}},
				BundleDependencies: []string{"a", "b"},
			},
			Bundled: []*pb.Requirements_NPM_Bundle{{
				Path:    "node_modules/a",
				Name:    "a",
				Version: "9.9.9",
				Dependencies: &pb.Requirements_NPM_Dependencies{
					Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
						Name:        "duplicate",
						Requirement: "^0.0.1",
					}},
				},
			}, {
				Path:    "node_modules/a/node_modules/duplicate",
				Name:    "duplicate",
				Version: "0.0.1",
			}, {
				Path:    "node_modules/b",
				Name:    "b",
				Version: "9.9.9",
				Dependencies: &pb.Requirements_NPM_Dependencies{
					Dependencies: []*pb.Requirements_NPM_Dependencies_Dependency{{
						Name:        "duplicate",
						Requirement: "^0.0.1",
					}},
				},
			}, {
				Path:    "node_modules/b/node_modules/duplicate",
				Name:    "duplicate",
				Version: "0.0.1",
			}},
		},
		out: map[VersionKey][]RequirementVersion{
			root: {
				req("a", "^9.9.0", ""),
				req("a", "*", "Scope bundle"),
				req("b", "^9.9.0", ""),
				req("b", "*", "Scope bundle"),
				req("test>1.0.0>a", "9.9.9", ""),
				req("test>1.0.0>b", "9.9.9", ""),
			},
			vk("test>1.0.0>a", "9.9.9"): {
				req("duplicate", "^0.0.1", ""),
				req("test>1.0.0>a>duplicate", "0.0.1", ""),
			},
			vk("test>1.0.0>a>duplicate", "0.0.1"): {},
			vk("test>1.0.0>b", "9.9.9"): {
				req("duplicate", "^0.0.1", ""),
				req("test>1.0.0>b>duplicate", "0.0.1", ""),
			},
			vk("test>1.0.0>b>duplicate", "0.0.1"): {},
		},
	}} {
		client := APIClient{
			bundledVersions: make(map[string]bundledVersion),
		}
		// Start with the root, to populate any bundles.
		got, err := client.npmRequirements(root, c.in)
		if err != nil && len(c.out[root]) != 0 {
			t.Errorf("npmDependencies(%v): %v", c.in, err)
			continue
		}
		if d := cmp.Diff(c.out[root], got); d != "" {
			t.Errorf("npmDependencies(%v):\n(- want, + got):\n%s", c.in, d)
		}
		// Check bundles.
		for v, want := range c.out {
			if v == root {
				continue
			}
			got, err := client.Requirements(ctx, v)
			if err != nil {
				if want != nil {
					t.Errorf("npmDependencies(%v): %v", c.in, err)
				}
				continue
			}
			if got == nil {
				got = []RequirementVersion{}
			}
			if d := cmp.Diff(want, got); d != "" {
				t.Errorf("npmDependencies(%v):\n(- want, + got):\n%s", c.in, d)
			}
		}
	}
}
