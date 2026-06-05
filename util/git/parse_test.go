// Copyright 2026 Google LLC
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

package git

import (
	"errors"
	"testing"
)

func TestParseRemote(t *testing.T) {
	tests := []struct {
		name    string
		want    string // Expected u.String()
		wantErr error
	}{
		{
			name: "https://github.com/user/repo.git",
			want: "https://github.com/user/repo.git",
		},
		{
			name: "ssh://git@github.com/user/repo.git",
			want: "ssh://git@github.com/user/repo.git",
		},
		{
			name: "git@github.com:user/repo.git",
			want: "ssh://git@github.com/user/repo.git",
		},
		{
			name: "git@github.com:/user/repo.git",
			want: "ssh://git@github.com/user/repo.git",
		},
		{
			name: "git@[2001:db8::1]:user/repo.git",
			want: "ssh://git@[2001:db8::1]/user/repo.git",
		},
		{
			name: "[2001:db8::1]:user/repo.git",
			want: "ssh://[2001:db8::1]/user/repo.git",
		},
		{
			name: "git://github.com/user/repo.git",
			want: "git://github.com/user/repo.git",
		},
		{
			name: "ftp://example.com/repo.git",
			want: "ftp://example.com/repo.git",
		},
		{
			name: "git+https://github.com/user/repo.git",
			want: "git+https://github.com/user/repo.git",
		},
		{
			name: "ssh+git://git@github.com/user/repo",
			want: "ssh+git://git@github.com/user/repo",
		},
		{
			name: "host:",
			want: "ssh://host/",
		},
		{
			name:    "",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "transport",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "hg::https://example.com/repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "file:///path/to/repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "something://github.com/user/repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "ssh://git@github.com",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "git://github.com",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "git://user@github.com/repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "git:///repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "git@github.com/user/repo",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "git@[2001:db8::1abc/test",
			wantErr: ErrInvalidRepo,
		},
		{
			name:    "https://example.com:1234a/test",
			wantErr: ErrInvalidRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.name)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("parse(%q) = %v; wantErr %v", tt.name, got, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parse(%q) error = %v; wantErr %v", tt.name, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse(%q) unexpected error: %v", tt.name, err)
			}
			if got.String() != tt.want {
				t.Errorf("parse(%q) = %v; want %v", tt.name, got, tt.want)
			}
		})
	}
}
