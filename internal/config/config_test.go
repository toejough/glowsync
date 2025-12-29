//nolint:varnamelen // Test files use idiomatic short variable names (t, tt, etc.)
package config_test

import (
	"os"
	"testing"

	"github.com/joe/copy-files/internal/config"
	. "github.com/onsi/gomega" //nolint:revive // Dot import is idiomatic for Gomega matchers
)

func TestChangeTypeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct       config.ChangeType
		expected string
	}{
		{config.MonotonicCount, "monotonic-count"},
		{config.FluctuatingCount, "fluctuating-count"},
		{config.Content, "content"},
		{config.DeviousContent, "devious-content-changes"},
		{config.Paranoid, "paranoid-does-not-mean-wrong"},
		{config.ChangeType(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.expected {
			t.Errorf("ChangeType(%d).String() = %q, want %q", tt.ct, got, tt.expected)
		}
	}
}

func TestChangeTypeUnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected config.ChangeType
		wantErr  bool
	}{
		{"monotonic", config.MonotonicCount, false},
		{"content", config.Content, false},
		{"paranoid", config.Paranoid, false},
		{"invalid", config.MonotonicCount, true},
	}

	for _, tt := range tests {
		var ct config.ChangeType

		err := ct.UnmarshalText([]byte(tt.input))
		if (err != nil) != tt.wantErr {
			t.Errorf("UnmarshalText(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}

		if !tt.wantErr && ct != tt.expected {
			t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, ct, tt.expected)
		}
	}
}

func TestConfigDescription(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}

	desc := cfg.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestConfigVersion(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}

	version := cfg.Version()
	if version == "" {
		t.Error("Version() should not be empty")
	}
}

func TestParseChangeType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected config.ChangeType
		wantErr  bool
	}{
		{"monotonic-count", config.MonotonicCount, false},
		{"monotonic", config.MonotonicCount, false},
		{"MONOTONIC", config.MonotonicCount, false},
		{"fluctuating-count", config.FluctuatingCount, false},
		{"fluctuating", config.FluctuatingCount, false},
		{"content", config.Content, false},
		{"CONTENT", config.Content, false},
		{"devious-content-changes", config.DeviousContent, false},
		{"devious", config.DeviousContent, false},
		{"paranoid-does-not-mean-wrong", config.Paranoid, false},
		{"paranoid", config.Paranoid, false},
		{"invalid", config.MonotonicCount, true},
		{"", config.MonotonicCount, true},
	}

	for _, tt := range tests {
		got, err := config.ParseChangeType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseChangeType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}

		if !tt.wantErr && got != tt.expected {
			t.Errorf("ParseChangeType(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseFlags(t *testing.T) {
	t.Parallel()
	// This test is tricky because ParseFlags calls arg.MustParse which modifies os.Args.
	// We can't easily mock this without changing the implementation, so we test
	// the PostProcessConfig function instead, which contains the testable logic.
	// The actual ParseFlags function is tested indirectly through integration tests.

	// Save original os.Args
	oldArgs := os.Args

	defer func() { os.Args = oldArgs }()

	// Test with no arguments - should enable interactive mode
	os.Args = []string{"cmd"}

	cfg, err := config.ParseFlags()
	if err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	if !cfg.InteractiveMode {
		t.Error("InteractiveMode should be true when no paths are provided")
	}
}

func TestPostProcessConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cfg             config.Config
		wantInteractive bool
		wantErr         bool
	}{
		{
			name:            "no paths - should enable interactive mode",
			cfg:             config.Config{},
			wantInteractive: true,
			wantErr:         false,
		},
		{
			name:            "already interactive - should not validate",
			cfg:             config.Config{InteractiveMode: true, SourcePath: "/nonexistent"},
			wantInteractive: true,
			wantErr:         false,
		},
		{
			name:            "invalid source path - should error",
			cfg:             config.Config{SourcePath: "/nonexistent", DestPath: "/some/dest"},
			wantInteractive: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Use imptest wrapper
			wrapper := config.NewPostProcessConfigImp(t, config.PostProcessConfig)
			wrapper.Start(&tt.cfg)

			if tt.wantErr {
				wrapper.ExpectReturnedValuesShould(BeNil(), Not(BeNil()))
			} else {
				wrapper.ExpectReturnedValuesShould(
					And(
						Not(BeNil()),
						HaveField("InteractiveMode", Equal(tt.wantInteractive)),
					),
					BeNil(),
				)
			}
		})
	}
}

func TestValidatePaths(t *testing.T) {
	t.Parallel()

	// Note: ValidatePaths uses os.Stat internally, which we can't easily mock
	// without changing the config package to accept a FileSystem interface.
	// For now, we test the logic without filesystem access by using paths
	// that we know will fail validation (empty strings).

	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name:    "missing source path",
			cfg:     config.Config{SourcePath: "", DestPath: "/some/dest"},
			wantErr: true,
		},
		{
			name:    "missing dest path",
			cfg:     config.Config{SourcePath: "/some/source", DestPath: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.ValidatePaths()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePaths() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "empty pattern is valid",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "simple wildcard",
			pattern: "*.mov",
			wantErr: false,
		},
		{
			name:    "double star",
			pattern: "**/*.mov",
			wantErr: false,
		},
		{
			name:    "brace expansion",
			pattern: "*.{mov,mp4}",
			wantErr: false,
		},
		{
			name:    "complex pattern",
			pattern: "videos/**/*.{mov,mp4,avi}",
			wantErr: false,
		},
		{
			name:    "invalid pattern - unclosed bracket",
			pattern: "[invalid",
			wantErr: true,
		},
		{
			name:    "invalid pattern - unclosed brace",
			pattern: "*.{mov",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateFilePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}

			if err != nil && !tt.wantErr {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
