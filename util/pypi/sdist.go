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

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// SdistVersion attempts to extract the version from the name of an sdist file.
// The format of the names is not standardized, but it is a strong enough
// convention that pip relies on it (see
// https://github.com/pypa/pip/blob/0442875a68f19b0118b0b88c747bdaf6b24853ba/src/pip/_internal/index/package_finder.py#L978).
// The filenames are formatted <name>-<version>, where the name is not
// necessarily canonicalized. The returned version will be canonicalized if
// possible.
func SdistVersion(canonName, filename string) (string, string, error) {
	// Take every substring ending in "-" and see if it canonicalizes to the
	// name we are looking for.
	// Start by trimming the extension.
	nameVersion := strings.TrimSuffix(filename, filepath.Ext(filename))
	// .tar.gz sdists have two extensions, make sure to trim .tar.
	nameVersion = strings.TrimSuffix(nameVersion, ".tar")
	for i, r := range nameVersion {
		if r != '-' {
			continue
		}
		name := CanonPackageName(nameVersion[:i])
		if name == canonName {
			return nameVersion[:i], nameVersion[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid filename for package %q: %q", canonName, filename)
}

// Regular expression indicating a setup.py or setup.cfg specifies dependencies.
// There may be some false positives: a line could be commented out or not in
// the right place. There will be no false negatives; to specify dependencies
// there must be at least one match for this pattern.
var installRequiresPattern = regexp.MustCompile(`install_requires[ \t]*=`)

// SdistMetadata attempts to read metadata out of the supplied reader assuming
// it contains an sdist. The reader should be either a tar or a zip file,
// the extension of the supplied filename will be used to distinguish.
// Note that when the setup.py or setup.cfg holds dependencies, SdistMetadata
// returns an UnsupportedError and partial metadata results.
func SdistMetadata(ctx context.Context, fileName string, r io.Reader) (*Metadata, error) {
	// setupPy and setupCFG indicate whether we have found dependency information
	// in a setup.py or setup.cfg.
	setupPy, setupCFG := false, false
	var meta Metadata

	walkFn := func(name string, r io.Reader) error {
		_, name, ok := strings.Cut(name, "/")
		if !ok {
			return nil
		}
		if name == "setup.py" && !setupPy {
			setupPy = installRequiresPattern.MatchReader(bufio.NewReader(r))
			return nil
		}
		if name == "setup.cfg" && !setupCFG {
			setupCFG = installRequiresPattern.MatchReader(bufio.NewReader(r))
			return nil
		}
		if name != "PKG-INFO" {
			return nil
		}
		if meta.Name != "" {
			// Multiple top level PKG-INFO is only possible if the contains multiple
			// packages. This is invalid and therefore unsupported.
			return UnsupportedError{
				msg:         "multiple top level PKG-INFO",
				packageType: "sdist",
			}
		}
		contents, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		md, err := ParseMetadata(ctx, string(contents))
		if err != nil {
			return err
		}
		meta.Name = md.Name
		meta.Version = md.Version
		meta.Summary = md.Summary
		meta.Description = md.Description
		meta.Homepage = md.Homepage
		meta.Author = md.Author
		meta.AuthorEmail = md.AuthorEmail
		meta.Maintainer = md.Maintainer
		meta.MaintainerEmail = md.MaintainerEmail
		meta.License = md.License
		meta.Classifiers = md.Classifiers
		meta.ProjectURLs = md.ProjectURLs
		if len(meta.Dependencies) == 0 {
			meta.Dependencies = md.Dependencies
		}
		return nil
	}
	switch {
	case strings.HasSuffix(fileName, ".tar.gz"),
		strings.HasSuffix(fileName, ".tgz"):
		tgz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer tgz.Close()
		if err := walkTarFiles(tgz, walkFn); err != nil {
			return nil, err
		}
	case strings.HasSuffix(fileName, ".zip"):
		// TODO: try and avoid this.
		contents, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		if err := walkZipFiles(bytes.NewReader(contents), int64(len(contents)), walkFn); err != nil {
			return nil, err
		}
	default:
		return nil, UnsupportedError{
			msg:         fmt.Sprintf("unsupported sdist format: %s", fileName),
			packageType: "sdist",
		}
	}
	if meta.Name == "" {
		return nil, UnsupportedError{
			msg:         "no PKG-INFO",
			packageType: "sdist",
		}
	}
	if len(meta.Dependencies) == 0 {
		switch {
		// If we found no dependencies in PKG-INFO but saw an
		// install_requires line in a setup.py or setup.cfg file then
		// report and error; we can't handle those dependencies yet.
		case setupCFG:
			return &meta, UnsupportedError{
				msg:         "dependencies in setup.cfg, not in PKG-INFO",
				packageType: "sdist",
			}
		case setupPy:
			return &meta, UnsupportedError{
				msg:         "dependencies in setup.py, not in PKG-INFO",
				packageType: "sdist",
			}
		default:
			// It genuinely has no dependencies.
		}
	}
	return &meta, nil
}

// walkTarFiles walks through the files in a tar archive, applying the given
// function one at a time to the name of the file and a reader containing its
// contents until all files have been visited or the first error.
func walkTarFiles(r io.Reader, f func(string, io.Reader) error) error {
	tfr := tar.NewReader(r)
	for {
		h, err := tfr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if err := f(h.Name, tfr); err != nil {
			return err
		}
	}
	return nil
}
