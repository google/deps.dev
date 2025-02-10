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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
	"unicode/utf8"

	"deps.dev/util/semver"
)

// Metadata holds metadata for a distribution as defined in
// https://packaging.python.org/specifications/core-metadata/.
type Metadata struct {
	// Name and Version are the only fields required by the spec.
	// Taken directly from Metadata and not canonicalized.
	Name, Version string

	// Optional metadata as defined by the spec.
	Summary         string
	Description     string
	Homepage        string
	Author          string
	AuthorEmail     string
	Maintainer      string
	MaintainerEmail string
	License         string
	Classifiers     []string
	ProjectURLs     []string

	Dependencies []Dependency
}

// ParseMetadata reads a METADATA or PKG-INFO file and collects as much
// information as possible. The earliest version of this format was a set of RFC
// 822 headers (see https://www.python.org/dev/peps/pep-0241/) with later
// versions (https://www.python.org/dev/peps/pep-0566/) adding the ability to
// include a message body rendering the format essentially the same as an email.
// The latest specification is here:
// https://packaging.python.org/en/latest/specifications/core-metadata/. For
// reference distlib, the library used by pip for this job, uses python's
// standard library email reader to read these files (see
// https://bitbucket.org/pypa/distlib/src/default/distlib/metadata.py). The
// current version of the specification requires metadata to be encoded as
// UTF-8, so an error will be returned if any invalid UTF-8 is discovered.
func ParseMetadata(ctx context.Context, data string) (Metadata, error) {
	if !utf8.ValidString(data) {
		// TODO: maybe we could be a bit more lenient to support
		// older packages.
		return Metadata{}, parseErrorf("invalid UTF-8")
	}
	// Add a newline to the end; some files have no body which is an error to
	// net/mail. Adding a newline ensures it will parse an empty body.
	buf := bytes.NewBufferString(data)
	buf.WriteByte('\n')
	msg, err := mail.ReadMessage(buf)
	if err != nil {
		return Metadata{}, parseErrorf("parsing python metadata: %v", err)
	}
	md := Metadata{}

	header := func(name string) (value string) {
		vs := msg.Header[name]
		if len(vs) > 1 {
			log.Printf("Header set multiple times: %q: %q", name, vs)
		}
		if len(vs) == 1 && vs[0] != "UNKNOWN" {
			value = vs[0]
		}
		return
	}
	multiHeader := func(name string) (values []string) {
		for _, v := range msg.Header[name] {
			if v != "UNKNOWN" {
				values = append(values, v)
			}
		}
		return
	}

	// Dependencies need some parsing and will always be needed.
	for _, d := range msg.Header["Requires-Dist"] {
		dep, err := ParseDependency(d)
		if err != nil {
			return Metadata{}, err
		}
		md.Dependencies = append(md.Dependencies, dep)
	}

	md.Name = header("Name")
	md.Version = header("Version")
	md.Summary = header("Summary")
	md.Description = header("Description")
	md.Homepage = header("Home-Page")
	md.Author = header("Author")
	md.AuthorEmail = header("Author-Email")
	md.Maintainer = header("Maintainer")
	md.MaintainerEmail = header("Maintainer-Email")
	md.License = header("License")
	md.ProjectURLs = multiHeader("Project-Url")
	md.Classifiers = multiHeader("Classifier")

	// The description may be in the message body.
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return Metadata{}, parseErrorf("reading metadata description: %v", err)
	}
	if len(body) > 0 {
		// Remove the extra line we added earlier to ensure a valid message.
		body = body[:len(body)-1]
		md.Description = string(body)
	}
	return md, nil
}

// Dependency is a dependency on a package.
type Dependency struct {
	Name        string
	Extras      string
	Constraint  string
	Environment string
}

// ParseDependency parses a python requirement statement according to PEP 508
// (https://www.python.org/dev/peps/pep-0508/), apart from URL requirements.
func ParseDependency(v string) (Dependency, error) {
	var d Dependency
	if v == "" {
		return d, parseErrorf("invalid python requirement: empty string")
	}
	const whitespace = " \t" // according to the PEP this is the only allowed whitespace
	s := strings.Trim(v, whitespace)
	// For our purposes, the name is some characters ending with space or the
	// start of something else.
	nameEnd := strings.IndexAny(s, whitespace+"[(;<=!~>")
	if nameEnd == 0 {
		return d, parseErrorf("invalid python requirement: empty name")
	}
	if nameEnd < 0 {
		d.Name = CanonPackageName(s)
		return d, nil
	}
	d.Name = CanonPackageName(s[:nameEnd])
	s = strings.TrimLeft(s[nameEnd:], whitespace)
	// Does it have extras?
	if s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return d, parseErrorf("invalid python requirement: %q has unterminated extras section", v)
		}
		// Extract whatever is inside the []
		d.Extras = strings.Trim(s[1:end], whitespace)
		s = s[end+1:]
	}
	// Does it have a constraint?
	if len(s) > 0 && s[0] != ';' {
		end := strings.IndexByte(s, ';')
		if end < 0 {
			end = len(s) // all of the remainder is the constraint
		}
		d.Constraint = strings.Trim(s[:end], whitespace)
		// May be parenthesized, we can remove those.
		if strings.HasPrefix(d.Constraint, "(") && strings.HasSuffix(d.Constraint, ")") {
			d.Constraint = d.Constraint[1 : len(d.Constraint)-1]
		}
		s = s[end:]
	}
	// Anything left must be a condition starting with ';'. Otherwise there should
	// be no way for s to be non-empty. If it is something's wrong, that's an
	// error.
	if len(s) > 0 && s[0] != ';' {
		return d, parseErrorf("invalid python requirement: internal parse error on %q", v)
	}
	if s != "" {
		d.Environment = strings.Trim(s[1:], whitespace) // s[1] == ';'
	}
	return d, nil
}

// CanonVersion canonicalizes a version string. If the version does not parse
// according to PEP 440 it is returned as-is.
func CanonVersion(ver string) string {
	v, err := semver.PyPI.Parse(ver)
	if err != nil {
		return ver
	}
	return v.Canon(true)
}

// CanonPackageName returns the canonical form of the given PyPI package name.
func CanonPackageName(name string) string {
	// https://github.com/pypa/pip/blob/20.0.2/src/pip/_vendor/packaging/utils.py
	// https://www.python.org/dev/peps/pep-0503/
	// Names may only be [-_.A-Za-z0-9].
	// Replace runs of [-_.] with a single "-", then lowercase everything.
	var out bytes.Buffer
	run := false // whether a run of [-_.] has started.
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
			out.WriteByte(c)
			run = false
		case 'A' <= c && c <= 'Z':
			out.WriteByte(c + ('a' - 'A'))
			run = false
		case c == '-' || c == '_' || c == '.':
			if !run {
				out.WriteByte('-')
			}
			run = true
		default:
			run = false
		}
	}
	return out.String()
}

// ParseError is returned when we encounter data that fails to parse.
type ParseError struct {
	msg string
}

func (p ParseError) Error() string {
	return p.msg
}

// parseErrorf constructs a pypiParseError with a formatted message.
func parseErrorf(format string, args ...any) ParseError {
	return ParseError{msg: fmt.Sprintf(format, args...)}
}

// UnsupportedError is an error used to indicate when we encounter types of
// packaging that we can not yet handle.
type UnsupportedError struct {
	msg         string
	packageType string
}

func (p UnsupportedError) Error() string {
	return fmt.Sprintf("%s: %s", p.packageType, p.msg)
}
