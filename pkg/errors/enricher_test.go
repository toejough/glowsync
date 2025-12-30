package errors_test

import (
	"errors"
	"testing"

	pkgerrors "github.com/joe/copy-files/pkg/errors"
)

func TestEnricher_EnrichAlreadyActionableError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalActionable := pkgerrors.NewActionableError(
		"permission denied",
		pkgerrors.CategoryPermission,
		[]string{"existing suggestion"},
		"/original/path",
	)

	enriched := enricher.Enrich(originalActionable, "/new/path")

	// Should return the same error unchanged
	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr != originalActionable {
		t.Error("expected same ActionableError instance when enriching ActionableError")
	}
}

func TestEnricher_EnrichCopyError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("short write")

	enriched := enricher.Enrich(originalErr, "/dest/file.txt")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr.Category() != pkgerrors.CategoryCopy {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryCopy, actionableErr.Category())
	}
}

func TestEnricher_EnrichDeleteError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("directory not empty: /path/to/dir")

	enriched := enricher.Enrich(originalErr, "/path/to/dir")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr.Category() != pkgerrors.CategoryDelete {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryDelete, actionableErr.Category())
	}
}

func TestEnricher_EnrichDiskSpaceError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("no space left on device")

	enriched := enricher.Enrich(originalErr, "/dev/sda1")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr.Category() != pkgerrors.CategoryDiskSpace {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryDiskSpace, actionableErr.Category())
	}

	// Should have disk space related suggestions
	suggestions := actionableErr.Suggestions()
	foundDiskSuggestion := false

	for _, suggestion := range suggestions {
		if containsSubstring(suggestion, "space") || containsSubstring(suggestion, "df") {
			foundDiskSuggestion = true

			break
		}
	}

	if !foundDiskSuggestion {
		t.Errorf("expected disk space suggestion, got: %v", suggestions)
	}
}

func TestEnricher_EnrichPathError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("no such file or directory: /missing/file.txt")

	enriched := enricher.Enrich(originalErr, "/missing/file.txt")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr.Category() != pkgerrors.CategoryPath {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryPath, actionableErr.Category())
	}
}

func TestEnricher_EnrichStandardError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("permission denied: /path/to/file.txt")

	enriched := enricher.Enrich(originalErr, "/path/to/file.txt")

	// Should return ActionableError
	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	// Should have correct category
	if actionableErr.Category() != pkgerrors.CategoryPermission {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryPermission, actionableErr.Category())
	}

	// Should have suggestions
	if len(actionableErr.Suggestions()) == 0 {
		t.Error("expected suggestions, got none")
	}

	// Should have affected path
	if actionableErr.AffectedPath() != "/path/to/file.txt" {
		t.Errorf("expected path %q, got %q", "/path/to/file.txt", actionableErr.AffectedPath())
	}

	// Should preserve original error message
	if actionableErr.OriginalError() != originalErr.Error() {
		t.Errorf("expected original error %q, got %q", originalErr.Error(), actionableErr.OriginalError())
	}
}

func TestEnricher_EnrichUnknownError(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("something completely unexpected")

	enriched := enricher.Enrich(originalErr, "/some/path")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	if actionableErr.Category() != pkgerrors.CategoryUnknown {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryUnknown, actionableErr.Category())
	}

	// Should still have helpful suggestions
	if len(actionableErr.Suggestions()) == 0 {
		t.Error("expected suggestions for unknown error, got none")
	}
}

func TestEnricher_EnrichWithEmptyPath(t *testing.T) {
	t.Parallel()

	enricher := pkgerrors.NewEnricher()
	originalErr := errors.New("permission denied")

	enriched := enricher.Enrich(originalErr, "")

	var actionableErr pkgerrors.ActionableError
	if !errors.As(enriched, &actionableErr) {
		t.Fatalf("expected ActionableError, got %T", enriched)
	}

	// Should still categorize and provide suggestions
	if actionableErr.Category() != pkgerrors.CategoryPermission {
		t.Errorf("expected category %q, got %q", pkgerrors.CategoryPermission, actionableErr.Category())
	}

	if len(actionableErr.Suggestions()) == 0 {
		t.Error("expected suggestions even with empty path, got none")
	}

	// Path should be empty
	if actionableErr.AffectedPath() != "" {
		t.Errorf("expected empty path, got %q", actionableErr.AffectedPath())
	}
}

//nolint:funlen // Comprehensive test cases for path extraction patterns
func TestEnricher_ExtractPathFromErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		errorMsg     string
		providedPath string
		expectedPath string
		category     pkgerrors.ErrorCategory
	}{
		{
			name:         "extract path from 'open /path: permission denied' format",
			errorMsg:     "open /home/user/file.txt: permission denied",
			providedPath: "",
			expectedPath: "/home/user/file.txt",
			category:     pkgerrors.CategoryPermission,
		},
		{
			name:         "extract path from 'stat /path: no such file' format",
			errorMsg:     "stat /var/log/app.log: no such file or directory",
			providedPath: "",
			expectedPath: "/var/log/app.log",
			category:     pkgerrors.CategoryPath,
		},
		{
			name:         "extract path from 'remove /path: directory not empty' format",
			errorMsg:     "remove /tmp/data: directory not empty",
			providedPath: "",
			expectedPath: "/tmp/data",
			category:     pkgerrors.CategoryDelete,
		},
		{
			name:         "extract relative path from error message",
			errorMsg:     "open ./config.yaml: permission denied",
			providedPath: "",
			expectedPath: "./config.yaml",
			category:     pkgerrors.CategoryPermission,
		},
		{
			name:         "prefer provided path over extracted path",
			errorMsg:     "open /extracted/path.txt: permission denied",
			providedPath: "/provided/path.txt",
			expectedPath: "/provided/path.txt",
			category:     pkgerrors.CategoryPermission,
		},
		{
			name:         "no path extraction when no path in error",
			errorMsg:     "permission denied",
			providedPath: "",
			expectedPath: "",
			category:     pkgerrors.CategoryPermission,
		},
		{
			name:         "extract Windows path",
			errorMsg:     "open C:\\Users\\test\\file.txt: permission denied",
			providedPath: "",
			expectedPath: "C:\\Users\\test\\file.txt",
			category:     pkgerrors.CategoryPermission,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			enricher := pkgerrors.NewEnricher()
			originalErr := errors.New(testCase.errorMsg)

			enriched := enricher.Enrich(originalErr, testCase.providedPath)

			var actionableErr pkgerrors.ActionableError
			if !errors.As(enriched, &actionableErr) {
				t.Fatalf("expected ActionableError, got %T", enriched)
			}

			if actionableErr.AffectedPath() != testCase.expectedPath {
				t.Errorf("expected path %q, got %q", testCase.expectedPath, actionableErr.AffectedPath())
			}

			if actionableErr.Category() != testCase.category {
				t.Errorf("expected category %q, got %q", testCase.category, actionableErr.Category())
			}
		})
	}
}
