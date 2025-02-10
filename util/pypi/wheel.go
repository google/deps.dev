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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// WheelInfo holds all of the information kept in the name of a wheel file.
type WheelInfo struct {
	Name      string
	Version   string
	BuildTag  WheelBuildTag
	Platforms []PEP425Tag
}

// WheelBuildTag holds the components of a wheel's build tag.
type WheelBuildTag struct {
	Num int
	Tag string
}

// PEP425Tag holds a compatibility tag defined in
// https://www.python.org/dev/peps/pep-0425/
type PEP425Tag struct {
	Python   string
	ABI      string
	Platform string
}

// ParseWheelName extracts all of the information in the name of a wheel. The
// wheel naming format is described in PEP 427
// (https://www.python.org/dev/peps/pep-0427/#file-name-convention). The name
// and version will always be canonicalized if possible.
func ParseWheelName(name string) (*WheelInfo, error) {
	if !strings.HasSuffix(name, ".whl") {
		return nil, fmt.Errorf("not a wheel filename: %q", name)
	}
	// Strip the suffix
	name = name[:len(name)-4]
	parts := strings.Split(name, "-")
	if len(parts) != 5 && len(parts) != 6 {
		return nil, fmt.Errorf("wheel name %q has %d elements, not 5 or 6", name, len(parts))
	}
	pwi := &WheelInfo{
		Name:    parts[0],
		Version: parts[1],
	}
	if len(parts) == 6 {
		buildTag := parts[2]
		split := strings.IndexFunc(buildTag, func(r rune) bool {
			return !unicode.IsDigit(r)
		})
		if split == 0 { // Must start with at least one digit.
			return nil, fmt.Errorf("invalid wheel name %q: build tag %q does not start with digit", name, buildTag)
		} else if split == -1 {
			split = len(buildTag)
		}
		num, err := strconv.Atoi(buildTag[:split])
		if err != nil {
			return nil, fmt.Errorf("invalid wheel name %q: %v", name, err)
		}
		pwi.BuildTag.Num = num
		pwi.BuildTag.Tag = buildTag[split:]
	}
	tag := PEP425Tag{
		Python:   parts[len(parts)-3],
		ABI:      parts[len(parts)-2],
		Platform: parts[len(parts)-1],
	}
	pwi.Platforms = expandPEP425Tag(tag)
	return pwi, nil
}

// WheelMetadata extracts the metadata from a wheel file. The file format is
// defined in PEP 427 (https://www.python.org/dev/peps/pep-0427/#file-format)
// and is relatively simple compared to sdists. In particular: wheels can not
// have a setup.py or setup.cfg and the metadata version must be 1.1 or greater.
// This means that the metadata definitely supports dependencies and there is
// nowhere else to specify them.
func WheelMetadata(ctx context.Context, r io.ReaderAt, size int64) (*Metadata, error) {
	var meta *Metadata
	err := walkZipFiles(r, size, func(name string, r io.Reader) error {
		// Metadata lives in <package-name>-<version>.dist-info/METADATA.
		dir, name, ok := strings.Cut(name, "/")
		if !ok {
			return nil
		}
		if !strings.HasSuffix(dir, ".dist-info") {
			return nil
		}
		if name != "METADATA" {
			return nil
		}
		if meta != nil {
			return UnsupportedError{
				msg:         "multiple METADATA files",
				packageType: "wheel",
			}
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		md, err := ParseMetadata(ctx, string(b))
		if err != nil {
			return err
		}
		meta = &md
		return nil
	})
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, UnsupportedError{
			msg:         "no METADATA file",
			packageType: "wheel",
		}
	}
	return meta, nil
}

// expandPEP425Tag expands any compressed tag sets in the given tag to produce
// the full set of supported systems. It uses the algorithm described in the PEP
// (https://www.python.org/dev/peps/pep-0425/#compressed-tag-sets). Note this
// can generate a fair number of impossible tags that are not supported by any
// actual Python implementation.
func expandPEP425Tag(tag PEP425Tag) []PEP425Tag {
	var allTags []PEP425Tag
	for _, py := range strings.Split(tag.Python, ".") {
		for _, abi := range strings.Split(tag.ABI, ".") {
			for _, plat := range strings.Split(tag.Platform, ".") {
				allTags = append(allTags, PEP425Tag{
					Python:   py,
					ABI:      abi,
					Platform: plat,
				})
			}
		}
	}
	return allTags
}

// walkZipFiles walks through the files in a zip archive, applying the given
// function one at a time to the name of the file and a reader containing its
// contents until all files have been visited or the first error. Unfortunately
// there is no clear way to avoid loading the whole file into memory; zip files
// store their file listings at the end so it is not necessarily possible to
// process them sequentially.
func walkZipFiles(r io.ReaderAt, size int64, callback func(string, io.Reader) error) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := callback(f.Name, rc); err != nil {
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}
