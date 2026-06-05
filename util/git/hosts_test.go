package git

import (
	"net/url"
	"testing"
)

func TestGetHostHandler(t *testing.T) {
	// Replace all the handlers so we can corrcetly test the noop behavior.
	oldHandlers := hostHandlers
	defer func() { hostHandlers = oldHandlers }()
	hostHandlers = nil

	h1 := &StandardHostHandler{}

	RegisterHostHandler("example.com", h1)

	tests := []struct {
		host string
		want HostHandler
	}{
		{"example.com", h1},
		{"foo.example.com", noop},
		{"other.com", noop},
	}

	for _, tt := range tests {
		got := getHostHandler(tt.host)
		if got != tt.want {
			t.Errorf("getHostHandler(%q) = %p; want %p", tt.host, got, tt.want)
		}
	}
}

func TestStandardHostHandler_Validate(t *testing.T) {
	tests := []struct {
		name    string
		handler StandardHostHandler
		url     string
		wantErr bool
	}{
		{
			name:    "no restrictions",
			handler: StandardHostHandler{},
			url:     "https://example.com/foo/bar",
			wantErr: false,
		},
		{
			name:    "segments OK",
			handler: StandardHostHandler{MinPathSegments: 2, MaxPathSegments: 2},
			url:     "https://example.com/foo/bar",
			wantErr: false,
		},
		{
			name:    "segments Fail",
			handler: StandardHostHandler{MinPathSegments: 3, MaxPathSegments: 3},
			url:     "https://example.com/foo/bar",
			wantErr: true,
		},
		{
			name:    "prefix OK",
			handler: StandardHostHandler{PathPrefix: "/git/"},
			url:     "https://example.com/git/repo",
			wantErr: false,
		},
		{
			name:    "invalid prefix",
			handler: StandardHostHandler{PathPrefix: "/git/"},
			url:     "https://example.com/other/repo",
			wantErr: true,
		},
		{
			name:    "prefix and segments OK",
			handler: StandardHostHandler{PathPrefix: "/git/", MinPathSegments: 1, MaxPathSegments: 1},
			url:     "https://example.com/git/repo",
			wantErr: false,
		},
		{
			name:    "min segments OK",
			handler: StandardHostHandler{MinPathSegments: 2},
			url:     "https://example.com/foo/bar",
			wantErr: false,
		},
		{
			name:    "min segments OK (more than min)",
			handler: StandardHostHandler{MinPathSegments: 2},
			url:     "https://example.com/foo/bar/baz",
			wantErr: false,
		},
		{
			name:    "min segments Fail",
			handler: StandardHostHandler{MinPathSegments: 2},
			url:     "https://example.com/foo",
			wantErr: true,
		},
		{
			name:    "max segments OK",
			handler: StandardHostHandler{MaxPathSegments: 2},
			url:     "https://example.com/foo/bar",
			wantErr: false,
		},
		{
			name:    "max segments OK (less than max)",
			handler: StandardHostHandler{MaxPathSegments: 2},
			url:     "https://example.com/foo",
			wantErr: false,
		},
		{
			name:    "max segments Fail",
			handler: StandardHostHandler{MaxPathSegments: 2},
			url:     "https://example.com/foo/bar/baz",
			wantErr: true,
		},
		{
			name:    "range OK (min)",
			handler: StandardHostHandler{MinPathSegments: 2, MaxPathSegments: 3},
			url:     "https://example.com/foo/bar",
			wantErr: false,
		},
		{
			name:    "range OK (max)",
			handler: StandardHostHandler{MinPathSegments: 2, MaxPathSegments: 3},
			url:     "https://example.com/foo/bar/baz",
			wantErr: false,
		},
		{
			name:    "range Fail (less)",
			handler: StandardHostHandler{MinPathSegments: 2, MaxPathSegments: 3},
			url:     "https://example.com/foo",
			wantErr: true,
		},
		{
			name:    "range Fail (more)",
			handler: StandardHostHandler{MinPathSegments: 2, MaxPathSegments: 3},
			url:     "https://example.com/foo/bar/baz/qux",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.url)
			err := tt.handler.Validate(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStandardHostHandler_Canon(t *testing.T) {
	tests := []struct {
		name    string
		handler StandardHostHandler
		url     string
		want    string
	}{
		{
			name:    "force scheme",
			handler: StandardHostHandler{ForceScheme: "https"},
			url:     "http://example.com/foo",
			want:    "https://example.com/foo",
		},
		{
			name:    "strip user",
			handler: StandardHostHandler{StripUser: true},
			url:     "https://user:pass@example.com/foo",
			want:    "https://example.com/foo",
		},
		{
			name:    "lower path segments",
			handler: StandardHostHandler{LowerPathSegments: 2},
			url:     "https://example.com/Foo/Bar/Baz",
			want:    "https://example.com/foo/bar/Baz",
		},
		{
			name:    "has dot git suffix (add)",
			handler: StandardHostHandler{HasDotGitSuffix: true},
			url:     "https://example.com/foo/bar",
			want:    "https://example.com/foo/bar.git",
		},
		{
			name:    "has dot git suffix (remove)",
			handler: StandardHostHandler{HasDotGitSuffix: false},
			url:     "https://example.com/foo/bar.git",
			want:    "https://example.com/foo/bar",
		},
		{
			name:    "has trailing slash (add)",
			handler: StandardHostHandler{HasTrailingSlash: true},
			url:     "https://example.com/foo/bar",
			want:    "https://example.com/foo/bar/",
		},
		{
			name:    "has trailing slash (remove)",
			handler: StandardHostHandler{HasTrailingSlash: false},
			url:     "https://example.com/foo/bar/",
			want:    "https://example.com/foo/bar",
		},
		{
			name:    "prefix and lower path segments",
			handler: StandardHostHandler{PathPrefix: "/git/", LowerPathSegments: 1},
			url:     "https://example.com/git/Foo/Bar",
			want:    "https://example.com/git/foo/Bar",
		},
		{
			name:    "negative lower path segments (-1) 3 segments",
			handler: StandardHostHandler{LowerPathSegments: -1},
			url:     "https://example.com/Foo/Bar/Baz",
			want:    "https://example.com/foo/bar/Baz",
		},
		{
			name:    "negative lower path segments (-1) 2 segments",
			handler: StandardHostHandler{LowerPathSegments: -1},
			url:     "https://example.com/Foo/Bar",
			want:    "https://example.com/foo/Bar",
		},
		{
			name:    "negative lower path segments (-1) 1 segment",
			handler: StandardHostHandler{LowerPathSegments: -1},
			url:     "https://example.com/Foo",
			want:    "https://example.com/Foo",
		},
		{
			name:    "negative lower path segments (-2) 3 segments",
			handler: StandardHostHandler{LowerPathSegments: -2},
			url:     "https://example.com/Foo/Bar/Baz",
			want:    "https://example.com/foo/Bar/Baz",
		},
		{
			name:    "negative lower path segments (-2) 4 segments",
			handler: StandardHostHandler{LowerPathSegments: -2},
			url:     "https://example.com/Foo/Bar/Baz/Qux",
			want:    "https://example.com/foo/bar/Baz/Qux",
		},
		{
			name:    "prefix and negative lower path segments (-1)",
			handler: StandardHostHandler{PathPrefix: "/git/", LowerPathSegments: -1},
			url:     "https://example.com/git/Foo/Bar/Baz",
			want:    "https://example.com/git/foo/bar/Baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.url)
			got := tt.handler.Canon(u)
			if got.String() != tt.want {
				t.Errorf("Canon() = %v; want %v", got.String(), tt.want)
			}
		})
	}
}
