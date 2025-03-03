// Copyright 2025 Google LLC
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

package pypi

import "deps.dev/util/resolve"

// versionMap is a map from resolve.PackageKeys to resolve.Versions that also
// allows constant time access to the most recently inserted key.
type versionMap struct {
	m     map[resolve.PackageKey]resolve.VersionKey
	stack []resolve.PackageKey // stack tracks the insertion order of the map keys.
}

// newVersionMap returns a new, empty versionMap with the specified capacity.
func newVersionMap(capacity int) *versionMap {
	return &versionMap{
		m:     make(map[resolve.PackageKey]resolve.VersionKey, capacity),
		stack: make([]resolve.PackageKey, 0, capacity),
	}
}

// Len returns the number of elements in the map.
func (v *versionMap) Len() int {
	return len(v.m)
}

// Get retrieves a value from the map.
func (v *versionMap) Get(pkg resolve.PackageKey) (resolve.VersionKey, bool) {
	ver, ok := v.m[pkg]
	return ver, ok
}

// Set puts a package/version pair into the map. If the key is already present
// it is treated as newly added.
func (v *versionMap) Set(pkg resolve.PackageKey, version resolve.VersionKey) {
	// TODO: we could avoid this scan if the key is new.
	for i, p := range v.stack {
		if p == pkg {
			v.stack = append(v.stack[:i], v.stack[i+1:]...)
			break
		}
	}
	v.m[pkg] = version
	v.stack = append(v.stack, pkg)
}

// Pop removes the most recently inserted key pair and returns it.
func (v *versionMap) Pop() (resolve.PackageKey, resolve.VersionKey) {
	if len(v.stack) == 0 {
		return resolve.PackageKey{}, resolve.VersionKey{}
	}
	pkg := v.stack[len(v.stack)-1]
	version := v.m[pkg]
	delete(v.m, pkg)
	v.stack = v.stack[:len(v.stack)-1]
	return pkg, version
}

// Iterate applies the provided function to all pairs in the map in the order
// they were inserted.
func (v *versionMap) Iterate(f func(resolve.PackageKey, resolve.VersionKey)) {
	for _, pkg := range v.stack {
		f(pkg, v.m[pkg])
	}
}

// Clone makes a copy of the map with the same contents and insertion order.
func (v *versionMap) Clone() *versionMap {
	w := &versionMap{
		m:     make(map[resolve.PackageKey]resolve.VersionKey, v.Len()),
		stack: append([]resolve.PackageKey(nil), v.stack...),
	}
	for pkg, ver := range v.m {
		w.m[pkg] = ver
	}
	return w
}
