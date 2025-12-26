//go:build mage
// +build mage

package main

import (
	"context"
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

// TestForFail runs the unit tests purely to find out whether any fail
func TestForFail() error {
	fmt.Println("Running unit tests for overall pass/fail...")
	return run(
		context.Background(),
		"go",
		"test",
		"-timeout=10s",
		"./...",
		"-failfast",
		"-shuffle=on",
		"-race",
	)
}

// Lint lints the codebase
func Lint() error {
	fmt.Println("Linting...")
	return run(context.Background(), "golangci-lint", "run", "-c", ".golangci.yml", "./...")
}

// LintForFail lints the codebase purely to find out whether anything fails
func LintForFail() error {
	fmt.Println("Linting to check for overall pass/fail...")
	return run(
		context.Background(),
		"golangci-lint", "run",
		"-c", ".golangci.yml",
		"--fix=false",
		"--max-issues-per-linter=1",
		"--max-same-issues=1",
		"./...",
	)
}

// CheckNils checks for nils
func CheckNils() error {
	fmt.Println("Running check for nils...")
	return run(context.Background(), "nilaway", "./...")
}

// CheckForFail runs all checks on the code for determining whether any fail
func CheckForFail() error {
	fmt.Println("Checking for failures...")
	mg.SerialDeps(LintForFail, TestForFail, CheckNils)
	return nil
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
	mg.Deps(Test)
	mg.Deps(Fmt)
	mg.Deps(Lint)
	mg.Deps(CheckNils)
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

// Helper function to run commands with context
func run(c context.Context, command string, arg ...string) error {
	cmd := exec.CommandContext(c, command, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

