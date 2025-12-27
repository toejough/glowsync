// Package config handles application configuration and command-line argument parsing.
package config

//go:generate impgen config.PostProcessConfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
)

// ChangeType represents the type of changes expected in the sync operation
type ChangeType int

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
func (ct ChangeType) String() string {
	switch ct {
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

// ParseChangeType parses a string into a ChangeType
func ParseChangeType(s string) (ChangeType, error) {
	s = strings.ToLower(s)
	switch s {
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
		return MonotonicCount, fmt.Errorf("invalid change type: %s (valid: monotonic, fluctuating, content, devious, paranoid)", s)
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

// Config holds the application configuration
type Config struct {
	SourcePath      string     `arg:"-s,--source" help:"Source directory path"`
	DestPath        string     `arg:"-d,--dest" help:"Destination directory path"`
	InteractiveMode bool       `arg:"-i,--interactive" help:"Run in interactive mode"`
	AdaptiveMode    bool       `arg:"--adaptive" default:"true" help:"Use adaptive concurrency"`
	Workers         int        `arg:"-w,--workers" default:"4" help:"Number of concurrent workers (0 = adaptive)"`
	TypeOfChange    ChangeType `arg:"--type-of-change,--type" default:"monotonic-count" help:"Type of changes expected: monotonic-count|fluctuating-count|content|devious-content-changes|paranoid-does-not-mean-wrong (aliases: monotonic|fluctuating|content|devious|paranoid)"`
}

// Description returns the program description for go-arg
func (Config) Description() string {
	return "A fast file synchronization CLI tool with a rich Terminal UI"
}

// Version returns the version string for go-arg
func (Config) Version() string {
	return "copy-files 1.0.0"
}

// ParseFlags parses command-line flags and returns configuration
func ParseFlags() (*Config, error) {
	cfg := &Config{
		AdaptiveMode: true,
		Workers:      4,
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
		if err := cfg.ValidatePaths(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// ValidatePaths validates that source and destination paths are valid
func (cfg *Config) ValidatePaths() error {
	// Check source path is provided
	if cfg.SourcePath == "" {
		return fmt.Errorf("source path is required")
	}

	// Check destination path is provided
	if cfg.DestPath == "" {
		return fmt.Errorf("destination path is required")
	}

	// Check if source exists and is a directory
	sourceInfo, err := os.Stat(cfg.SourcePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", cfg.SourcePath)
	}
	if err != nil {
		return fmt.Errorf("cannot access source path: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", cfg.SourcePath)
	}

	// Check if destination exists and is a directory
	destInfo, err := os.Stat(cfg.DestPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("destination path does not exist: %s", cfg.DestPath)
	}
	if err != nil {
		return fmt.Errorf("cannot access destination path: %w", err)
	}
	if !destInfo.IsDir() {
		return fmt.Errorf("destination path is not a directory: %s", cfg.DestPath)
	}

	return nil
}
