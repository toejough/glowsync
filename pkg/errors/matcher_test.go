package errors_test

import (
	"testing"

	"github.com/joe/copy-files/pkg/errors"
)

func TestPatternMatcher_CaseInsensitive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "uppercase permission denied",
			errorMsg: "PERMISSION DENIED",
			expected: errors.CategoryPermission,
		},
		{
			name:     "mixed case no space left",
			errorMsg: "No Space Left On Device",
			expected: errors.CategoryDiskSpace,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_MatchCopyErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "short write",
			errorMsg: "short write",
			expected: errors.CategoryCopy,
		},
		{
			name:     "input/output error",
			errorMsg: "input/output error",
			expected: errors.CategoryCopy,
		},
		{
			name:     "i/o error",
			errorMsg: "i/o error during copy",
			expected: errors.CategoryCopy,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_MatchDeleteErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "directory not empty",
			errorMsg: "directory not empty: /path/to/dir",
			expected: errors.CategoryDelete,
		},
		{
			name:     "cannot remove",
			errorMsg: "cannot remove /path/file.txt",
			expected: errors.CategoryDelete,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_MatchDiskSpaceErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "no space left on device",
			errorMsg: "no space left on device",
			expected: errors.CategoryDiskSpace,
		},
		{
			name:     "disk full",
			errorMsg: "disk full: cannot write",
			expected: errors.CategoryDiskSpace,
		},
		{
			name:     "quota exceeded",
			errorMsg: "disk quota exceeded",
			expected: errors.CategoryDiskSpace,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_MatchPathErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "no such file or directory",
			errorMsg: "no such file or directory: /path/to/file.txt",
			expected: errors.CategoryPath,
		},
		{
			name:     "file not found",
			errorMsg: "file not found",
			expected: errors.CategoryPath,
		},
		{
			name:     "path does not exist",
			errorMsg: "path does not exist",
			expected: errors.CategoryPath,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_MatchPermissionErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
		expected errors.ErrorCategory
	}{
		{
			name:     "permission denied",
			errorMsg: "permission denied",
			expected: errors.CategoryPermission,
		},
		{
			name:     "access denied",
			errorMsg: "access denied to /path/file.txt",
			expected: errors.CategoryPermission,
		},
		{
			name:     "operation not permitted",
			errorMsg: "operation not permitted",
			expected: errors.CategoryPermission,
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != testCase.expected {
				t.Errorf("expected category %q, got %q for error: %q",
					testCase.expected, category, testCase.errorMsg)
			}
		})
	}
}

func TestPatternMatcher_UnknownErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errorMsg string
	}{
		{
			name:     "random error message",
			errorMsg: "something completely unexpected happened",
		},
		{
			name:     "generic error",
			errorMsg: "an error occurred",
		},
	}

	matcher := errors.NewPatternMatcher()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			category := matcher.Match(testCase.errorMsg)
			if category != errors.CategoryUnknown {
				t.Errorf("expected category %q, got %q for error: %q",
					errors.CategoryUnknown, category, testCase.errorMsg)
			}
		})
	}
}
