package shared

import (
	"fmt"
	"time"
)

// ============================================================================
// Formatting Functions
// These are used by multiple screens for consistent display
// ============================================================================

// FormatBytes formats bytes into human-readable format (e.g., "1.5 MB")
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration into human-readable format (e.g., "2m 30s")
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// FormatRate formats transfer rate into human-readable format (e.g., "5.2 MB/s")
func FormatRate(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

// FormatPercentage formats a percentage with one decimal place
func FormatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

// FormatCount formats a count with thousand separators
func FormatCount(count int) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	if count < 1000000 {
		return fmt.Sprintf("%d,%03d", count/1000, count%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", count/1000000, (count/1000)%1000, count%1000)
}

