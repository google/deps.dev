package git

import (
	"errors"
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCanon  string
		wantParsed string
		wantErr    error
	}{
		{
			name:       "github happy path",
			input:      "https://github.com/user/repo.git",
			wantCanon:  "https://github.com/user/repo.git",
			wantParsed: "https://github.com/user/repo.git",
		},
		{
			name:       "github ssh to https",
			input:      "git@github.com:user/repo.git",
			wantCanon:  "https://github.com/user/repo.git", // defaultHostHandler forces https
			wantParsed: "ssh://git@github.com/user/repo.git",
		},
		{
			name:       "github mixed case",
			input:      "https://Github.com/User/Repo.git",
			wantCanon:  "https://github.com/user/repo.git", // lowerPathSegments = 2
			wantParsed: "https://Github.com/User/Repo.git",
		},
		{
			name:       "gitee happy path",
			input:      "https://gitee.com/user/repo.git",
			wantCanon:  "https://gitee.com/user/repo.git",
			wantParsed: "https://gitee.com/user/repo.git",
		},
		{
			name:       "gitee lower path segments",
			input:      "https://gitee.com/User/Repo.git",
			wantCanon:  "https://gitee.com/user/Repo.git", // lowerPathSegments = 1
			wantParsed: "https://gitee.com/User/Repo.git",
		},
		{
			name:    "invalid repo",
			input:   "not-a-repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "github invalid path segments",
			input:   "https://github.com/only-one",
			wantErr: ErrInvalidRepo, // defaultHostHandler requires 2 segments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseRepo(tt.input)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ParseRepo(%q) = %v; wantErr %v", tt.input, r, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseRepo(%q) error = %v; wantErr %v", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRepo(%q) unexpected error: %v", tt.input, err)
			}

			if got := r.Canon().String(); got != tt.wantCanon {
				t.Errorf("Canon() = %v; want %v", got, tt.wantCanon)
			}
			if got := r.Parsed().String(); got != tt.wantParsed {
				t.Errorf("Parsed() = %v; want %v", got, tt.wantParsed)
			}
			if got := r.Raw(); got != tt.input {
				t.Errorf("Raw() = %v; want %v", got, tt.input)
			}
		})
	}
}

func TestRepo_ID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard github",
			input: "https://github.com/user/repo.git",
			want:  "github.com/user/repo",
		},
		{
			name:  "github ssh",
			input: "git@github.com:user/repo.git",
			want:  "github.com/user/repo",
		},
		{
			name:  "ipv6 host",
			input: "ssh://git@[2001:db8::1]/user/repo.git",
			want:  "[2001:db8::1]/user/repo",
		},
		{
			name:  "no dot git",
			input: "https://github.com/user/repo",
			want:  "github.com/user/repo",
		},
		{
			name:  "trailing slash",
			input: "https://github.com/user/repo/",
			want:  "github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseRepo(tt.input)
			if err != nil {
				t.Fatalf("ParseRepo(%q) unexpected error: %v", tt.input, err)
			}
			if got := r.ID(); got != tt.want {
				t.Errorf("ID() = %v; want %v", got, tt.want)
			}
		})
	}
}
