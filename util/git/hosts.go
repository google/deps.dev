package git

import (
	"fmt"
	"math"
	"net/url"
	"strings"
)

// HostHandler defines the interface for host-specific validation and
// canonicalization of URLs.
type HostHandler interface {
	// Validate checks if the URL is valid for this host.
	Validate(u *url.URL) error

	// Canon canonicalizes the URL for this host.
	Canon(u *url.URL) *url.URL
}

var hostHandlers []struct {
	exactHost string
	handler   HostHandler
}

// RegisterHostHandler allows for the registration of a host handler
// that is triggered when the host of a URL exactly matches the given host,
// ignoring case.
//
// This method is not thread-safe, and is expected to be called from init().
func RegisterHostHandler(exactHost string, handler HostHandler) {
	hostHandlers = append(hostHandlers, struct {
		exactHost string
		handler   HostHandler
	}{
		exactHost: exactHost,
		handler:   handler,
	})
}

// getHostHandler returns the registered handler for the host.
func getHostHandler(host string) HostHandler {
	host = strings.ToLower(host)
	for _, h := range hostHandlers {
		if h.exactHost == host {
			return h.handler
		}
	}
	return noop
}

// noopHostHandler is a HostHandler that does nothing.
type noopHostHandler struct{}

func (h *noopHostHandler) Validate(u *url.URL) error {
	return nil
}

func (h *noopHostHandler) Canon(u *url.URL) *url.URL {
	return u
}

var noop = &noopHostHandler{}

// StandardHostHandler implements HostHandler with common validation and
// canonicalization settings.
type StandardHostHandler struct {
	// ForceScheme will replace any URL scheme with its value when set.
	ForceScheme string

	// StripUser will remove the user from the URL when true.
	StripUser bool

	// HasTrailingSlash will add a trailing slash to the URL if it doesn't have
	// one. If false any trailing slash will be removed.
	HasTrailingSlash bool

	// HasDotGitSuffix will add a .git suffix to the URL if it doesn't have one.
	// If false any .git suffix will be removed.
	HasDotGitSuffix bool

	// PathPrefix is the prefix that is required to be set for a valid git
	// repository for this host.
	PathPrefix string

	// MinPathSegments is the minimum number of path segments that are required for a
	// valid git repository for this host. If 0 there is no restriction. If PathPrefix
	// is not empty, then only segments after the prefix are considered.
	MinPathSegments int

	// MaxPathSegments is the maximum number of path segments that are required for a
	// valid git repository for this host. If 0 there is no restriction. If PathPrefix
	// is not empty, then only segments after the prefix are considered.
	MaxPathSegments int

	// LowerPathSegments is the number of path segments that should be lowercased
	// following any PathPrefix, during the canonicalization process. This is to
	// ensure that URLs for case-insensitive hosts are canonicalized correctly.
	//
	// A zero value means no path segements will be lowercased.
	// A positive value of N means the first N path segments following the
	// PathPrefix will be lowercased.
	// A negative value of -N means all but the last N path segments following
	// the PathPrefix will be lowercased.
	LowerPathSegments int
}

// Validate implements the HostHandler interface.
func (h *StandardHostHandler) Validate(u *url.URL) error {
	path := strings.TrimRight(u.Path, "/")
	if h.PathPrefix != "" {
		prefix := "/" + strings.Trim(h.PathPrefix, "/")
		if !strings.HasPrefix(path, prefix) {
			return fmt.Errorf("invalid path prefix: must start with %q", h.PathPrefix)
		}
		path = strings.TrimPrefix(path, prefix)
	}

	// Any slashes at the end of the path are stripped above, we strip any at the
	// start so that we can count the remaining path segments correctly.
	path = strings.TrimLeft(path, "/")
	var segments []string
	if path != "" {
		segments = strings.Split(path, "/")
	}

	if h.MinPathSegments > 0 && len(segments) < h.MinPathSegments {
		return fmt.Errorf("incorrect number of path segments: got %d, want at least %d", len(segments), h.MinPathSegments)
	}
	if h.MaxPathSegments > 0 && len(segments) > h.MaxPathSegments {
		return fmt.Errorf("incorrect number of path segments: got %d, want at most %d", len(segments), h.MaxPathSegments)
	}
	return nil
}

// Canon implements the HostHandler interface.
func (h *StandardHostHandler) Canon(u *url.URL) *url.URL {
	res := *u // shallow copy

	if h.ForceScheme != "" {
		res.Scheme = h.ForceScheme
	}
	if h.StripUser {
		res.User = nil
	}

	if h.LowerPathSegments != 0 {
		// Strip the prefix and trailing slash so that we can count the remaining path segments correctly.
		path := strings.TrimRight(res.Path, "/")
		prefix := ""
		if h.PathPrefix != "" {
			prefix = "/" + strings.Trim(h.PathPrefix, "/")
			path = strings.TrimPrefix(path, prefix)
		}

		// Split the remaining path into segments.
		segments := strings.Split(strings.TrimLeft(path, "/"), "/")
		toLower := h.LowerPathSegments
		if toLower < 0 {
			// Convert the negative number to the number of segments to lowercase.
			// For example, -1 means all but the last segment, which is equivalent to
			// len(segments) - 1.
			toLower = len(segments) + h.LowerPathSegments
			if toLower < 0 {
				// Ensure the number of segements to lower is not negative.
				toLower = 0
			}
		}

		// Lowercase the first N segments.
		for i := 0; i < len(segments) && i < toLower; i++ {
			segments[i] = strings.ToLower(segments[i])
		}

		// Reconstruct the path with the prefix and the lowercased segments.
		res.Path = prefix + "/" + strings.Join(segments, "/")
	}

	if h.HasDotGitSuffix {
		if !strings.HasSuffix(res.Path, ".git") {
			res.Path += ".git"
		}
	} else {
		res.Path = strings.TrimSuffix(res.Path, ".git")
	}

	if h.HasTrailingSlash {
		if !strings.HasSuffix(res.Path, "/") {
			res.Path += "/"
		}
	} else {
		res.Path = strings.TrimSuffix(res.Path, "/")
	}

	return &res
}

var defaultHostHandler = &StandardHostHandler{
	ForceScheme:       "https",
	StripUser:         true,
	HasDotGitSuffix:   true,
	LowerPathSegments: 2,
	MinPathSegments:   2,
	MaxPathSegments:   2,
}

func init() {
	// Register default handlers for well-known hosts.
	RegisterHostHandler("github.com", defaultHostHandler)
	RegisterHostHandler("bitbucket.org", defaultHostHandler)

	RegisterHostHandler("gitlab.com", &StandardHostHandler{
		ForceScheme:       "https",
		StripUser:         true,
		HasDotGitSuffix:   true,
		LowerPathSegments: math.MaxInt, // All path segments are lowercased.
		MinPathSegments:   2,
	})

	RegisterHostHandler("gitee.com", &StandardHostHandler{
		ForceScheme:       "https",
		StripUser:         true,
		HasDotGitSuffix:   true,
		LowerPathSegments: 1,
		MinPathSegments:   2,
		MaxPathSegments:   2,
	})
	RegisterHostHandler("gitee.cn", &StandardHostHandler{
		ForceScheme:       "https",
		StripUser:         true,
		HasDotGitSuffix:   true,
		LowerPathSegments: 1,
		MinPathSegments:   2,
		MaxPathSegments:   2,
	})
}
