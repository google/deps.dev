package git

import (
	"net/url"
	"strings"
)

// defaultPorts for supported schemes. These are published on
// https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xhtml
// and available in /etc/services.
var defaultPorts = map[string]string{
	"ftp":   "21",
	"ssh":   "22",
	"http":  "80",
	"https": "443",
	"ftps":  "990",
	"git":   "9418",
}

// canon returns a canonicalized URL for the given git repository URL.
//
// A shallow copy of the URL is made so that the original URL is not modified.
func canon(repo *url.URL) *url.URL {
	u := *repo // shallow copy

	// Ensure the host is lowercased.
	u.Host = strings.ToLower(u.Host)

	// Remove any deprecated git prefix or suffix.
	if strings.HasSuffix(u.Scheme, "+git") {
		u.Scheme = u.Scheme[:len(u.Scheme)-4]
	} else if strings.HasPrefix(u.Scheme, "git+") {
		u.Scheme = u.Scheme[4:]
	}

	// Strip the default port if it is present.
	if port, ok := defaultPorts[u.Scheme]; ok && u.Port() == port {
		// Remove the port from the URL. There should always be a colon if the
		// port is present.
		// We do this rather than u.Host = u.Hostname() because the latter will
		// remove the brackets from IPv6 addresses.
		lastColon := strings.LastIndex(u.Host, ":")
		if lastColon > 0 {
			u.Host = u.Host[:lastColon]
		}
	}

	// Always strip passwords if they are present. They should never really be
	// present for open source repositories.
	if _, ok := u.User.Password(); ok {
		u.User = url.User(u.User.Username())
	}

	return &u
}
