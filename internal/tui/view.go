package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	fileItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	fileItemCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	fileItemCopyingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226"))

	fileItemErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))
)

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.state {
	case "initializing":
		return m.renderInitializingView()
	case "analyzing":
		return m.renderAnalyzingView()
	case "syncing":
		return m.renderSyncingView()
	case "cancelling":
		return m.renderCancellingView()
	case "complete":
		return m.renderCompleteView()
	case "cancelled":
		return m.renderCancelledView()
	case "error":
		return m.renderErrorView()
	default:
		return "Unknown state"
	}
}

func (m Model) renderInitializingView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🚀 Starting Copy Files"))
	b.WriteString("\n\n")

	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	b.WriteString(labelStyle.Render("Initializing..."))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("Setting up file logging and preparing to analyze directories"))
	b.WriteString("\n")

	return boxStyle.Render(b.String())
}

func (m Model) renderAnalyzingView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 Analyzing Files"))
	b.WriteString("\n\n")

	if m.status != nil {
		// Show current phase
		var phaseText string
		switch m.status.AnalysisPhase {
		case "counting_source":
			phaseText = "Counting files in source..."
		case "scanning_source":
			phaseText = "Scanning source directory..."
		case "counting_dest":
			phaseText = "Counting files in destination..."
		case "scanning_dest":
			phaseText = "Scanning destination directory..."
		case "comparing":
			phaseText = "Comparing files to determine sync plan..."
		case "deleting":
			phaseText = "Checking for files to delete..."
		case "complete":
			phaseText = "Analysis complete!"
		default:
			phaseText = "Initializing..."
		}

		b.WriteString(m.spinner.View())
		b.WriteString(" ")
		b.WriteString(labelStyle.Render(phaseText))
		b.WriteString("\n\n")

		// Show scan progress with progress bar or count
		if m.status.AnalysisPhase == "counting_source" || m.status.AnalysisPhase == "counting_dest" {
			// Counting phase - show count so far
			if m.status.ScannedFiles > 0 {
				b.WriteString(fmt.Sprintf("Found: %d items so far...\n\n", m.status.ScannedFiles))
			}
		} else if m.status.AnalysisPhase == "scanning_source" || m.status.AnalysisPhase == "scanning_dest" ||
			m.status.AnalysisPhase == "comparing" || m.status.AnalysisPhase == "deleting" {
			if m.status.TotalFilesToScan > 0 {
				// Show progress bar
				scanPercent := float64(m.status.ScannedFiles) / float64(m.status.TotalFilesToScan)
				b.WriteString(m.overallProgress.ViewAs(scanPercent))
				b.WriteString("\n")
				b.WriteString(fmt.Sprintf("%d / %d items (%.1f%%)\n\n",
					m.status.ScannedFiles,
					m.status.TotalFilesToScan,
					scanPercent*100))
			} else if m.status.ScannedFiles > 0 {
				// Fallback: show count without progress bar
				b.WriteString(fmt.Sprintf("Processed: %d items\n\n", m.status.ScannedFiles))
			}
		}

		// Show current path being scanned
		if m.status.CurrentPath != "" {
			b.WriteString(fmt.Sprintf("Current: %s\n", m.status.CurrentPath))
			b.WriteString("\n")
		}

		// Show errors if any
		if len(m.status.Errors) > 0 {
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ Errors: %d", len(m.status.Errors))))
			b.WriteString("\n\n")
		}

		// Show analysis log
		if len(m.status.AnalysisLog) > 0 {
			b.WriteString(labelStyle.Render("Activity Log:"))
			b.WriteString("\n")

			// Show last 10 log entries
			startIdx := 0
			if len(m.status.AnalysisLog) > 10 {
				startIdx = len(m.status.AnalysisLog) - 10
			}

			for i := startIdx; i < len(m.status.AnalysisLog); i++ {
				b.WriteString(fmt.Sprintf("  %s\n", m.status.AnalysisLog[i]))
			}
		}
	} else {
		b.WriteString(m.spinner.View())
		b.WriteString(" Scanning directories and comparing files...\n\n")
	}

	return boxStyle.Render(b.String())
}

func (m Model) renderSyncingView() string {
	if m.status == nil {
		return "Loading..."
	}

	var b strings.Builder

	// Show different title based on finalization phase
	if m.status.FinalizationPhase == "updating_cache" {
		b.WriteString(titleStyle.Render("📦 Finalizing..."))
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("Updating destination cache..."))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("(This helps the next sync run faster)"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(titleStyle.Render("📦 Syncing Files"))
		b.WriteString("\n\n")
	}

	// Overall progress (all files including already synced)
	b.WriteString(labelStyle.Render("Overall Progress (All Files):"))
	b.WriteString("\n")

	var totalOverallPercent float64
	if m.status.TotalBytesInSource > 0 {
		// Already synced bytes + transferred bytes this session
		totalProcessedBytes := m.status.AlreadySyncedBytes + m.status.TransferredBytes
		totalOverallPercent = float64(totalProcessedBytes) / float64(m.status.TotalBytesInSource)
	}
	b.WriteString(m.overallProgress.ViewAs(totalOverallPercent))
	b.WriteString("\n")

	totalProcessedFiles := m.status.AlreadySyncedFiles + m.status.ProcessedFiles
	b.WriteString(fmt.Sprintf("%d / %d files (%.1f%%) • %s / %s\n\n",
		totalProcessedFiles,
		m.status.TotalFilesInSource,
		totalOverallPercent*100,
		formatBytes(m.status.AlreadySyncedBytes + m.status.TransferredBytes),
		formatBytes(m.status.TotalBytesInSource)))

	// Session progress (only files being copied this session)
	b.WriteString(labelStyle.Render("This Session:"))
	b.WriteString("\n")

	var sessionPercent float64
	if m.status.TotalBytes > 0 {
		sessionPercent = float64(m.status.TransferredBytes) / float64(m.status.TotalBytes)
	}
	b.WriteString(m.fileProgress.ViewAs(sessionPercent))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%d / %d files (%.1f%%) • %s / %s",
		m.status.ProcessedFiles,
		m.status.TotalFiles,
		sessionPercent*100,
		formatBytes(m.status.TransferredBytes),
		formatBytes(m.status.TotalBytes)))

	if m.status.FailedFiles > 0 {
		b.WriteString(fmt.Sprintf(" (%d failed)", m.status.FailedFiles))
	}
	b.WriteString("\n\n")

	// Statistics
	b.WriteString(labelStyle.Render("Statistics:"))
	b.WriteString("\n")

	if m.status.AlreadySyncedFiles > 0 {
		b.WriteString(fmt.Sprintf("Already synced: %d files (%s)\n",
			m.status.AlreadySyncedFiles,
			formatBytes(m.status.AlreadySyncedBytes)))
	}

	// Show worker count with bottleneck info
	if m.status.AdaptiveMode {
		bottleneckInfo := ""
		if m.status.Bottleneck != "" {
			switch m.status.Bottleneck {
			case "source":
				bottleneckInfo = " 🔴 source-limited"
			case "destination":
				bottleneckInfo = " 🟡 dest-limited"
			case "balanced":
				bottleneckInfo = " 🟢 balanced"
			}
		}
		b.WriteString(fmt.Sprintf("Workers: %d (adaptive, max: %d)%s\n",
			m.status.ActiveWorkers, m.status.MaxWorkers, bottleneckInfo))
	} else {
		b.WriteString(fmt.Sprintf("Workers: %d\n", m.status.ActiveWorkers))
	}

	// Calculate elapsed time - use EndTime if sync is complete, otherwise current time
	var elapsed float64
	if !m.status.EndTime.IsZero() {
		elapsed = m.status.EndTime.Sub(m.status.StartTime).Seconds()
	} else {
		elapsed = time.Since(m.status.StartTime).Seconds()
	}

	var currentSpeed float64
	var timeRemaining time.Duration
	var completionTime time.Time

	if elapsed > 0 && m.status.TransferredBytes > 0 {
		currentSpeed = float64(m.status.TransferredBytes) / elapsed
		b.WriteString(fmt.Sprintf("Speed: %s/s\n", formatBytes(int64(currentSpeed))))

		if currentSpeed > 0 && m.status.EndTime.IsZero() {
			// Only show time remaining if sync is still in progress
			remainingBytes := m.status.TotalBytes - m.status.TransferredBytes
			timeRemaining = time.Duration(float64(remainingBytes)/currentSpeed) * time.Second
			completionTime = time.Now().Add(timeRemaining)

			b.WriteString(fmt.Sprintf("Time Remaining: %s\n", formatDuration(timeRemaining)))
			b.WriteString(fmt.Sprintf("Estimated Completion: %s\n", completionTime.Format("15:04:05")))
		}
	} else {
		b.WriteString(fmt.Sprintf("Speed: %s/s\n", formatBytes(int64(m.status.BytesPerSecond))))
	}

	// Show I/O time breakdown to explain bottleneck
	if m.status.AdaptiveMode && m.status.TotalReadTime > 0 && m.status.TotalWriteTime > 0 {
		totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
		if totalIOTime > 0 {
			readPercent := float64(m.status.TotalReadTime) / float64(totalIOTime) * 100
			writePercent := float64(m.status.TotalWriteTime) / float64(totalIOTime) * 100

			b.WriteString(fmt.Sprintf("I/O time: %.1f%% read, %.1f%% write\n",
				readPercent, writePercent))
		}
	}

	b.WriteString("\n")

	// Calculate how many file progress bars we can show based on available screen height
	// Count lines used so far (approximate)
	linesUsed := 0
	linesUsed += 2  // Title
	linesUsed += 4  // Overall progress section
	linesUsed += 4  // Session progress section
	linesUsed += 8  // Statistics section (varies, but estimate)
	linesUsed += 2  // Section header
	linesUsed += 5  // Error section (if shown)
	linesUsed += 2  // Bottom padding

	// Each file takes 3 lines (filename + progress bar + blank line)
	linesPerFile := 3
	availableLines := m.height - linesUsed
	if availableLines < 0 {
		availableLines = 0
	}
	maxFilesToShow := availableLines / linesPerFile
	if maxFilesToShow < 1 {
		maxFilesToShow = 1 // Always show at least 1 file
	}

	// Currently copying files with progress bars
	if len(m.status.CurrentFiles) > 0 {
		// Count how many files are actually copying and display them
		totalCopying := 0
		filesDisplayed := 0

		b.WriteString(labelStyle.Render(fmt.Sprintf("Currently Copying (%d):", len(m.status.CurrentFiles))))
		b.WriteString("\n")

		// Display up to maxFilesToShow files
		for _, file := range m.status.FilesToSync {
			if file.Status == "copying" {
				totalCopying++

				if filesDisplayed < maxFilesToShow {
					b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), fileItemCopyingStyle.Render(file.RelativePath)))

					// Show progress bar for this file
					var filePercent float64
					if file.Size > 0 {
						filePercent = float64(file.Transferred) / float64(file.Size)
					}
					b.WriteString("  ")
					b.WriteString(m.fileProgress.ViewAs(filePercent))
					b.WriteString("\n")

					filesDisplayed++
				}
			}
		}

		// Show how many more files are being copied but not displayed
		if totalCopying > filesDisplayed {
			b.WriteString(dimStyle.Render(fmt.Sprintf("... and %d more files\n", totalCopying-filesDisplayed)))
		}
	} else {
		// Show recent files when nothing is currently copying
		b.WriteString(labelStyle.Render("Recent Files:"))
		b.WriteString("\n")

		maxFiles := 5
		if maxFiles > maxFilesToShow {
			maxFiles = maxFilesToShow
		}
		startIdx := len(m.status.FilesToSync) - maxFiles
		if startIdx < 0 {
			startIdx = 0
		}

		for i := startIdx; i < len(m.status.FilesToSync) && i < startIdx+maxFiles; i++ {
			file := m.status.FilesToSync[i]
			var style lipgloss.Style
			var icon string

			switch file.Status {
			case "complete":
				style = fileItemCompleteStyle
				icon = "✓"
			case "copying":
				style = fileItemCopyingStyle
				icon = m.spinner.View()
			case "error":
				style = fileItemErrorStyle
				icon = "✗"
			default:
				style = fileItemStyle
				icon = "○"
			}

			b.WriteString(fmt.Sprintf("%s %s\n", icon, style.Render(file.RelativePath)))
		}
	}

	// Show errors if any
	if len(m.status.Errors) > 0 {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ Errors (%d):", len(m.status.Errors))))
		b.WriteString("\n")

		// Show up to 5 most recent errors
		maxErrors := 5
		startIdx := 0
		if len(m.status.Errors) > maxErrors {
			startIdx = len(m.status.Errors) - maxErrors
		}

		for i := startIdx; i < len(m.status.Errors); i++ {
			fileErr := m.status.Errors[i]
			b.WriteString(fmt.Sprintf("  ✗ %s\n", fileErr.FilePath))
			// Truncate error message if too long
			errMsg := fileErr.Error.Error()
			if len(errMsg) > 60 {
				errMsg = errMsg[:57] + "..."
			}
			b.WriteString(fmt.Sprintf("    %s\n", errMsg))
		}

		if len(m.status.Errors) > maxErrors {
			b.WriteString(fmt.Sprintf("  ... and %d more (see completion screen)\n", len(m.status.Errors)-maxErrors))
		}
	}

	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Press Ctrl+C to cancel"))

	return boxStyle.Render(b.String())
}

func (m Model) renderCompleteView() string {
	var b strings.Builder

	// Show different title based on whether there were errors
	if m.status != nil && m.status.FailedFiles > 0 {
		b.WriteString(errorStyle.Render("⚠ Sync Complete with Errors"))
	} else {
		b.WriteString(successStyle.Render("✓ Sync Complete!"))
	}
	b.WriteString("\n\n")

	if m.status != nil {
		// Use EndTime if available, otherwise fall back to current time
		endTime := m.status.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}
		elapsed := endTime.Sub(m.status.StartTime)

		// Overall summary
		b.WriteString(labelStyle.Render("Summary:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Total files in source: %d (%s)\n",
			m.status.TotalFilesInSource,
			formatBytes(m.status.TotalBytesInSource)))

		if m.status.AlreadySyncedFiles > 0 {
			b.WriteString(fmt.Sprintf("Already up-to-date: %d files (%s)\n",
				m.status.AlreadySyncedFiles,
				formatBytes(m.status.AlreadySyncedBytes)))
		}

		b.WriteString("\n")
		b.WriteString(labelStyle.Render("This Session:"))
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("Files synced successfully: %d\n", m.status.ProcessedFiles))
		if m.status.CancelledFiles > 0 {
			b.WriteString(fmt.Sprintf("Files cancelled: %d\n", m.status.CancelledFiles))
		}
		if m.status.FailedFiles > 0 {
			b.WriteString(fmt.Sprintf("Files failed: %d\n", m.status.FailedFiles))
		}
		b.WriteString(fmt.Sprintf("Total files to copy: %d\n", m.status.TotalFiles))
		b.WriteString(fmt.Sprintf("Total bytes to copy: %s\n", formatBytes(m.status.TotalBytes)))
		b.WriteString(fmt.Sprintf("Time elapsed: %s\n", formatDuration(elapsed)))

		// Calculate average speed based on actual elapsed time
		if elapsed.Seconds() > 0 {
			avgSpeed := float64(m.status.TotalBytes) / elapsed.Seconds()
			b.WriteString(fmt.Sprintf("Average speed: %s/s\n", formatBytes(int64(avgSpeed))))
		}

		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Statistics:"))
		b.WriteString("\n")

		// Show worker count with bottleneck info
		if m.status.AdaptiveMode {
			bottleneckInfo := ""
			if m.status.Bottleneck != "" {
				switch m.status.Bottleneck {
				case "source":
					bottleneckInfo = " 🔴 source-limited"
				case "destination":
					bottleneckInfo = " 🟡 dest-limited"
				case "balanced":
					bottleneckInfo = " 🟢 balanced"
				}
			}
			b.WriteString(fmt.Sprintf("Workers: %d (adaptive, max: %d)%s\n",
				m.status.ActiveWorkers, m.status.MaxWorkers, bottleneckInfo))
		} else {
			b.WriteString(fmt.Sprintf("Workers: %d\n", m.status.ActiveWorkers))
		}

		// Show read/write speeds to explain bottleneck
		if m.status.AdaptiveMode && m.status.TotalReadTime > 0 && m.status.TotalWriteTime > 0 {
			totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
			if totalIOTime > 0 && m.status.TransferredBytes > 0 {
				// Calculate effective speeds based on time spent
				readSpeed := float64(m.status.TransferredBytes) / m.status.TotalReadTime.Seconds()
				writeSpeed := float64(m.status.TransferredBytes) / m.status.TotalWriteTime.Seconds()

				b.WriteString(fmt.Sprintf("Read speed: %s/s • Write speed: %s/s\n",
					formatBytes(int64(readSpeed)),
					formatBytes(int64(writeSpeed))))
			}
		}

		// Show recently completed files
		if len(m.status.RecentlyCompleted) > 0 {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("Recently Completed:"))
			b.WriteString("\n")
			for _, file := range m.status.RecentlyCompleted {
				// Truncate long paths
				displayPath := file
				if len(displayPath) > 60 {
					displayPath = "..." + displayPath[len(displayPath)-57:]
				}
				b.WriteString(fmt.Sprintf("  ✓ %s\n", displayPath))
			}
		}

		// Show adaptive concurrency stats if used
		if m.status.AdaptiveMode && m.status.MaxWorkers > 0 {
			b.WriteString(fmt.Sprintf("Max workers used: %d\n", m.status.MaxWorkers))

			// Show bottleneck analysis
			if m.status.TotalReadTime > 0 || m.status.TotalWriteTime > 0 {
				totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
				readPercent := float64(m.status.TotalReadTime) / float64(totalIOTime) * 100
				writePercent := float64(m.status.TotalWriteTime) / float64(totalIOTime) * 100

				b.WriteString(fmt.Sprintf("I/O breakdown: %.1f%% read, %.1f%% write", readPercent, writePercent))
				if m.status.Bottleneck != "" {
					switch m.status.Bottleneck {
					case "source":
						b.WriteString(" (source-limited)")
					case "destination":
						b.WriteString(" (dest-limited)")
					case "balanced":
						b.WriteString(" (balanced)")
					}
				}
				b.WriteString("\n")
			}
		}

		// Show error details if any
		if len(m.status.Errors) > 0 {
			b.WriteString("\n")
			b.WriteString(errorStyle.Render("Errors:"))
			b.WriteString("\n")

			// Show up to 10 errors
			maxErrors := 10
			for i, fileErr := range m.status.Errors {
				if i >= maxErrors {
					remaining := len(m.status.Errors) - maxErrors
					b.WriteString(fmt.Sprintf("... and %d more error(s)\n", remaining))
					break
				}
				b.WriteString(fmt.Sprintf("  ✗ %s: %v\n", fileErr.FilePath, fileErr.Error))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Press Enter or Ctrl+C to exit"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Debug log saved to: copy-files-debug.log"))

	return boxStyle.Render(b.String())
}


func (m Model) renderCancellingView() string {
	var b strings.Builder

	b.WriteString(errorStyle.Render("⚠ Cancelling..."))
	b.WriteString("\n\n")

	if m.status != nil {
		// Show different message based on finalization phase
		if m.status.FinalizationPhase == "updating_cache" {
			b.WriteString(labelStyle.Render("Updating destination cache..."))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render("(This helps the next sync run faster)"))
			b.WriteString("\n\n")
		} else if len(m.status.CurrentFiles) > 0 {
			b.WriteString(labelStyle.Render("Finishing current files, please wait..."))
			b.WriteString("\n\n")
		} else {
			b.WriteString(labelStyle.Render("Cleaning up..."))
			b.WriteString("\n\n")
		}

		// Show current progress
		b.WriteString(fmt.Sprintf("Files completed: %d / %d\n", m.status.ProcessedFiles, m.status.TotalFiles))
		b.WriteString(fmt.Sprintf("Bytes transferred: %s / %s\n",
			formatBytes(m.status.TransferredBytes),
			formatBytes(m.status.TotalBytes)))

		// Calculate and show current speed - use EndTime if available
		var elapsed float64
		if !m.status.EndTime.IsZero() {
			elapsed = m.status.EndTime.Sub(m.status.StartTime).Seconds()
		} else {
			elapsed = time.Since(m.status.StartTime).Seconds()
		}

		if elapsed > 0 && m.status.TransferredBytes > 0 {
			currentSpeed := float64(m.status.TransferredBytes) / elapsed
			b.WriteString(fmt.Sprintf("Average speed: %s/s\n", formatBytes(int64(currentSpeed))))
		}

		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Statistics:"))
		b.WriteString("\n")

		// Show worker count with bottleneck info
		if m.status.AdaptiveMode {
			bottleneckInfo := ""
			if m.status.Bottleneck != "" {
				switch m.status.Bottleneck {
				case "source":
					bottleneckInfo = " 🔴 source-limited"
				case "destination":
					bottleneckInfo = " 🟡 dest-limited"
				case "balanced":
					bottleneckInfo = " 🟢 balanced"
				}
			}
			b.WriteString(fmt.Sprintf("Workers: %d (adaptive, max: %d)%s\n",
				m.status.ActiveWorkers, m.status.MaxWorkers, bottleneckInfo))
		} else {
			b.WriteString(fmt.Sprintf("Workers: %d\n", m.status.ActiveWorkers))
		}

		// Show I/O time breakdown to explain bottleneck
		if m.status.AdaptiveMode && m.status.TotalReadTime > 0 && m.status.TotalWriteTime > 0 {
			totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
			if totalIOTime > 0 {
				readPercent := float64(m.status.TotalReadTime) / float64(totalIOTime) * 100
				writePercent := float64(m.status.TotalWriteTime) / float64(totalIOTime) * 100

				b.WriteString(fmt.Sprintf("I/O time: %.1f%% read, %.1f%% write\n",
					readPercent, writePercent))
			}
		}

		// Show files currently being copied
		if len(m.status.CurrentFiles) > 0 {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("Finishing:"))
			b.WriteString("\n")
			for _, file := range m.status.CurrentFiles {
				displayPath := file
				if len(displayPath) > 60 {
					displayPath = "..." + displayPath[len(displayPath)-57:]
				}
				b.WriteString(fmt.Sprintf("  ⏳ %s\n", displayPath))
			}

			b.WriteString("\n")
			b.WriteString(dimStyle.Render("Workers will stop after completing current files..."))
		}
	}

	return boxStyle.Render(b.String())
}

func (m Model) renderCancelledView() string {
	var b strings.Builder

	b.WriteString(errorStyle.Render("⚠ Sync Cancelled"))
	b.WriteString("\n\n")

	if m.status != nil {
		// Calculate elapsed time
		endTime := time.Now()
		elapsed := endTime.Sub(m.status.StartTime)

		// Overall summary
		b.WriteString(labelStyle.Render("Summary:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Total files in source: %d (%s)\n",
			m.status.TotalFilesInSource,
			formatBytes(m.status.TotalBytesInSource)))

		if m.status.AlreadySyncedFiles > 0 {
			b.WriteString(fmt.Sprintf("Already up-to-date: %d files (%s)\n",
				m.status.AlreadySyncedFiles,
				formatBytes(m.status.AlreadySyncedBytes)))
		}

		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Progress Before Cancellation:"))
		b.WriteString("\n")

		if m.status.ProcessedFiles > 0 || m.status.CancelledFiles > 0 {
			b.WriteString(fmt.Sprintf("Files synced successfully: %d\n", m.status.ProcessedFiles))
			if m.status.CancelledFiles > 0 {
				b.WriteString(fmt.Sprintf("Files cancelled: %d\n", m.status.CancelledFiles))
			}
			if m.status.FailedFiles > 0 {
				b.WriteString(fmt.Sprintf("Files failed: %d\n", m.status.FailedFiles))
			}
			b.WriteString(fmt.Sprintf("Total files to copy: %d\n", m.status.TotalFiles))
			b.WriteString(fmt.Sprintf("Bytes transferred: %s / %s\n",
				formatBytes(m.status.TransferredBytes),
				formatBytes(m.status.TotalBytes)))

			if elapsed.Seconds() > 0 {
				avgSpeed := float64(m.status.TransferredBytes) / elapsed.Seconds()
				b.WriteString(fmt.Sprintf("Average speed: %s/s\n", formatBytes(int64(avgSpeed))))
			}
			b.WriteString(fmt.Sprintf("Time elapsed: %s\n", formatDuration(elapsed)))

			b.WriteString("\n")
			b.WriteString(labelStyle.Render("Statistics:"))
			b.WriteString("\n")

			// Show worker count with bottleneck info
			if m.status.AdaptiveMode {
				bottleneckInfo := ""
				if m.status.Bottleneck != "" {
					switch m.status.Bottleneck {
					case "source":
						bottleneckInfo = " 🔴 source-limited"
					case "destination":
						bottleneckInfo = " 🟡 dest-limited"
					case "balanced":
						bottleneckInfo = " 🟢 balanced"
					}
				}
				b.WriteString(fmt.Sprintf("Workers: %d (adaptive, max: %d)%s\n",
					m.status.ActiveWorkers, m.status.MaxWorkers, bottleneckInfo))
			} else {
				b.WriteString(fmt.Sprintf("Workers: %d\n", m.status.ActiveWorkers))
			}

			// Show I/O time breakdown to explain bottleneck
			if m.status.AdaptiveMode && m.status.TotalReadTime > 0 && m.status.TotalWriteTime > 0 {
				totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
				if totalIOTime > 0 {
					readPercent := float64(m.status.TotalReadTime) / float64(totalIOTime) * 100
					writePercent := float64(m.status.TotalWriteTime) / float64(totalIOTime) * 100

					b.WriteString(fmt.Sprintf("I/O time: %.1f%% read, %.1f%% write\n",
						readPercent, writePercent))
				}
			}

			// Show recently completed files
			if len(m.status.RecentlyCompleted) > 0 {
				b.WriteString("\n")
				b.WriteString(labelStyle.Render("Recently Completed:"))
				b.WriteString("\n")
				for _, file := range m.status.RecentlyCompleted {
					// Truncate long paths
					displayPath := file
					if len(displayPath) > 60 {
						displayPath = "..." + displayPath[len(displayPath)-57:]
					}
					b.WriteString(fmt.Sprintf("  ✓ %s\n", displayPath))
				}
			}
		} else {
			b.WriteString("No files were transferred before cancellation.\n")
		}

		// Show what's remaining
		if m.status.TotalFiles > m.status.ProcessedFiles {
			remaining := m.status.TotalFiles - m.status.ProcessedFiles
			remainingBytes := m.status.TotalBytes - m.status.TransferredBytes
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("Not Completed:"))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("Files not synced: %d (%s)\n", remaining, formatBytes(remainingBytes)))
		}

		// Show errors if any
		if len(m.status.Errors) > 0 {
			b.WriteString("\n")
			b.WriteString(errorStyle.Render(fmt.Sprintf("Errors (%d):", len(m.status.Errors))))
			b.WriteString("\n")

			// Show up to 10 errors
			maxErrors := 10
			for i, fileErr := range m.status.Errors {
				if i >= maxErrors {
					remaining := len(m.status.Errors) - maxErrors
					b.WriteString(fmt.Sprintf("... and %d more error(s)\n", remaining))
					break
				}
				b.WriteString(fmt.Sprintf("  ✗ %s: %v\n", fileErr.FilePath, fileErr.Error))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Press Enter or Ctrl+C to exit"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Debug log saved to: copy-files-debug.log"))

	return boxStyle.Render(b.String())
}

func (m Model) renderErrorView() string {
	var b strings.Builder

	b.WriteString(errorStyle.Render("✗ Error"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(m.err.Error())
	}

	// Show error details if we have status
	if m.status != nil && len(m.status.Errors) > 0 {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Encountered %d error(s):", len(m.status.Errors))))
		b.WriteString("\n")

		// Show up to 10 errors
		maxErrors := 10
		for i, fileErr := range m.status.Errors {
			if i >= maxErrors {
				remaining := len(m.status.Errors) - maxErrors
				b.WriteString(fmt.Sprintf("\n... and %d more error(s)", remaining))
				break
			}
			b.WriteString(fmt.Sprintf("  ✗ %s\n    %v\n", fileErr.FilePath, fileErr.Error))
		}
	}

	// Show statistics if we have status and sync started
	if m.status != nil && m.status.TotalFiles > 0 {
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Progress before error:"))
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("Files completed: %d / %d\n", m.status.ProcessedFiles, m.status.TotalFiles))
		if m.status.FailedFiles > 0 {
			b.WriteString(fmt.Sprintf("Files failed: %d\n", m.status.FailedFiles))
		}
		if m.status.CancelledFiles > 0 {
			b.WriteString(fmt.Sprintf("Files cancelled: %d\n", m.status.CancelledFiles))
		}
		b.WriteString(fmt.Sprintf("Bytes transferred: %s / %s\n",
			formatBytes(m.status.TransferredBytes),
			formatBytes(m.status.TotalBytes)))

		// Calculate elapsed time
		var elapsed time.Duration
		if !m.status.EndTime.IsZero() {
			elapsed = m.status.EndTime.Sub(m.status.StartTime)
		} else if !m.status.StartTime.IsZero() {
			elapsed = time.Since(m.status.StartTime)
		}

		if elapsed > 0 && m.status.TransferredBytes > 0 {
			avgSpeed := float64(m.status.TransferredBytes) / elapsed.Seconds()
			b.WriteString(fmt.Sprintf("Average speed: %s/s\n", formatBytes(int64(avgSpeed))))
		}

		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Statistics:"))
		b.WriteString("\n")

		// Show worker count with bottleneck info
		if m.status.AdaptiveMode {
			bottleneckInfo := ""
			if m.status.Bottleneck != "" {
				switch m.status.Bottleneck {
				case "source":
					bottleneckInfo = " 🔴 source-limited"
				case "destination":
					bottleneckInfo = " 🟡 dest-limited"
				case "balanced":
					bottleneckInfo = " 🟢 balanced"
				}
			}
			b.WriteString(fmt.Sprintf("Workers: %d (adaptive, max: %d)%s\n",
				m.status.ActiveWorkers, m.status.MaxWorkers, bottleneckInfo))
		} else {
			b.WriteString(fmt.Sprintf("Workers: %d\n", m.status.ActiveWorkers))
		}

		// Show I/O time breakdown to explain bottleneck
		if m.status.AdaptiveMode && m.status.TotalReadTime > 0 && m.status.TotalWriteTime > 0 {
			totalIOTime := m.status.TotalReadTime + m.status.TotalWriteTime
			if totalIOTime > 0 {
				readPercent := float64(m.status.TotalReadTime) / float64(totalIOTime) * 100
				writePercent := float64(m.status.TotalWriteTime) / float64(totalIOTime) * 100

				b.WriteString(fmt.Sprintf("I/O time: %.1f%% read, %.1f%% write\n",
					readPercent, writePercent))
			}
		}
	}

	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Press Enter or Ctrl+C to exit"))

	return boxStyle.Render(b.String())
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
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

// formatDuration formats duration into human-readable format
func formatDuration(d time.Duration) string {
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

