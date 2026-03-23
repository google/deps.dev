// Package git provides utilities for working with git repositories.
package git

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var validGitRemoteSchemes = []string{
	"http",
	"https",
	"ftp",
	"ftps",
	"ssh",
	"git",
}

var schemeRequiresPath = []string{
	"ssh",
	"git",
}

// ErrInvalidRepo indicates that the provided string is not a valid git
// repository name or URL.
var ErrInvalidRepo = errors.New("invalid git repository")

// schemeRegexp matches schemes the same way git does in is_urlschemechar(). It
// is more permissive than RFC3986 as it can start with digits.
var schemeRegexp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+.-]*`)

// gitRemoteHelper matches any custom transport using gitremote-helpers via the
// <transport>::<address> syntax. It returns the transport name and true if a
// match is found.
func gitRemoteHelper(name string) (string, bool) {
	loc := schemeRegexp.FindStringIndex(name)
	if loc != nil && loc[0] == 0 && loc[1]+1 < len(name) && name[loc[1]] == ':' && name[loc[1]+1] == ':' {
		return name[:loc[1]], true
	}
	return "", false
}

// isGitURL checks if the given string is a URL according to the same logic
// used by the git source code in url.c.
func isGitURL(url string) bool {
	if url == "" {
		// An empty string is not a git URL.
		return false
	}

	loc := schemeRegexp.FindStringIndex(url)
	if loc == nil || loc[0] != 0 || loc[1] == 0 {
		// Either there was no scheme found, or the location of the scheme is not
		// anchored to the beginning of the URL.
		return false
	}

	// Check if there is a '://' immediately following the scheme.
	return loc[1]+2 < len(url) && url[loc[1]] == ':' && url[loc[1]+1] == '/' && url[loc[1]+2] == '/'
}

// isSCP checks if a url is SCP-like by checking if the colon appears before any
// slash. This mimics the logic of the git source code in connect.c, but
// without the local file checking.
func isSCP(url string) bool {
	colonPos := strings.Index(url, ":")
	slashPos := strings.Index(url, "/")
	return colonPos > 0 && (slashPos < 0 || colonPos < slashPos)
}

// parseSCP parses an SCP-style git remote name into a URL. It is intended to be
// used by ParseRemote after isSCP has returned true.
// The logic here is based off the git source code in connect.c.
func parseSCP(name string) (*url.URL, error) {
	// Handle the case where the host is an IPv6 address. These are surrounded in
	// square brackets. There is also a possibility that there is a user and an
	// "@" before the IPv6 address that needs to be accounted for.
	index := 0
	if bracketOpenPos := strings.Index(name, "@["); bracketOpenPos >= 0 {
		// Jump over the "@" to get to the start of the IPv6 address.
		index = bracketOpenPos + 1
	}
	if name[index] == '[' {
		// We have found an IPv6 address. Find the closing bracket.
		bracketClosedOff := strings.IndexRune(name[index:], ']')
		if bracketClosedOff >= 0 {
			// Found the closing bracket, so move the index past it.
			index += bracketClosedOff + 1
		}
	}
	// Now we can safely hunt for the colon separating the user+host from the
	// path. SCP URLs do not have ports, so we do not have to worry about them.
	colonPos := strings.IndexRune(name[index:], ':')
	if colonPos < 0 {
		return nil, fmt.Errorf("no colon in %q", name)
	}
	// Adjust the colon position to be relative to the start of the string.
	index += colonPos

	userHost := name[:index]
	path := name[index+1:]

	if len(path) > 0 && path[0] == '/' {
		// Remove preceeding slash so that we don't end up with a double slash.
		path = path[1:]
	}

	// Formulate a text URL we can parse.
	urlStr := fmt.Sprintf("ssh://%s/%s", userHost, path)

	return url.Parse(urlStr)
}

// parse parses a git repository name and returns a URL.
//
// Both URL and SCP-like remote names are supported.
//
// Note that this function is intended to be used for processing open source git
// repository data, not for private git repositories or git repositories that
// are intended to be used.
//
// Custom gitremote-helpers are explicitly rejected. Local file paths and
// bundles are also rejected.
func parse(name string) (*url.URL, error) {
	// Reject any custom gitremote-helpers (https://git-scm.com/docs/gitremote-helpers)
	// following the same logic as the git source code to ensure the same behavior.
	if helper, ok := gitRemoteHelper(name); ok {
		return nil, fmt.Errorf("%w: custom transport %q", ErrInvalidRepo, helper)
	}

	var u *url.URL
	var err error

	if isGitURL(name) {
		u, err = url.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("%w: url parsing: %w", ErrInvalidRepo, err)
		}
	} else if isSCP(name) {
		u, err = parseSCP(name)
		if err != nil {
			return nil, fmt.Errorf("%w: scp parsing: %w", ErrInvalidRepo, err)
		}
	} else {
		return nil, fmt.Errorf("%w: unable to parse %q", ErrInvalidRepo, name)
	}

	// Extract the scheme so we can manipulate it before validation, but we
	// preserve the original as we are not canonicalizing the URL here.
	scheme := u.Scheme

	// Remove any deprecated or unnecessary suffix or prefix, as these can be added
	// from package manifests, or referencing the deprecated "git+ssh" or "ssh+git"
	// schemes.
	if strings.HasSuffix(scheme, "+git") {
		scheme = scheme[:len(scheme)-4]
	} else if strings.HasPrefix(scheme, "git+") {
		scheme = scheme[4:]
	}

	// Explicitly reject the file scheme to ensure the error message is useful.
	if scheme == "file" {
		return nil, fmt.Errorf("%w: file scheme not supported", ErrInvalidRepo)
	}

	// Validate the scheme is a scheme that is supported natively by git. Open
	// source repositories should not be using custom gitremote-helpers.
	if !slices.Contains(validGitRemoteSchemes, scheme) {
		return nil, fmt.Errorf("%w: custom transport %q", ErrInvalidRepo, u.Scheme)
	}

	// For schemes the require a path ensure they have a path component.
	if slices.Contains(schemeRequiresPath, scheme) && u.Path == "" {
		return nil, fmt.Errorf("%w: %q scheme requires a path", ErrInvalidRepo, u.Scheme)
	}

	// For the git scheme ensure there is no user info, as it is an unauthenticated
	// transport.
	if scheme == "git" && u.User != nil {
		return nil, fmt.Errorf("%w: git scheme has authentication", ErrInvalidRepo)
	}

	// Ensure the host is set. It is the only required component of a git remote name.
	if u.Host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidRepo)
	}

	return u, nil
}
