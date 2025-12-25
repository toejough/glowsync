// Package config handles application configuration and command-line argument parsing.
package config

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	SourcePath      string
	DestPath        string
	InteractiveMode bool
	AdaptiveMode    bool
	Workers         int
	UseCache        bool
}

// ParseFlags parses command-line flags and returns configuration
func ParseFlags() (*Config, error) {
	cfg := &Config{}
	
	flag.StringVar(&cfg.SourcePath, "source", "", "Source directory path")
	flag.StringVar(&cfg.SourcePath, "s", "", "Source directory path (shorthand)")
	flag.StringVar(&cfg.DestPath, "dest", "", "Destination directory path")
	flag.StringVar(&cfg.DestPath, "d", "", "Destination directory path (shorthand)")
	flag.BoolVar(&cfg.InteractiveMode, "interactive", false, "Run in interactive mode")
	flag.BoolVar(&cfg.InteractiveMode, "i", false, "Run in interactive mode (shorthand)")
	flag.BoolVar(&cfg.AdaptiveMode, "adaptive", true, "Use adaptive concurrency (default: true)")
	flag.IntVar(&cfg.Workers, "workers", 4, "Number of concurrent workers (0 = adaptive)")
	flag.BoolVar(&cfg.UseCache, "cache", true, "Use cached scan results (default: true)")

	flag.Parse()
	
	// If no flags provided, default to interactive mode
	if cfg.SourcePath == "" && cfg.DestPath == "" {
		cfg.InteractiveMode = true
	}
	
	// Validate paths if not in interactive mode
	if !cfg.InteractiveMode {
		if cfg.SourcePath == "" {
			return nil, fmt.Errorf("source path is required")
		}
		if cfg.DestPath == "" {
			return nil, fmt.Errorf("destination path is required")
		}
		
		// Check if source exists
		if _, err := os.Stat(cfg.SourcePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("source path does not exist: %s", cfg.SourcePath)
		}
	}
	
	return cfg, nil
}

