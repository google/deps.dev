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
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "deps.dev/api/v3"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/version"
)

// APIClient is a Client that fetches data from the deps.dev API. For now it
// only supports NPM because that is the only system with a resolver
// implementation. It performs no caching, and nearly every method is an API
// call so it can be slow when resolving large dependency graphs. Note that
// bundled versions are constructed from the bundling version's Requirements
// call, so will be inaccessible until this is called at which point the client
// will store them. For dependency resolution this is typically not an issue, as
// bundled versions will only be visited after the version that bundles them. It
// is safe for concurrent use.
type APIClient struct {
	c pb.InsightsClient

	// bundledVersionsMu controls access to bundledVersions.
	bundledVersionsMu sync.Mutex
	// bundledVersions holds bundled npm packages. It is populated with the
	// results of calls to GetRequirements, and is keyed by the mangled
	// names the resolver uses to refer to such versions, which include
	// the bundling package and version as well as the path to the bundled
	// package version within the bundle. The names should be considered
	// opaque.
	bundledVersions map[string]bundledVersion
}

// bundledVersion is an npm package that was found inside another npm package.
type bundledVersion struct {
	Version
	requirements []RequirementVersion
}

// NewAPIClient creates a new APIClient using the provided gRPC client to
// call the deps.dev Insights service.
func NewAPIClient(c pb.InsightsClient) *APIClient {
	return &APIClient{c: c, bundledVersions: make(map[string]bundledVersion)}
}

func (a *APIClient) Version(ctx context.Context, vk VersionKey) (Version, error) {
	if isNPMBundle(vk.Name) {
		bv, ok := a.getBundledVersion(vk.Name)
		if !ok {
			return Version{}, fmt.Errorf("bundled version %v: %w", vk, ErrNotFound)
		}
		return bv.Version, nil
	}
	resp, err := a.c.GetVersion(ctx, &pb.GetVersionRequest{
		VersionKey: &pb.VersionKey{
			System:  pb.System(vk.System),
			Name:    vk.Name,
			Version: vk.Version,
		},
	})
	if status.Code(err) == codes.NotFound {
		return Version{}, fmt.Errorf("version %v: %w", vk, ErrNotFound)
	}
	if err != nil {
		return Version{}, err
	}

	if vk.System == Maven {
		// Fetch repositories and serve as dependency registries.
		reqResp, err := a.c.GetRequirements(ctx, &pb.GetRequirementsRequest{
			VersionKey: &pb.VersionKey{
				System:  pb.System_MAVEN,
				Name:    vk.Name,
				Version: vk.Version,
			},
		})
		if status.Code(err) == codes.NotFound {
			return Version{}, fmt.Errorf("requirements %v: %w", vk, ErrNotFound)
		}
		if err != nil {
			return Version{}, err
		}
		if reqResp.Maven != nil {
			for _, repo := range reqResp.Maven.Repositories {
				resp.Registries = append(resp.Registries, "dep:"+repo.Url)
			}
		}
	}
	// Use the VersionKey provided rather than the possibly canonicalized
	// name and version returned by the API in case the resolver needs to do
	// any direct comparisons.
	return makeVersion(vk, resp, strings.Join(resp.Registries, "|")), nil
}

func (a *APIClient) Versions(ctx context.Context, pk PackageKey) ([]Version, error) {
	if isNPMBundle(pk.Name) {
		bv, ok := a.getBundledVersion(pk.Name)
		if !ok {
			return nil, fmt.Errorf("bundled package %v: %w", pk, ErrNotFound)
		}
		return []Version{bv.Version}, nil
	}
	resp, err := a.c.GetPackage(ctx, &pb.GetPackageRequest{
		PackageKey: &pb.PackageKey{
			System: pb.System(pk.System),
			Name:   pk.Name,
		},
	})
	if status.Code(err) == codes.NotFound {
		return nil, fmt.Errorf("package %v: %w", pk, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	vers := make([]Version, len(resp.Versions))
	for i, v := range resp.Versions {
		// Use the name provided, let the resolver decide whether
		// canonicalization matters.
		vers[i] = makeVersion(VersionKey{
			PackageKey:  pk,
			VersionType: Concrete,
			Version:     v.VersionKey.Version,
		}, v, "")
	}
	return vers, nil
}

func (a *APIClient) Requirements(ctx context.Context, vk VersionKey) ([]RequirementVersion, error) {
	if isNPMBundle(vk.Name) {
		bv, ok := a.getBundledVersion(vk.Name)
		if !ok {
			return nil, fmt.Errorf("bundled version %v: %w", vk, ErrNotFound)
		}
		return bv.requirements, nil
	}
	resp, err := a.c.GetRequirements(ctx, &pb.GetRequirementsRequest{
		VersionKey: &pb.VersionKey{
			System:  pb.System(vk.System),
			Name:    vk.Name,
			Version: vk.Version,
		},
	})
	if status.Code(err) == codes.NotFound {
		return nil, fmt.Errorf("version %v: %w", vk, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}

	switch vk.System {
	case Maven:
		return a.mavenRequirements(ctx, vk, resp.Maven)
	case NPM:
		return a.npmRequirements(vk, resp.Npm)
	}
	return nil, errors.New("unsupported system")
}

func (a *APIClient) MatchingVersions(ctx context.Context, vk VersionKey) ([]Version, error) {
	if isNPMBundle(vk.Name) {
		bv, ok := a.getBundledVersion(vk.Name)
		if !ok {
			return nil, fmt.Errorf("bundled version %v: %w", vk, ErrNotFound)
		}
		// Surprising, because there should only be one version of each
		// bundled package, but clearly doesn't match.
		if bv.Version.Version != vk.Version {
			return nil, nil
		}
		return []Version{bv.Version}, nil
	}
	vers, err := a.Versions(ctx, vk.PackageKey)
	if err != nil {
		return nil, err
	}
	return MatchRequirement(vk, vers), nil
}

func (a *APIClient) getBundledVersion(name string) (bundledVersion, bool) {
	a.bundledVersionsMu.Lock()
	defer a.bundledVersionsMu.Unlock()
	bv, ok := a.bundledVersions[name]
	return bv, ok
}

func (a *APIClient) npmRequirements(root VersionKey, reqs *pb.Requirements_NPM) ([]RequirementVersion, error) {
	rootDeps := flattenNPMDeps(reqs.Dependencies)
	// Generate fake packages/versions for anything bundled by this version
	// and add them to the dependencies with the mangled names expected by
	// the resolver.
	type bundle struct {
		vk           VersionKey
		originalName string
		deps         []RequirementVersion
	}
	allDeps := map[string]bundle{
		root.Name: {vk: root, deps: rootDeps},
	}
	// Sort by the length of the path, so that we're guaranteed to process
	// bundles closer to the root before their nested bundles.
	sort.Slice(reqs.Bundled, func(i, j int) bool {
		return len(reqs.Bundled[i].Path) < len(reqs.Bundled[j].Path)
	})
	for _, b := range reqs.Bundled {
		bundleDeps := flattenNPMDeps(b.Dependencies)
		// For a package "b" bundled by package "a" which is itself
		// bundled by root, the path will be
		// "node_modules/a/node_modules/b".
		pkgs := strings.Split(strings.TrimPrefix(b.Path, "node_modules/"), "/node_modules/")
		mangled := mangledName(root, pkgs)
		// Add a single Concrete version, and a Requirement version
		// that matches.
		bundleVK := VersionKey{
			PackageKey: PackageKey{
				System: NPM,
				Name:   mangled,
			},
			VersionType: Concrete,
			Version:     b.Version,
		}
		allDeps[mangled] = bundle{
			vk:           bundleVK,
			originalName: b.Name,
			deps:         bundleDeps,
		}
		// Add this to the dependencies of the bundled package
		// immediately preceding it (which could be the root).
		parentName := root.Name
		if i := len(pkgs) - 1; i > 0 {
			parentName = mangledName(root, pkgs[:i])
		}
		parentBundle, ok := allDeps[parentName]
		if !ok {
			return nil, fmt.Errorf("internal error: missing bundle parent for %s", mangled)
		}
		parentBundle.deps = append(parentBundle.deps, RequirementVersion{
			VersionKey: VersionKey{
				PackageKey:  bundleVK.PackageKey,
				VersionType: Requirement,
				Version:     b.Version,
			},
			Type: dep.NewType(),
		})
		allDeps[parentName] = parentBundle
	}
	// Add all of the new bundles
	a.bundledVersionsMu.Lock()
	defer a.bundledVersionsMu.Unlock()
	for name, bundle := range allDeps {
		if name == root.Name {
			// This is not a bundled version, we don't need to store
			// it.
			continue
		}
		v := Version{VersionKey: bundle.vk}
		v.SetAttr(version.DerivedFrom, bundle.originalName)
		a.bundledVersions[name] = bundledVersion{
			Version:      v,
			requirements: bundle.deps,
		}
	}
	return allDeps[root.Name].deps, nil
}

func flattenNPMDeps(deps *pb.Requirements_NPM_Dependencies) []RequirementVersion {
	var flattened []RequirementVersion
	addDeps := func(ds []*pb.Requirements_NPM_Dependencies_Dependency, t dep.Type) {
		for _, d := range ds {
			typ := t.Clone()
			name, req := d.Name, d.Requirement
			if r, ok := strings.CutPrefix(d.Requirement, "npm:"); ok {
				// This is an aliased dependency, add it as a
				// dependency on the actual name and keep the
				// alias in the KnownAs attribute.
				typ.AddAttr(dep.KnownAs, d.Name)
				if i := strings.LastIndex(r, "@"); i >= 0 {
					name = r[:i]
					req = r[i+1:]
				}
			}
			flattened = append(flattened, RequirementVersion{
				VersionKey: VersionKey{
					PackageKey: PackageKey{
						System: NPM,
						Name:   name,
					},
					VersionType: Requirement,
					Version:     req,
				},
				Type: typ,
			})
		}
	}
	addDeps(deps.GetDependencies(), dep.NewType())
	addDeps(deps.GetDevDependencies(), dep.NewType(dep.Dev))
	addDeps(deps.GetOptionalDependencies(), dep.NewType(dep.Opt))

	peerType := dep.NewType()
	peerType.AddAttr(dep.Scope, "peer")
	addDeps(deps.GetPeerDependencies(), peerType)

	// The resolver expects bundleDependencies to be present as regular
	// dependencies with a "*" version specifier, even if they were already
	// in the regular dependencies.
	bundleType := dep.NewType()
	bundleType.AddAttr(dep.Scope, "bundle")
	for _, name := range deps.GetBundleDependencies() {
		flattened = append(flattened, RequirementVersion{
			VersionKey: VersionKey{
				PackageKey: PackageKey{
					System: NPM,
					Name:   name,
				},
				VersionType: Requirement,
				Version:     "*",
			},
			Type: bundleType,
		})
	}
	SortDependencies(flattened)
	return flattened
}

// defaultGetter matches pb.Version and pb.Package_Version, with getters for the
// values we need to construct version attributes.
type defaultGetter interface {
	GetIsDefault() bool
}

func makeVersion(vk VersionKey, d defaultGetter, regs string) Version {
	var attr version.AttrSet
	if vk.System == NPM && d.GetIsDefault() {
		// For NPM, the "default" version is either the highest by
		// semver or the version with a "latest" dist-tag.
		attr.SetAttr(version.Tags, "latest")
	}
	if regs != "" {
		attr.SetAttr(version.Registries, regs)
	}
	return Version{VersionKey: vk, AttrSet: attr}
}

// isNPMBundle returns whether the provided package name is a mangled
// name for a bundled package/version.
func isNPMBundle(name string) bool {
	return strings.Contains(name, ">")
}

// mangledName produces a name of the kind expected by the npm resolver for a
// bundled version of a package. The name includes the name and version of the
// bundling package, and any other package names extracted from the path to the
// ultimate bundled package. For example, package "a" version "1.0.0" that
// bundles package "b" would produce "a>1.0.0>b". If the bundled package "b" had
// another package "c" in its node_modules folder, "c"'s mangled name would be
// "a>1.0.0>b>c". The npm resolver uses this name to reconstruct the path to the
// package, which is why it needs to contain all of the preceding package names
// in order.
func mangledName(root VersionKey, pkgs []string) string {
	return fmt.Sprintf("%s>%s>%s", root.Name, root.Version, strings.Join(pkgs, ">"))
}
