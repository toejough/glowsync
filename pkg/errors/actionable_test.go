package errors_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/errors"
)

func TestActionableError_FormatSuggestionsWithEmptySuggestions(t *testing.T) {
	t.Parallel()

	err := errors.NewActionableError(
		"unknown error",
		errors.CategoryUnknown,
		[]string{},
		"/path",
	)

	formatted := errors.FormatSuggestions(err)

	// Should return empty string for no suggestions
	if formatted != "" {
		t.Errorf("expected empty string for no suggestions, got %q", formatted)
	}
}

func TestActionableError_FormatSuggestionsWithMultipleSuggestions(t *testing.T) {
	t.Parallel()

	err := errors.NewActionableError(
		"permission denied",
		errors.CategoryPermission,
		[]string{
			"Check permissions with 'ls -la'",
			"Ensure you have read/write access",
			"Try running with sudo",
		},
		"/path/to/file",
	)

	formatted := errors.FormatSuggestions(err)

	// Should format as bulleted list
	expected := "  • Check permissions with 'ls -la'\n  • Ensure you have read/write access\n  • Try running with sudo"
	if formatted != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, formatted)
	}
}

func TestActionableError_FormatSuggestionsWithNonActionableError(t *testing.T) {
	t.Parallel()

	// Test with a regular error that isn't ActionableError
	formatted := errors.FormatSuggestions(nil)

	// Should return empty string for nil error
	if formatted != "" {
		t.Errorf("expected empty string for nil error, got %q", formatted)
	}
}

func TestActionableError_FormatSuggestionsWithSingleSuggestion(t *testing.T) {
	t.Parallel()

	err := errors.NewActionableError(
		"no space left on device",
		errors.CategoryDiskSpace,
		[]string{"Run 'df -h' to check available space"},
		"/dev/sda1",
	)

	formatted := errors.FormatSuggestions(err)

	// Should format as single bulleted item
	expected := "  • Run 'df -h' to check available space"
	if formatted != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, formatted)
	}
}

func TestActionableError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	err := errors.NewActionableError(
		"original error message",
		errors.CategoryPermission,
		[]string{"Check permissions with 'ls -la'"},
		"/path/to/file",
	)

	// Should implement error interface
	var _ error = err

	if err.Error() == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestActionableError_ProvidesAffectedPath(t *testing.T) {
	t.Parallel()

	path := "/tmp/test/file.txt"
	err := errors.NewActionableError(
		"file not found",
		errors.CategoryPath,
		[]string{"Check if path exists"},
		path,
	)

	if err.AffectedPath() != path {
		t.Errorf("expected path %q, got %q", path, err.AffectedPath())
	}
}

func TestActionableError_ProvidesErrorCategory(t *testing.T) {
	t.Parallel()

	err := errors.NewActionableError(
		"no space left on device",
		errors.CategoryDiskSpace,
		[]string{"Free up space"},
		"/dev/sda1",
	)

	if err.Category() != errors.CategoryDiskSpace {
		t.Errorf("expected category %q, got %q", errors.CategoryDiskSpace, err.Category())
	}
}

func TestActionableError_ProvidesOriginalErrorMessage(t *testing.T) {
	t.Parallel()

	originalMsg := "permission denied"
	err := errors.NewActionableError(
		originalMsg,
		errors.CategoryPermission,
		[]string{"Check permissions"},
		"/test/path",
	)

	if err.OriginalError() != originalMsg {
		t.Errorf("expected original error %q, got %q", originalMsg, err.OriginalError())
	}
}

func TestActionableError_ProvidesSuggestions(t *testing.T) {
	t.Parallel()

	suggestions := []string{
		"Check permissions with 'ls -la /path'",
		"Ensure you have read/write access",
	}
	err := errors.NewActionableError(
		"permission denied",
		errors.CategoryPermission,
		suggestions,
		"/path",
	)

	got := err.Suggestions()
	if len(got) != len(suggestions) {
		t.Fatalf("expected %d suggestions, got %d", len(suggestions), len(got))
	}

	for i, want := range suggestions {
		if got[i] != want {
			t.Errorf("suggestion[%d]: expected %q, got %q", i, want, got[i])
		}
	}
}

func TestErrorCategory_CategoriesAreDistinct(t *testing.T) {
	t.Parallel()

	categories := []errors.ErrorCategory{
		errors.CategoryPermission,
		errors.CategoryDiskSpace,
		errors.CategoryPath,
		errors.CategoryDelete,
		errors.CategoryCopy,
		errors.CategoryUnknown,
	}

	seen := make(map[errors.ErrorCategory]bool)
	for _, cat := range categories {
		if seen[cat] {
			t.Errorf("duplicate category: %q", cat)
		}

		seen[cat] = true
	}
}

func TestErrorCategory_DefinesRequiredCategories(t *testing.T) {
	t.Parallel()

	categories := []errors.ErrorCategory{
		errors.CategoryPermission,
		errors.CategoryDiskSpace,
		errors.CategoryPath,
		errors.CategoryDelete,
		errors.CategoryCopy,
		errors.CategoryUnknown,
	}

	for _, cat := range categories {
		if cat == "" {
			t.Error("category should not be empty string")
		}
	}
}
