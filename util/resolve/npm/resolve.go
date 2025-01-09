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

/*
Package npm implements a resolver for NPM dependencies, based on npm version
6.14.12.
*/
package npm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/resolve/version"
	"deps.dev/util/semver"
)

const (
	debug = false
)

// resolver implements resolve.Resolver for NPM.
// Dependencies are resolved using the algorithm employed by "npm install",
// assuming a fresh installation: https://docs.npmjs.com/cli/install#algorithm.
// The resolution is tree-based and depends on the process order of the nodes.
// A queue maintains the node to be processed. It is initialized with the
// given version to resolve.
// The tree only contains concrete versions (in **/node_modules by npm).
// Take the last node of the queue (it is a DFS here):
// - Retrieve its regular dependencies (requirements) from the client.
// - In lexicographic order of name, for each dependency (it is a BFS):
//   - Look up in the tree (from current up to root) if a concrete version
//     resolving the requirement exists.
//   - If it exists, use it as the resolution. If the found concrete node has
//     not been yet analyzed move it to the end of the queue to have it
//     processed earlier than as scheduled by the BFS.
//   - If no concrete version resolves, create a tree node with the highest
//     semver that satisfies the requirement. Plug the node in the tree as
//     close to the root as possible under the constraint that no two children
//     of a given node can have the same name (the highest is to be a child
//     of the root). Put the created node at the end of the processing queue.
type resolver struct {
	client resolve.Client
}

// NewResolver creates a Resolver connected to the given client.
// It is safe for concurrent use.
func NewResolver(client resolve.Client) resolve.Resolver {
	return &resolver{client: client}
}

// treeNode is a node in the resolution tree.
// It contains only concrete versions and is rooted with the initial version to
// resolve.
type treeNode struct {
	// processed marks the node has being processed, i.e. its dependencies have
	// been retrieved and inserted (potentially non processed) in the tree.
	processed bool
	// ver is the concrete version represented by this node and is non
	// mangled. It may be zero in case of a bundled version that does not
	// exist elsewhere.
	ver resolve.Version
	// pkg is the package of this node and is non mangled. It is never zero,
	// even if the bundled version does not exist.
	pkg resolve.PackageKey
	// ideps are the imported dependencies of the version.
	ideps []resolve.RequirementVersion
	// parent is the node's parent in the tree.
	parent *treeNode
	// children are the tree nodes children of the node. The version keys are
	// concrete, such that children[k].vk = k. Note that the nodes are not
	// the resolution of the direct dependency of the node's version.
	// In nodejs, that would be the direct content of the node_modules folder.
	children map[resolve.PackageKey]*treeNode
	// alias are the tree nodes children of the node. This happens when a node
	// is installed as an alias, and not using its package name.
	alias map[string]*treeNode
	// protected are slots that cannot be used so they don't shadow an
	// installation that has been placed higher in the tree.
	protected map[resolve.PackageKey]bool
	// aliasProtected is similar to protected, but keyed by alias.
	aliasProtected map[string]bool
	// The NodeID of this version in a corresponding resolve.Graph. For all
	// nodes created from a bundle, it is 0 until the version is used (i.e.
	// is the resolution of a requirement). This is a way to detect unused
	// bundled versions.
	id resolve.NodeID
	// bundled holds the bundled version when the node is bundled. This is
	// where we find the mangled version and information on the origin.
	bundled *bundledVersion
}

// bundledVersion represents a bundled version.
type bundledVersion struct {
	// Version holds the derived package version. It is from a derived
	// package, thus with a mangled name.
	Version resolve.Version
	// alias holds the alias under which the version has been installed
	// in the bundle.
	alias string
	// derivedFromVersion holds the version from which the bundled version
	// is derived. It may not necessarily exist in the client.
	derivedFromVersion resolve.Version
	// derivedFromPackage holds the package from which the bundled version
	// is derived from. This is a non-mangled package, and should always
	// exist in the client..
	derivedFromPackage resolve.PackageKey
}

// Resolve resolves the transitive dependencies of the given NPM concrete version.
// It returns an error if the version is invalid.
// It internally creates a resolved tree, similar to the one produced by "npm
// install" as a hierarchy of node_modules folders.
func (r *resolver) Resolve(ctx context.Context, vk resolve.VersionKey) (*resolve.Graph, error) {
	if vk.System != resolve.NPM {
		return nil, fmt.Errorf("expected NPM version, got %q", vk)
	}
	if vk.VersionType != resolve.Concrete {
		return nil, fmt.Errorf("expected Concrete version, got %q", vk)
	}

	start := time.Now()
	g := &resolve.Graph{}

	v, err := r.client.Version(ctx, vk)
	if err != nil {
		return nil, err
	}
	root, err := r.newTreeNode(ctx, v)
	if err != nil {
		return nil, err
	}
	root.id = g.AddNode(vk)
	// The resolution does not start with an empty context, but with a context
	// seeded by the (potential) bundled versions that have been published in
	// the registry.
	if err := r.injectDerivedFrom(ctx, root, root.ver); err != nil {
		return nil, fmt.Errorf("inject derived from for %v: %w", vk, err)
	}
	queue := []*treeNode{root}
	var insQueue []*treeNode
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		last := len(queue) - 1
		var cur *treeNode
		cur, queue = queue[last], queue[:last]
		if cur.processed {
			continue
		}
		cur.processed = true
		if debug {
			log.Printf("Current %s", r.treeNodeString(cur))
		}
		insQueue = insQueue[:0]
		// BFS in lexicographic order of the requirements.
		for _, idep := range cur.ideps {
			dvers, err := r.client.MatchingVersions(ctx, idep.VersionKey)
			if err != nil {
				return nil, fmt.Errorf("cannot find matching versions for %s: %w", idep.Version, err)
			}
			// wouldPick holds the version that would be picked if no dedup
			// occurs.
			var wouldPick resolve.Version
			if len(dvers) > 0 {
				wouldPick = dvers[len(dvers)-1]
			}
			if debug {
				if wouldPick.VersionKey == (resolve.VersionKey{}) {
					log.Printf("%s -> %s (%s), wouldPick: <nil>", r.treeNodeString(cur), idep, idep.Type)
				} else {
					log.Printf("%s -> %s (%s), wouldPick: %s", r.treeNodeString(cur), idep.Version, idep.Type, wouldPick)
				}
			}
			// Walk up the tree looking for one of the resolved concrete
			// versions; if one exists then we don't need to resolve it here.
			var resolved *treeNode
			installHere := false
			ipk := idep.PackageKey
			alias, _ := idep.Type.GetAttr(dep.KnownAs)
			for node := cur; node != nil; node = node.parent {
				child, unaliased := r.candidate(node, ipk, alias)
				if child == nil {
					continue
				}
				if unaliased {
					// Fast path, no need to fall back to
					// manual matching.
					for _, dver := range dvers {
						if child.ver.VersionKey == dver.VersionKey {
							resolved = child
							break
						}
					}
				} else {
					c, err := semver.NPM.ParseConstraint(idep.Version)
					if err != nil {
						return nil, fmt.Errorf("ParseConstraint %s: %w", idep.Version, err)
					}
					var cvk resolve.Version
					if child.ver.VersionKey != (resolve.VersionKey{}) {
						cvk = child.ver
					} else if child.bundled != nil {
						cvk = child.bundled.derivedFromVersion
					} else {
						return nil, errors.New("unknown child version")
					}
					if c.Match(cvk.Version) {
						resolved = child
					}
					break
				}
				// Star matches anything if it is already installed.
				if idep.Version == "*" {
					resolved = child
				}
				_, err := r.client.Version(ctx, child.ver.VersionKey)
				if err != nil && !errors.Is(err, resolve.ErrNotFound) {
					return nil, err
				}
				if err == nil || child.bundled == nil {
					break
				}
				// A bundled version that doesn't exist outside
				// the bundle.
				iver := idep.Version
				c, err := semver.NPM.ParseConstraint(iver)
				if err != nil {
					return nil, fmt.Errorf("ParseConstraint %s: %w", iver, err)
				}
				if c.Match(child.bundled.derivedFromVersion.Version) {
					resolved = child
					break
				}
				if node == cur {
					// Discard the child as it doesn't match and is at this
					// level so that it can be replaced at this level by a
					// matching version.
					if debug {
						log.Printf("delete bundled child, %s does not match %s", r.treeNodeString(child), iver)
					}
					delete(child.parent.children, child.bundled.derivedFromPackage)
					installHere = true
				}
				// If the package is found at this level stop here as the
				// installed version shadows anything higher up in the tree.
				break
			}
			if debug {
				if resolved == nil {
					log.Printf("not resolved")
				} else {
					log.Printf("resolved by %v", r.treeNodeString(resolved))
				}
			}
			if resolved != nil {
				if !resolved.processed {
					insQueue = append(insQueue, resolved)
				}
				// Mark protected all nodes between the current and where the
				// reused version lies.
				parent := cur
				for parent != nil {
					if c, _ := r.candidate(parent, ipk, alias); c != nil {
						break
					}
					if alias != "" {
						if parent.aliasProtected == nil {
							parent.aliasProtected = make(map[string]bool)
						}
						parent.aliasProtected[alias] = true
					} else {
						parent.protected[ipk] = true
					}
					parent = parent.parent
				}
				dt := idep.Type
				if resolved.id == 0 && resolved.parent != nil {
					resolved.id = g.AddNode(resolved.bundled.Version.VersionKey)
					if debug {
						log.Printf("Added node (resolved): %s", g.Nodes[resolved.id].Version)
					}
					dt = dt.Clone()
					dt.AddAttr(dep.Selector, "")
				}
				if err := g.AddEdge(cur.id, resolved.id, idep.Version, dt); err != nil {
					return nil, err
				}
				continue
			}
			// No matching concrete version for the requirement.
			if wouldPick.VersionKey == (resolve.VersionKey{}) {
				g.AddError(cur.id, idep.VersionKey, fmt.Sprintf("could not find a version that satisfies requirement %s for package %s", idep.Version, idep.Name))
				continue
			}

			// Nothing up in the tree resolves the requirement.
			// Select the highest non-deprecated concrete version, create a node
			// for it, and place it as high as possible in the tree (except if
			// this is the replacement of a mismatched bundled version, in which
			// case install at this level).
			latest := r.concreteForLatest(ctx, wouldPick)
			for i := len(dvers) - 1; i >= 0; i-- {
				v := dvers[i]
				if v.Equal(latest) {
					wouldPick = v
					break
				}
				if !v.HasAttr(version.Blocked) {
					wouldPick = v
					break
				}
			}
			node, err := r.newTreeNode(ctx, wouldPick)
			if err != nil {
				return nil, fmt.Errorf("cannot create tree node: %w", err)
			}
			// Inject the bundle in the tree node.
			if err := r.injectDerivedFrom(ctx, node, wouldPick); err != nil {
				return nil, fmt.Errorf("cannot inject derived from for %s: %w", wouldPick, err)
			}

			// Find parent for the new node.
			parent := cur
			if c, _ := r.candidate(parent, node.pkg, alias); c != nil {
				err := g.AddError(cur.id, idep.VersionKey,
					fmt.Sprintf("cannot install two versions of this package at the same level: %v (%s)", node.pkg, alias))
				if err != nil {
					return nil, err
				}
				continue
			}
			for !installHere && parent.parent != nil {
				if c, _ := r.candidate(parent.parent, node.pkg, alias); c != nil {
					break
				}
				if r.protected(parent.parent, node.pkg, alias) {
					break
				}
				parent.protected[node.pkg] = true
				parent = parent.parent
			}
			// If the parent and the installed version are from the same
			// package, the constraint will be unmatched as NPM will favor the
			// parent to the installed version in node_modules, and the
			// installed version marked extraneous. The name of the top level
			// package (the root) is not considered by NPM and it allows to
			// install one version in node_modules.
			if parent.parent != nil && parent.pkg == node.pkg {
				cvk := node.ver
				pvk := parent.ver
				err := g.AddError(cur.id, idep.VersionKey,
					fmt.Sprintf("unreachable version %s %s installed under %s %s", cvk.Name, cvk.Version, pvk.Name, pvk.Version))
				if err != nil {
					return nil, err
				}
				continue
			}
			if alias == "" {
				parent.children[node.pkg] = node
			} else {
				if parent.alias == nil {
					parent.alias = make(map[string]*treeNode)
				}
				parent.alias[alias] = node
			}
			node.parent = parent
			insQueue = append(insQueue, node)
			node.id = g.AddNode(node.ver.VersionKey)
			if debug {
				log.Printf("Added node (regular): %s", g.Nodes[node.id].Version)
			}
			dt := idep.Type.Clone()
			dt.AddAttr(dep.Selector, "")
			if err := g.AddEdge(cur.id, node.id, idep.Version, dt); err != nil {
				return nil, err
			}
		}
		// Reverse the insertion queue, to have a DFS in the transitive
		// resolution.
		for i := len(insQueue) - 1; i >= 0; i-- {
			queue = append(queue, insQueue[i])
		}
	}

	// Check that all tree nodes have a node id. Otherwise, this is an
	// extraneous version from a bundle and must be reported as an error.
	queue = queue[:0]
	queue = append(queue, root)
	var errs []string
	for len(queue) > 0 {
		cur := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if cur.id == 0 && cur.parent != nil {
			bvk := cur.bundled.derivedFromVersion
			errs = append(errs, fmt.Sprintf("unused bundled version %s %s", bvk.Name, bvk.Version))
			continue
		}
		for _, c := range cur.children {
			queue = append(queue, c)
		}
	}
	sort.Strings(errs)
	g.Error = strings.Join(errs, ",")

	if debug {
		log.Print(r.treeString(root, "", ""))
		log.Print(g.String())
	}

	g.Duration = time.Since(start)
	return g, nil
}

func (r *resolver) candidate(cur *treeNode, ipk resolve.PackageKey, alias string) (child *treeNode, unaliased bool) {
	if alias == "" {
		if child = cur.children[ipk]; child != nil {
			return child, true
		}
		return cur.alias[ipk.Name], false
	}
	if child = cur.alias[alias]; child != nil {
		return child, false
	}
	for p, c := range cur.children {
		if p.Name == alias {
			return c, false
		}
	}
	return nil, false
}

func (r *resolver) protected(cur *treeNode, ipk resolve.PackageKey, alias string) bool {
	if alias == "" {
		if cur.protected[ipk] {
			return true
		}
		return cur.aliasProtected[ipk.Name]
	}
	if cur.aliasProtected[alias] {
		return true
	}
	for p := range cur.protected {
		if p.Name == alias {
			return true
		}
	}
	return false
}

// newTreeNode creates a new treeNode holding the given version key.
func (r *resolver) newTreeNode(ctx context.Context, ver resolve.Version) (*treeNode, error) {
	n := &treeNode{
		ver:       ver,
		pkg:       ver.PackageKey,
		children:  map[resolve.PackageKey]*treeNode{},
		protected: map[resolve.PackageKey]bool{},
	}
	reqs, err := r.client.Requirements(ctx, ver.VersionKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get Requirements for %s: %w", ver, err)
	}
	n.ideps, err = r.regularImports(ctx, ver.VersionKey, reqs)
	if err != nil {
		return nil, fmt.Errorf("cannot process regularImports for %s: %w", ver, err)
	}
	if debug {
		log.Printf("newTreeNode %p for %s: ideps: %v", n, ver, n.ideps)
	}
	return n, nil
}

// regularImports returns the regular imports contained in the given imports.
// The returned dependencies must be resolved in order.
func (r *resolver) regularImports(ctx context.Context, ver resolve.VersionKey, imps []resolve.RequirementVersion) ([]resolve.RequirementVersion, error) {
	var (
		regPackage = make(map[string]bool)
		optPackage = make(map[string]bool)
		deps       = make([]resolve.RequirementVersion, 0, len(imps))
	)

	for _, d := range imps {
		if debug {
			log.Printf("%s regularImports %s %s", ver, d.Version, d.Type)
		}
		// Dependencies that are both Dev and Opt behave like Opt.
		if d.Type.HasAttr(dep.Dev) {
			continue
		}
		if d.Type.HasAttr(dep.Opt) {
			optPackage[d.Name] = true
		}
		if d.Type.IsRegular() {
			regPackage[d.Name] = true
		}
	}

	// Records regular and optional dependencies, skipping regulars that are
	// overwritten by optional ones and the bundled dependencies.
	for _, d := range imps {
		if d.Type.HasAttr(dep.Dev) {
			continue
		}
		if !d.Type.HasAttr(dep.Opt) && optPackage[d.Name] {
			continue
		}
		// If the requirement points to a derived package, the requirement
		// represents the direct content of the bundle and is not a regular
		// dependency to be resolved, therefore skip it.
		if bundled, err := r.getBundledVersion(ctx, d); err != nil {
			return nil, fmt.Errorf("bundledAs(%s): %w", d.Version, err)
		} else if bundled != nil {
			continue
		}

		// If the dependency is declared in bundleDependencies, treat it
		// as regular only if it is not also present in the regular
		// dependencies.
		switch scope, _ := d.Type.GetAttr(dep.Scope); scope {
		case "bundle":
			if regPackage[d.Name] {
				continue
			}
		case "peer":
			continue
		}

		deps = append(deps, d)
	}

	return deps, nil
}

// concreteForLatest returns the concrete version pointed by "latest", if it
// exists. It returns the zero version otherwise.
func (r *resolver) concreteForLatest(ctx context.Context, v resolve.Version) resolve.Version {
	vk := v.VersionKey
	vk.VersionType = resolve.Requirement
	vk.Version = "latest"
	latest, err := r.client.MatchingVersions(ctx, vk)
	if err != nil || len(latest) != 1 {
		return resolve.Version{}
	}
	return latest[0]
}

// injectDerivedFrom injects recursively the bundle content of the given version
// inside the given tree.
func (r *resolver) injectDerivedFrom(ctx context.Context, node *treeNode, v resolve.Version) error {
	if debug {
		log.Printf("--> injectDerivedFrom for %s", r.treeNodeString(node))
		defer log.Printf("<-- injectDerivedFrom for %s", r.treeNodeString(node))
	}
	bvs, err := r.directBundleContent(ctx, v)
	if err != nil {
		return fmt.Errorf("cannot get bundled content: %w", err)
	}
	for _, bv := range bvs {
		cn, err := r.newTreeNode(ctx, bv.Version)
		if err != nil {
			return fmt.Errorf("cannot create tree node for %v: %w", bv, err)
		}
		cn.parent = node
		cn.bundled = bv
		cn.ver = bv.derivedFromVersion
		cn.pkg = bv.derivedFromPackage
		if bv.alias == "" {
			node.children[cn.pkg] = cn
		} else {
			if node.alias == nil {
				node.alias = make(map[string]*treeNode)
			}
			node.alias[bv.alias] = cn
		}
		if err := r.injectDerivedFrom(ctx, cn, bv.Version); err != nil {
			return err
		}
	}
	return nil
}

// directBundleContent returns a map of mangled concrete to origin concrete (or
// origin requirement if the origin concrete is missing). It only returns the
// direct mangled dependencies and does not recurse.
func (r *resolver) directBundleContent(ctx context.Context, v resolve.Version) ([]*bundledVersion, error) {
	deps, err := r.client.Requirements(ctx, v.VersionKey)
	if err != nil {
		return nil, err
	}
	var bvs []*bundledVersion
	for _, d := range deps {
		bv, err := r.getBundledVersion(ctx, d)
		if err != nil {
			return nil, fmt.Errorf("getBundledVersion(%s): %w", d, err)
		}
		if bv == nil {
			continue
		}
		bvs = append(bvs, bv)
	}
	return bvs, nil
}

// getBundledVersion maps the mangled concrete version pointed by the given
// requirement to its origin concrete version, non mangled.
// When the given requirement is not mangled, getBundledVersion returns nil.
func (r *resolver) getBundledVersion(ctx context.Context, d resolve.RequirementVersion) (*bundledVersion, error) {
	// Bundled content dependencies are regular.
	if !d.Type.IsRegular() {
		return nil, nil
	}
	// Bundled content dependencies match directly a singled derived package
	// version.
	vs, err := r.client.MatchingVersions(ctx, d.VersionKey)
	if err != nil || len(vs) != 1 {
		return nil, nil
	}
	v := vs[0]
	name, ok := v.GetAttr(version.DerivedFrom)
	if !ok {
		return nil, nil
	}

	derivedFrom := resolve.Version{VersionKey: v.VersionKey, AttrSet: v.AttrSet.Clone()}
	derivedFrom.Name = name
	bv := &bundledVersion{
		Version:            v,
		derivedFromVersion: derivedFrom,
		derivedFromPackage: derivedFrom.PackageKey,
	}

	mangled := d.Name
	if alias := mangled[strings.LastIndex(mangled, ">")+1:]; alias != name {
		bv.alias = alias
	}

	return bv, nil
}

// treeNodeString returns a string describing the node, for debug purposes.
func (r *resolver) treeNodeString(n *treeNode, withDeps ...bool) string {
	var s []string
	if n.ver.VersionKey != (resolve.VersionKey{}) {
		s = append(s, n.ver.String())
	}
	if n.bundled != nil {
		s = append(s, fmt.Sprintf("(%s)", n.bundled.Version))
		if n.bundled.derivedFromVersion.VersionKey != (resolve.VersionKey{}) {
			s = append(s, fmt.Sprintf("(%s)", n.bundled.derivedFromVersion))
		}
		if withDeps != nil && withDeps[0] {
			s = append(s, fmt.Sprintf("ideps: %v", n.ideps))
		}
	}
	return strings.Join(s, " ")
}

// treeString prints the installation tree, for debug purposes.
func (r *resolver) treeString(node *treeNode, prefix string, pkg string) string {
	s := fmt.Sprintf("%s[%s] %s\n", prefix, pkg, r.treeNodeString(node))
	for p, c := range node.children {
		s += r.treeString(c, prefix+"    ", p.Name)
	}
	return s
}
