// Package config handles application configuration and command-line argument parsing.
package config

import (
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
)

// Config holds the application configuration
type Config struct {
	SourcePath      string `arg:"-s,--source" help:"Source directory path"`
	DestPath        string `arg:"-d,--dest" help:"Destination directory path"`
	InteractiveMode bool   `arg:"-i,--interactive" help:"Run in interactive mode"`
	AdaptiveMode    bool   `arg:"--adaptive" default:"true" help:"Use adaptive concurrency"`
	Workers         int    `arg:"-w,--workers" default:"4" help:"Number of concurrent workers (0 = adaptive)"`
	UseCache        bool   `arg:"--cache" default:"true" help:"Use cached scan results"`
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
		UseCache:     true,
	}

	arg.MustParse(cfg)

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
