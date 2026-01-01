// Package config handles application configuration and command-line argument parsing.
package config

//go:generate impgen --target config.PostProcessConfig

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/bmatcuk/doublestar/v4"
)

// Exported constants.
const (
	// DefaultMaxWorkers is the default maximum number of concurrent workers
	DefaultMaxWorkers = 4
)

// ChangeType represents the type of changes expected in the sync operation
type ChangeType int

// ChangeType values.
const (
	// MonotonicCount - only files added OR removed (not both)
	MonotonicCount ChangeType = iota
	// FluctuatingCount - files added AND removed
	FluctuatingCount
	// Content - files may be altered (content changes)
	Content
	// DeviousContent - files altered with same modtime (devious changes)
	DeviousContent
	// Paranoid - meticulous byte-by-byte comparison
	Paranoid
)

// String returns the string representation of ChangeType
func (ct *ChangeType) String() string {
	switch *ct {
	case MonotonicCount:
		return "monotonic-count"
	case FluctuatingCount:
		return "fluctuating-count"
	case Content:
		return "content"
	case DeviousContent:
		return "devious-content-changes"
	case Paranoid:
		return "paranoid-does-not-mean-wrong"
	default:
		return "unknown"
	}
}

// UnmarshalText implements encoding.TextUnmarshaler for go-arg
func (ct *ChangeType) UnmarshalText(text []byte) error {
	parsed, err := ParseChangeType(string(text))
	if err != nil {
		return err
	}

	*ct = parsed

	return nil
}

// Exported variables.
var (
	ErrDestPathNotDirectory   = errors.New("destination path is not a directory")
	ErrDestPathNotExist       = errors.New("destination path does not exist")
	ErrDestPathRequired       = errors.New("destination path is required")
	ErrInvalidChangeType      = errors.New("invalid change type")
	ErrInvalidFilePattern     = errors.New("invalid file pattern")
	ErrSourcePathNotDirectory = errors.New("source path is not a directory")
	ErrSourcePathNotExist     = errors.New("source path does not exist")
	ErrSourcePathRequired     = errors.New("source path is required")
)

// Config holds the application configuration
type Config struct {
	SourcePath       string     `arg:"-s,--source"             help:"Source directory path"`
	DestPath         string     `arg:"-d,--dest"               help:"Destination directory path"`
	FilePattern      string     `arg:"--filter"                help:"File pattern filter (glob syntax, e.g., *.mov, **/*.{mov,mp4})"` //nolint:lll
	InteractiveMode  bool       `arg:"-i,--interactive"        help:"Run in interactive mode"`
	SkipConfirmation bool       `arg:"--yes,-y"                help:"Skip confirmation screen and proceed directly to sync"`                                                                                                                                                                //nolint:lll
	AdaptiveMode     bool       `arg:"--adaptive"              default:"true"                    help:"Use adaptive concurrency"`                                                                                                                                                           //nolint:lll,tagalign
	Workers          int        `arg:"-w,--workers"            default:"4"                       help:"Number of workers (0 = adaptive)"`                                                                                                                                                   //nolint:lll,tagalign
	TypeOfChange     ChangeType `arg:"--type-of-change,--type" default:"monotonic-count"         help:"Type of changes expected: monotonic-count|fluctuating-count|content|devious-content-changes|paranoid-does-not-mean-wrong (aliases: monotonic|fluctuating|content|devious|paranoid)"` //nolint:lll,tagalign // Struct tag with comprehensive help text
	Verbose          bool       `arg:"-v,--verbose"            help:"Enable verbose progress logging"`                                                                                                                                                                                      //nolint:tagalign
}

// Description returns the program description for go-arg
func (Config) Description() string {
	return "A fast file synchronization CLI tool with a rich Terminal UI"
}

// ValidatePaths validates that source and destination paths are valid.
// Supports both local paths and SFTP URLs (sftp://user@host:port/path).
// For SFTP URLs, basic URL parsing is validated, but remote existence cannot be
// checked until connection time.
func (cfg Config) ValidatePaths() error {
	// Check source path is provided
	if cfg.SourcePath == "" {
		return ErrSourcePathRequired
	}

	// Check destination path is provided
	if cfg.DestPath == "" {
		return ErrDestPathRequired
	}

	// Check if source is an SFTP URL
	if strings.HasPrefix(cfg.SourcePath, "sftp://") {
		// Validate SFTP URL format
		if err := validateSFTPURL(cfg.SourcePath); err != nil {
			return fmt.Errorf("invalid source SFTP URL: %w", err)
		}
		// Cannot validate remote paths until connection - will be validated during engine init
	} else {
		// Validate local source path
		if err := validateLocalPath(cfg.SourcePath, "source"); err != nil {
			return err
		}
	}

	// Check if destination is an SFTP URL
	if strings.HasPrefix(cfg.DestPath, "sftp://") {
		// Validate SFTP URL format
		if err := validateSFTPURL(cfg.DestPath); err != nil {
			return fmt.Errorf("invalid destination SFTP URL: %w", err)
		}
		// Cannot validate remote paths until connection - will be validated during engine init
	} else {
		// Validate local destination path
		if err := validateLocalPath(cfg.DestPath, "destination"); err != nil {
			return err
		}
	}

	return nil
}

// validateSFTPURL validates basic SFTP URL format
func validateSFTPURL(sftpURL string) error {
	// Basic validation - just check it has required components
	if !strings.Contains(sftpURL, "@") {
		return errors.New("SFTP URL must include username (sftp://user@host/path)")
	}
	if !strings.Contains(sftpURL, "/") || strings.Count(sftpURL, "/") < 3 {
		return errors.New("SFTP URL must include path (sftp://user@host/path)")
	}
	return nil
}

// validateLocalPath validates that a local path exists and is a directory
func validateLocalPath(path, pathType string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if pathType == "source" {
			return fmt.Errorf("%w: %s", ErrSourcePathNotExist, path)
		}
		return fmt.Errorf("%w: %s", ErrDestPathNotExist, path)
	}

	if err != nil {
		return fmt.Errorf("cannot access %s path: %w", pathType, err)
	}

	if !info.IsDir() {
		if pathType == "source" {
			return fmt.Errorf("%w: %s", ErrSourcePathNotDirectory, path)
		}
		return fmt.Errorf("%w: %s", ErrDestPathNotDirectory, path)
	}

	return nil
}

// Version returns the version string for go-arg
func (Config) Version() string {
	return "copy-files 1.0.0"
}

// ParseChangeType parses a string into a ChangeType
func ParseChangeType(changeTypeStr string) (ChangeType, error) {
	changeTypeStr = strings.ToLower(changeTypeStr)
	switch changeTypeStr {
	case "monotonic-count", "monotonic":
		return MonotonicCount, nil
	case "fluctuating-count", "fluctuating":
		return FluctuatingCount, nil
	case "content":
		return Content, nil
	case "devious-content-changes", "devious":
		return DeviousContent, nil
	case "paranoid-does-not-mean-wrong", "paranoid":
		return Paranoid, nil
	default:
		return MonotonicCount, fmt.Errorf(
			"%w: %s (valid: monotonic, fluctuating, content, devious, paranoid)",
			ErrInvalidChangeType, changeTypeStr)
	}
}

// ParseFlags parses command-line flags and returns configuration
func ParseFlags() (*Config, error) {
	cfg := &Config{
		AdaptiveMode: true,
		Workers:      DefaultMaxWorkers,
		TypeOfChange: MonotonicCount,
	}

	arg.MustParse(cfg)

	return PostProcessConfig(cfg)
}

// PostProcessConfig applies post-processing logic to a parsed config
func PostProcessConfig(cfg *Config) (*Config, error) {
	// If no flags provided, default to interactive mode
	if cfg.SourcePath == "" && cfg.DestPath == "" {
		cfg.InteractiveMode = true
	}

	// Validate paths if not in interactive mode
	if !cfg.InteractiveMode {
		err := cfg.ValidatePaths()
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// ValidateFilePattern validates a file pattern for glob syntax correctness
// Empty pattern is valid (matches all files)
// Returns ErrInvalidFilePattern if the pattern has invalid syntax
func ValidateFilePattern(pattern string) error {
	// Empty pattern is valid (no filtering)
	if pattern == "" {
		return nil
	}

	// Validate by attempting to match against a test path
	// doublestar will return an error if the pattern syntax is invalid
	_, err := doublestar.Match(strings.ToLower(pattern), "test")
	if err != nil {
		return fmt.Errorf("%w: %s (%w)", ErrInvalidFilePattern, pattern, err)
	}

	return nil
}
