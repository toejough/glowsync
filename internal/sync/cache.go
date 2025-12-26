package sync

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joe/copy-files/pkg/fileops"
)

// ScanCache represents cached scan results for a directory
type ScanCache struct {
	Path          string                      `json:"path"`
	ScanTime      time.Time                   `json:"scan_time"`
	Files         map[string]*fileops.FileInfo `json:"files"`
	DirectoryHash string                      `json:"directory_hash"` // Hash of directory structure
}

// getCacheDir returns the directory where cache files are stored
func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	cacheDir := filepath.Join(homeDir, ".cache", "copy-files")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return "", err
	}
	
	return cacheDir, nil
}

// getCachePath returns the cache file path for a given directory
func getCachePath(dirPath string) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	
	// Create a hash of the directory path to use as filename
	hash := sha256.Sum256([]byte(dirPath))
	cacheFile := fmt.Sprintf("scan-%x.json", hash[:8])
	
	return filepath.Join(cacheDir, cacheFile), nil
}

// computeDirectoryHash creates a hash based on directory structure
// This is a quick check - we hash the count of files
func computeDirectoryHash(files map[string]*fileops.FileInfo) string {
	h := sha256.New()

	// Simple approach: hash the count
	// For large directories, we don't want to hash everything
	_, _ = fmt.Fprintf(h, "count:%d", len(files))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// LoadScanCache attempts to load cached scan results
func LoadScanCache(dirPath string) (*ScanCache, error) {
	cachePath, err := getCachePath(dirPath)
	if err != nil {
		return nil, err
	}
	
	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache not found")
	}
	
	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	
	var cache ScanCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	
	return &cache, nil
}

// SaveScanCache saves scan results to cache
func SaveScanCache(dirPath string, files map[string]*fileops.FileInfo) error {
	cachePath, err := getCachePath(dirPath)
	if err != nil {
		return err
	}
	
	cache := ScanCache{
		Path:          dirPath,
		ScanTime:      time.Now(),
		Files:         files,
		DirectoryHash: computeDirectoryHash(files),
	}
	
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(cachePath, data, 0600)
}

// IsCacheValid checks if cached data is still valid
// Returns true if cache can be used, false if we need to rescan
// isDestination: if true, uses more conservative validation for destination directories
// validationLog: optional callback to log validation details
func IsCacheValid(cache *ScanCache, dirPath string, isDestination bool, validationLog func(string)) bool {
	log := func(msg string) {
		if validationLog != nil {
			validationLog(msg)
		}
	}

	// Check if path matches
	if cache.Path != dirPath {
		log(fmt.Sprintf("Cache path mismatch: cached=%s, requested=%s", cache.Path, dirPath))
		return false
	}

	// Check if cache is too old
	// Destinations: 30 minutes (conservative but practical)
	// Sources: 1 hour (less likely to change)
	maxAge := 1 * time.Hour
	if isDestination {
		maxAge = 30 * time.Minute
	}
	age := time.Since(cache.ScanTime)
	if age > maxAge {
		log(fmt.Sprintf("Cache too old: age=%s, maxAge=%s", age.Round(time.Second), maxAge))
		return false
	}

	// Quick validation: check if directory still exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		log(fmt.Sprintf("Directory does not exist: %s", dirPath))
		return false
	}

	// Sample files to see if they've changed
	// Destinations: check 20 files (more thorough)
	// Sources: check 10 files (faster)
	sampleSize := 10
	if isDestination {
		sampleSize = 20
	}
	checked := 0

	// If cache has files, we must be able to verify at least some of them
	// Otherwise the cache is invalid (e.g., files were deleted)
	expectedToCheck := len(cache.Files)
	if expectedToCheck > sampleSize {
		expectedToCheck = sampleSize
	}

	for relPath, cachedInfo := range cache.Files {
		if checked >= sampleSize {
			break
		}

		fullPath := filepath.Join(dirPath, relPath)
		info, err := os.Stat(fullPath)
		if err != nil {
			// File doesn't exist or can't be accessed - cache is invalid
			log(fmt.Sprintf("Sample file missing or inaccessible: %s (error: %v)", relPath, err))
			return false
		}

		// Check if size or mod time changed
		if info.Size() != cachedInfo.Size {
			log(fmt.Sprintf("Sample file size changed: %s (cached=%d, actual=%d)", relPath, cachedInfo.Size, info.Size()))
			return false
		}

		if !info.ModTime().Equal(cachedInfo.ModTime) {
			log(fmt.Sprintf("Sample file modtime changed: %s (cached=%s, actual=%s, diff=%s)",
				relPath, cachedInfo.ModTime.Format(time.RFC3339Nano), info.ModTime().Format(time.RFC3339Nano),
				info.ModTime().Sub(cachedInfo.ModTime)))
			return false
		}

		checked++
	}

	// If cache claimed to have files but we couldn't verify any, it's invalid
	if len(cache.Files) > 0 && checked == 0 {
		log(fmt.Sprintf("Cache invalid: expected to check %d files but verified 0", expectedToCheck))
		return false
	}

	log(fmt.Sprintf("Cache valid: checked %d sample files, all match", checked))
	return true
}

