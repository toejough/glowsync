//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target to run when none is specified
var Default = Build

// Build builds the binary
func Build() error {
	fmt.Println("Building...")
	return sh.Run("go", "build", "-o", "copy-files", "./cmd/copy-files")
}

// Test runs all tests
func Test() error {
	fmt.Println("Running tests...")
	return sh.Run("go", "test", "-v", "-race", "-coverprofile=coverage.out", "./...")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")
	return sh.Run("golangci-lint", "run", "./...")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	os.Remove("copy-files")
	os.Remove("coverage.out")
	return nil
}

// Install installs the binary
func Install() error {
	fmt.Println("Installing...")
	return sh.Run("go", "install", "./cmd/copy-files")
}

// Fmt formats the code
func Fmt() error {
	fmt.Println("Formatting code...")
	if err := sh.Run("gofmt", "-s", "-w", "."); err != nil {
		return err
	}
	return sh.Run("goimports", "-w", ".")
}

// Check runs all checks (fmt, lint, test)
func Check() error {
	mg.Deps(Fmt)
	mg.Deps(Lint)
	mg.Deps(Test)
	return nil
}

// Coverage generates and opens coverage report
func Coverage() error {
	if err := Test(); err != nil {
		return err
	}
	fmt.Println("Generating coverage report...")
	if err := sh.Run("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		return err
	}
	
	// Try to open the coverage report
	cmd := exec.Command("open", "coverage.html")
	if err := cmd.Run(); err != nil {
		fmt.Println("Coverage report generated at coverage.html")
	}
	return nil
}

