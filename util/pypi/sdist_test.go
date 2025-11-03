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
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"reflect"
	"sort"
	"testing"
	"time"
)

func tarfile(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	tfw := tar.NewWriter(&buf)
	for name, contents := range files {
		byteContents := []byte(contents)
		hdr := &tar.Header{
			Name:    name,
			Size:    int64(len(byteContents)),
			ModTime: time.Now(),
		}
		if err := tfw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tfw.Write(byteContents); err != nil {
			t.Fatal(err)
		}
	}
	if err := tfw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func targzfile(t *testing.T, files map[string]string) []byte {
	tf := tarfile(t, files)
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(tf); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipfile(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSdistMetadata(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		files       map[string]string
		want        *Metadata
		unsupported string
	}{
		{
			files: map[string]string{
				"test-1.1.1/":                       "",
				"test-1.1.1/file-to-ignore.txt":     "this is boring",
				"test-1.1.1/PKG-INFO":               numpyPkgInfoRaw,
				"test-1.1.1/test.egg-info/PKG-INFO": numbaMetadataRaw,
			},
			want: &numpyPkgInfo,
		},
		{
			files: map[string]string{
				"test-1.1.2/PKG-INFO":                   numpyPkgInfoRaw,
				"test-1.1.2/test.egg-info/requires.txt": "requirement-a\nrequirement-b\n",
			},
			want: &Metadata{
				Name:            numpyPkgInfo.Name,
				Version:         numpyPkgInfo.Version,
				Summary:         numpyPkgInfo.Summary,
				Description:     numpyPkgInfo.Description,
				Homepage:        numpyPkgInfo.Homepage,
				Author:          numpyPkgInfo.Author,
				AuthorEmail:     numpyPkgInfo.AuthorEmail,
				Maintainer:      numpyPkgInfo.Maintainer,
				MaintainerEmail: numpyPkgInfo.MaintainerEmail,
				License:         numpyPkgInfo.License,
				Classifiers:     numpyPkgInfo.Classifiers,
				ProjectURLs:     numpyPkgInfo.ProjectURLs,
				// requirements only in the
				// egg-info/requires.txt should be ignored.
				Dependencies: nil,
			},
		},
		// No PKG-INFO is an error
		{
			files: map[string]string{
				"test-1.1.1/METADATA":         numbaMetadataRaw,
				"test-1.1.1/setup.py":         "print('hello, test')",
				"test-1.1.1/test/__init__.py": "\n",
			},
			unsupported: "no PKG-INFO",
		},
		// Ensure cases that have dependencies that are not specified in a way we
		// understand but are otherwise valid give an appropriate error.
		{
			files: map[string]string{
				"test-1.1.3/PKG-INFO":  numpyPkgInfoRaw,
				"test-1.1.3/setup.cfg": "[options]\ninstall_requires = \n  requirement-a\n  requirement-b\n",
			},
			unsupported: "setup.cfg",
		},
		{
			files: map[string]string{
				"test-1.1.4/PKG-INFO": numpyPkgInfoRaw,
				"test-1.1.4/setup.py": "from setuptools import setup\n\nsetup(\n  install_requires=['requirement-a', 'requirement-b']\n  )\n",
			},
			unsupported: "setup.py",
		},
		{
			files: map[string]string{
				"double-a/PKG-INFO": numpyPkgInfoRaw,
				"double-b/PKG-INFO": numpyPkgInfoRaw,
			},
			unsupported: "multiple PKG-INFO",
		},
	}
	// tar.gz files
	for _, c := range cases {
		tf := targzfile(t, c.files)
		if c.unsupported != "" {
			unsupportedSdist(ctx, t, tf, "test-1.0.tar.gz", c.unsupported)
			continue
		}
		got, err := SdistMetadata(ctx, "test-0.0.1.tar.gz", bytes.NewBuffer(tf))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("sdist tar metadata: files:\n%+v\n got: %#v\nwant: %#v", c.files, got, c.want)
		}
	}
	// zip files
	for _, c := range cases {
		tf := zipfile(t, c.files)
		if c.unsupported != "" {
			unsupportedSdist(ctx, t, tf, "test-1.0.zip", c.unsupported)
			continue
		}
		got, err := SdistMetadata(ctx, "test-0.0.1.zip", bytes.NewBuffer(tf))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("sdist zip metadata: files:\n%+v\n got: %#v\nwant: %#v", c.files, got, c.want)
		}
	}
	// Unsupported formats.
	unsupportedSdist(ctx, t, []byte("this is a bz2"), "test-0.0.1.tar.bz2", "bz2 archive")
	unsupportedSdist(ctx, t, []byte("xz yay"), "test-0.0.1.tar.xz", "xz archive")
	unsupportedSdist(ctx, t, []byte("big z"), "test-0.0.1.tar.Z", "Z archive")
	// TODO: support the following, it is simpler than the tar.gz we do
	// already
	unsupportedSdist(ctx, t, []byte("raw tar"), "test-0.0.1.tar", "uncompressed tar")
}

func unsupportedSdist(ctx context.Context, t *testing.T, data []byte, name, msg string) {
	t.Helper()
	var uerr UnsupportedError
	if got, err := SdistMetadata(ctx, name, bytes.NewBuffer(data)); err == nil {
		t.Errorf("%s: want error from unsupported sdist format, got:\nmetadata:\n%+v", msg, got)
	} else if ok := errors.As(err, &uerr); !ok {
		t.Errorf("%s: want: pypiUnsupportedError, got: %T", msg, err)
	}
}
