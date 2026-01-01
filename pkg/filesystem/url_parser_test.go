//nolint:varnamelen // Test files use idiomatic short variable names (t, etc.)
package filesystem_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/filesystem"
)

// TestParsePath_Local tests ParsePath with local filesystem paths.
func TestParsePath_Local(t *testing.T) {
	t.Parallel()

	result, err := filesystem.ParsePath("/local/path")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.IsRemote {
		t.Error("IsRemote should be false for local path")
	}
	if result.LocalPath != "/local/path" {
		t.Errorf("LocalPath = %q, want %q", result.LocalPath, "/local/path")
	}
}

// TestParsePath_SFTP tests ParsePath with valid SFTP URLs.
//
//nolint:funlen // Comprehensive table-driven test with many SFTP URL parsing cases
func TestParsePath_SFTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantUser string
		wantHost string
		wantPort int
		wantPath string
	}{
		{
			name:     "basic SFTP URL",
			input:    "sftp://user@host/path",
			wantErr:  false,
			wantUser: "user",
			wantHost: "host",
			wantPort: 22,
			wantPath: "path",
		},
		{
			name:     "SFTP URL with custom port",
			input:    "sftp://admin@server.com:2222/home/data",
			wantErr:  false,
			wantUser: "admin",
			wantHost: "server.com",
			wantPort: 2222,
			wantPath: "home/data",
		},
		{
			name:    "SFTP URL without username",
			input:   "sftp://host/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := filesystem.ParsePath(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.User != tt.wantUser {
				t.Errorf("User = %q, want %q", result.User, tt.wantUser)
			}
			if result.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", result.Host, tt.wantHost)
			}
			if result.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", result.Port, tt.wantPort)
			}
			if result.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", result.Path, tt.wantPath)
			}
		})
	}
}
