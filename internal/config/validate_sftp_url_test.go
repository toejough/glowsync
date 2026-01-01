//nolint:varnamelen // Test files use idiomatic short variable names (t, tt, etc.)
package config_test

import (
	"strings"
	"testing"

	"github.com/joe/copy-files/internal/config"
)

// TestValidateSFTPURL tests the unexported validateSFTPURL function indirectly through ValidatePaths
//
//nolint:funlen // Comprehensive table-driven test with many URL validation cases
func TestValidateSFTPURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourcePath string
		destPath   string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid SFTP source URL",
			sourcePath: "sftp://user@host/path",
			destPath:   "sftp://user@host/dest",
			wantErr:    false,
		},
		{
			name:       "valid SFTP URL with port",
			sourcePath: "sftp://user@host:22/path/to/dir",
			destPath:   "sftp://admin@server/dest",
			wantErr:    false,
		},
		{
			name:       "valid SFTP URL with subdirectories",
			sourcePath: "sftp://admin@server.com/home/user/files",
			destPath:   "sftp://user@dest.com/backup",
			wantErr:    false,
		},
		{
			name:       "source missing username (no @)",
			sourcePath: "sftp://host/path",
			destPath:   "sftp://user@host/dest",
			wantErr:    true,
			errMsg:     "must include username",
		},
		{
			name:       "source missing path (only 2 slashes)",
			sourcePath: "sftp://user@host",
			destPath:   "sftp://user@host/dest",
			wantErr:    true,
			errMsg:     "must include path",
		},
		{
			name:       "source with trailing slash is considered valid",
			sourcePath: "sftp://user@host/",
			destPath:   "sftp://user@host/dest",
			wantErr:    false, // Has 3 slashes, so passes validation
		},
		{
			name:       "dest missing username",
			sourcePath: "sftp://user@host/source",
			destPath:   "sftp://host/dest",
			wantErr:    true,
			errMsg:     "must include username",
		},
		{
			name:       "dest missing path",
			sourcePath: "sftp://user@host/source",
			destPath:   "sftp://user@host",
			wantErr:    true,
			errMsg:     "must include path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{
				SourcePath: tt.sourcePath,
				DestPath:   tt.destPath,
			}

			err := cfg.ValidatePaths()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
