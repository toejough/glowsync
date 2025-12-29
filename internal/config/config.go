// Package config handles application configuration and command-line argument parsing.
package config

//go:generate impgen config.PostProcessConfig

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
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
	ErrSourcePathNotDirectory = errors.New("source path is not a directory")
	ErrSourcePathNotExist     = errors.New("source path does not exist")
	ErrSourcePathRequired     = errors.New("source path is required")
)

// Config holds the application configuration
type Config struct {
	SourcePath       string     `arg:"-s,--source"             help:"Source directory path"`
	DestPath         string     `arg:"-d,--dest"               help:"Destination directory path"`
	InteractiveMode  bool       `arg:"-i,--interactive"        help:"Run in interactive mode"`
	SkipConfirmation bool       `arg:"--yes,-y"                help:"Skip confirmation screen and proceed directly to sync"`                                                                                                                                                                //nolint:lll
	AdaptiveMode     bool       `arg:"--adaptive"              default:"true"                    help:"Use adaptive concurrency"`                                                                                                                                                           //nolint:lll
	Workers          int        `arg:"-w,--workers"            default:"4"                       help:"Number of workers (0 = adaptive)"`                                                                                                                                                   //nolint:lll
	TypeOfChange     ChangeType `arg:"--type-of-change,--type" default:"monotonic-count"         help:"Type of changes expected: monotonic-count|fluctuating-count|content|devious-content-changes|paranoid-does-not-mean-wrong (aliases: monotonic|fluctuating|content|devious|paranoid)"` //nolint:lll // Struct tag with comprehensive help text
}

// Description returns the program description for go-arg
func (Config) Description() string {
	return "A fast file synchronization CLI tool with a rich Terminal UI"
}

// ValidatePaths validates that source and destination paths are valid
func (cfg Config) ValidatePaths() error {
	// Check source path is provided
	if cfg.SourcePath == "" {
		return ErrSourcePathRequired
	}

	// Check destination path is provided
	if cfg.DestPath == "" {
		return ErrDestPathRequired
	}

	// Check if source exists and is a directory
	sourceInfo, err := os.Stat(cfg.SourcePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrSourcePathNotExist, cfg.SourcePath)
	}

	if err != nil {
		return fmt.Errorf("cannot access source path: %w", err)
	}

	if !sourceInfo.IsDir() {
		return fmt.Errorf("%w: %s", ErrSourcePathNotDirectory, cfg.SourcePath)
	}

	// Check if destination exists and is a directory
	destInfo, err := os.Stat(cfg.DestPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrDestPathNotExist, cfg.DestPath)
	}

	if err != nil {
		return fmt.Errorf("cannot access destination path: %w", err)
	}

	if !destInfo.IsDir() {
		return fmt.Errorf("%w: %s", ErrDestPathNotDirectory, cfg.DestPath)
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
