package errors

import "fmt"

// SuggestionGenerator generates actionable suggestions based on error category.
type SuggestionGenerator interface {
	Generate(category ErrorCategory, affectedPath string) []string
}

// NewSuggestionGenerator creates a new SuggestionGenerator.
func NewSuggestionGenerator() SuggestionGenerator {
	return &suggestionGenerator{}
}

// suggestionGenerator is the concrete implementation of SuggestionGenerator.
type suggestionGenerator struct{}

// Generate returns actionable suggestions based on the error category and affected path.
func (g *suggestionGenerator) Generate(category ErrorCategory, affectedPath string) []string {
	switch category {
	case CategoryPermission:
		return g.generatePermissionSuggestions(affectedPath)
	case CategoryDiskSpace:
		return g.generateDiskSpaceSuggestions(affectedPath)
	case CategoryPath:
		return g.generatePathSuggestions(affectedPath)
	case CategoryDelete:
		return g.generateDeleteSuggestions(affectedPath)
	case CategoryCopy:
		return g.generateCopySuggestions(affectedPath)
	case CategoryUnknown:
		return g.generateUnknownSuggestions(affectedPath)
	default:
		return g.generateUnknownSuggestions(affectedPath)
	}
}

func (g *suggestionGenerator) generateCopySuggestions(_ string) []string {
	return []string{
		"Check if there is sufficient disk space on the destination",
		"Verify the source and destination media are functioning correctly",
		"Try the operation again - this may be a transient I/O error",
		"Check system logs for hardware issues",
	}
}

func (g *suggestionGenerator) generateDeleteSuggestions(path string) []string {
	suggestions := []string{
		"Ensure the directory is empty before attempting to remove it",
		"Check if files or subdirectories are still present",
	}

	if path != "" {
		suggestions = append(suggestions, fmt.Sprintf("List contents with 'ls -la %s'", path))
	}

	suggestions = append(suggestions, "Remove contents first or use a recursive delete if appropriate")

	return suggestions
}

func (g *suggestionGenerator) generateDiskSpaceSuggestions(path string) []string {
	suggestions := []string{
		"Free up space on the destination device",
		"Check available space with 'df -h'",
		"Remove unnecessary files or move files to a different location",
	}

	if path != "" {
		suggestions = append(suggestions, "Verify disk usage for the filesystem containing "+path)
	}

	return suggestions
}

func (g *suggestionGenerator) generatePathSuggestions(path string) []string {
	suggestions := []string{
		"Verify the path exists and is spelled correctly",
	}

	if path != "" {
		suggestions = append(suggestions, "Check if the path exists: "+path)
		suggestions = append(suggestions, "Ensure all parent directories exist for "+path)
	} else {
		suggestions = append(suggestions, "Ensure all parent directories exist")
	}

	return suggestions
}

func (g *suggestionGenerator) generatePermissionSuggestions(path string) []string {
	suggestions := []string{
		"Ensure you have read/write permissions for the files and directories",
	}

	if path != "" {
		suggestions = append(suggestions, fmt.Sprintf("Check permissions with 'ls -la %s'", path))
	} else {
		suggestions = append(suggestions, "Check permissions with 'ls -la' on the affected path")
	}

	suggestions = append(suggestions, "Try running with appropriate permissions or as a privileged user")

	return suggestions
}

func (g *suggestionGenerator) generateUnknownSuggestions(path string) []string {
	suggestions := []string{
		"Check the error message for more details",
		"Verify file and directory permissions",
		"Ensure sufficient disk space is available",
	}

	if path != "" {
		suggestions = append(suggestions, "Verify the path is accessible: "+path)
	}

	return suggestions
}
