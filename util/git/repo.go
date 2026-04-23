package git

import (
	"fmt"
	"net/url"
	"strings"
)

// Repo represents a parsed and canonicalized git repository.
type Repo struct {
	raw       string
	parsed    *url.URL
	canonical *url.URL
}

// ParseRepo parses a git repository URL or SCP-like string and returns a Repo.
//
// It validates the repository against registered host handlers and computes a
// canonical URL.
//
// If the repository is not valid, an error is returned.
func ParseRepo(name string) (*Repo, error) {
	parsed, err := parse(name)
	if err != nil {
		return nil, err
	}

	h := getHostHandler(parsed.Hostname())

	// Carry out any host-based validation.
	if err := h.Validate(parsed); err != nil {
		return nil, fmt.Errorf("%w: host validation: %w", ErrInvalidRepo, err)
	}

	return &Repo{
		raw:       name,
		parsed:    parsed,
		canonical: h.Canon(canon(parsed)),
	}, nil
}

// Canon returns the canonicalized URL of the repository.
func (r *Repo) Canon() *url.URL {
	return r.canonical
}

// Parsed returns the parsed URL of the repository (preserving case and
// structure from parsing).
func (r *Repo) Parsed() *url.URL {
	return r.parsed
}

// Raw returns the original raw string used to parse the repository.
func (r *Repo) Raw() string {
	return r.raw
}

// ID returns a string that can be used to identify the repository.
//
// The ID is based on the canonical URL, but is made up of the host and path
// only.
// The ID is unsuitable for interacting with the repository.
func (r *Repo) ID() string {
	u := *(r.canonical) // Shallow copy

	host := u.Hostname()
	if strings.ContainsRune(host, ':') {
		// Add square brackets around IPv6 addresses.
		host = "[" + host + "]"
	}

	path := strings.TrimSuffix(u.EscapedPath(), ".git")
	path = strings.TrimSuffix(path, "/")

	return host + path
}
