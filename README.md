# Copy Files - Fast File Synchronization Tool

A beautiful, fast file synchronization CLI tool with a rich Terminal User Interface (TUI) built with Go and Bubbletea.

## Features

- 🚀 **Fast file synchronization** - Efficiently copies files from source to destination
- 🎨 **Beautiful TUI** - Rich terminal interface with progress bars, animations, and colors
- 📊 **Real-time progress tracking** - See transfer speed, time remaining, and completion estimates
- 📁 **Smart sync** - Only copies files that need updating (based on size and modification time)
- 🗑️ **Clean destination** - Removes files from destination that don't exist in source
- 📈 **Multiple progress views** - Track progress per file, per session, and overall
- ⚡ **Live updates** - Constantly updating estimates based on actual throughput
- 🎯 **Interactive or CLI mode** - Use interactively or with command-line arguments

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/joe/copy-files.git
cd copy-files

# Build the binary
go build -o copy-files ./cmd/copy-files

# Or use mage
mage build

# Install to $GOPATH/bin
mage install
```

## Usage

### Interactive Mode

Simply run the command without arguments to enter interactive mode:

```bash
./copy-files
```

You'll be prompted to enter source and destination paths.

### Command-Line Mode

Specify source and destination paths directly:

```bash
./copy-files --source /path/to/source --dest /path/to/destination
# or use short flags
./copy-files -s /path/to/source -d /path/to/destination
```

### Flags

- `--source`, `-s` - Source directory path
- `--dest`, `-d` - Destination directory path
- `--interactive`, `-i` - Force interactive mode
- `--workers`, `-w` - Number of concurrent workers (default: 4, 0 = adaptive)
- `--adaptive` - Use adaptive concurrency (default: true)
- `--cache` - Use cached scan results (default: true)
- `--help`, `-h` - Display help and exit
- `--version` - Display version and exit

## What You'll See

The TUI displays:

- **Overall Progress Bar** - Shows total sync progress with percentage
- **Current File Progress** - Individual file transfer progress
- **Transfer Statistics**:
  - Files processed / total files
  - Bytes transferred / total bytes
  - Current transfer speed (MB/s)
  - Estimated time remaining
  - Estimated completion time
- **Recent Files List** - Shows recently transferred files with status indicators
  - ✓ Complete
  - ○ Pending
  - ✗ Error

## Development

### Prerequisites

- Go 1.21 or later
- golangci-lint (for linting)
- mage (for build automation)

### Building

```bash
# Build the binary
mage build

# Run tests
mage test

# Run linter
mage lint

# Format code
mage fmt

# Run all checks (fmt, lint, test)
mage check

# Generate coverage report
mage coverage

# Clean build artifacts
mage clean
```

### Project Structure

```
copy-files/
├── cmd/copy-files/     # Main application entry point
├── internal/
│   ├── config/         # Configuration and CLI parsing
│   ├── sync/           # Synchronization engine
│   └── tui/            # Terminal UI components
├── pkg/
│   └── fileops/        # File operation utilities
├── magefile.go         # Mage build tasks
└── .golangci.yml       # Linter configuration
```

## How It Works

1. **Analysis Phase** - Scans source and destination directories to determine what needs syncing
2. **Deletion Phase** - Removes files from destination that don't exist in source
3. **Sync Phase** - Copies files that are new or have changed
4. **Progress Tracking** - Updates UI in real-time with transfer statistics

Files are compared based on:
- File size
- Modification time

If either differs, the file is copied from source to destination.

## Testing

The project includes comprehensive tests for all core functionality:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Architecture (C4 Diagram)

### System Context Diagram (Level 1)
```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│                            Copy-Files System                            │
│                                                                         │
│  A fast file synchronization CLI tool with a rich Terminal UI that     │
│  copies files from source to destination, tracking progress and        │
│  providing real-time statistics.                                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
                                    │ Uses
                                    │
                        ┌───────────┴───────────┐
                        │                       │
                        │        User           │
                        │                       │
                        │  Runs the CLI tool    │
                        │  to sync files        │
                        │                       │
                        └───────────────────────┘
```

### Container Diagram (Level 2)
```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Copy-Files Application                          │
│                                                                         │
│  ┌─────────────────┐         ┌─────────────────┐                      │
│  │                 │         │                 │                      │
│  │   CLI Entry     │────────▶│   Config        │                      │
│  │   (main.go)     │ Parses  │   Parser        │                      │
│  │                 │  flags  │                 │                      │
│  └────────┬────────┘         └─────────────────┘                      │
│           │                                                             │
│           │ Creates                                                     │
│           ▼                                                             │
│  ┌─────────────────┐         ┌─────────────────┐                      │
│  │                 │         │                 │                      │
│  │   TUI Layer     │────────▶│  Sync Engine    │                      │
│  │   (Bubbletea)   │ Controls│  (sync.Engine)  │                      │
│  │                 │         │                 │                      │
│  └─────────────────┘         └────────┬────────┘                      │
│           │                           │                                │
│           │ Displays                  │ Uses                           │
│           │ Progress                  │                                │
│           │                           ▼                                │
│           │                  ┌─────────────────┐                      │
│           │                  │                 │                      │
│           └─────────────────▶│  File Ops       │                      │
│                              │  (fileops pkg)  │                      │
│                              │                 │                      │
│                              └────────┬────────┘                      │
│                                       │                                │
└───────────────────────────────────────┼────────────────────────────────┘
                                        │
                                        │ Reads/Writes
                                        ▼
                              ┌─────────────────┐
                              │                 │
                              │  File System    │
                              │  (Source/Dest)  │
                              │                 │
                              └─────────────────┘
```

### Component Diagram (Level 3)
```
┌─────────────────────────────────────────────────────────────────────────┐
│                            Sync Engine                                  │
│                                                                         │
│  ┌─────────────────┐         ┌─────────────────┐                      │
│  │                 │         │                 │                      │
│  │   Analysis      │────────▶│   File Cache    │                      │
│  │   Phase         │ Stores  │   (cache.go)    │                      │
│  │                 │  scans  │                 │                      │
│  └────────┬────────┘         └─────────────────┘                      │
│           │                                                             │
│           │ Identifies                                                  │
│           │ changes                                                     │
│           ▼                                                             │
│  ┌─────────────────┐         ┌─────────────────┐                      │
│  │                 │         │                 │                      │
│  │   Deletion      │         │   Worker Pool   │                      │
│  │   Phase         │         │   (Fixed/       │                      │
│  │                 │         │   Adaptive)     │                      │
│  └────────┬────────┘         └────────┬────────┘                      │
│           │                           ▲                                │
│           │ Removes                   │                                │
│           │ orphaned                  │ Coordinates                    │
│           │ files                     │                                │
│           ▼                           │                                │
│  ┌─────────────────┐                 │                                │
│  │                 │                 │                                │
│  │   Sync Phase    │─────────────────┘                                │
│  │                 │  Uses workers                                     │
│  │                 │                                                   │
│  └────────┬────────┘                                                   │
│           │                                                             │
│           │ Updates                                                     │
│           ▼                                                             │
│  ┌─────────────────┐                                                   │
│  │                 │                                                   │
│  │   Status        │◀───────────────────────────────────────┐         │
│  │   Tracker       │  Callbacks                             │         │
│  │                 │                                         │         │
│  └─────────────────┘                                         │         │
│                                                               │         │
└───────────────────────────────────────────────────────────────┼─────────┘
                                                                │
                                                                │
┌───────────────────────────────────────────────────────────────┼─────────┐
│                              TUI Layer                        │         │
│                                                               │         │
│  ┌─────────────────┐         ┌─────────────────┐            │         │
│  │                 │         │                 │            │         │
│  │   Input Model   │────────▶│   Main Model    │────────────┘         │
│  │   (Interactive) │ Provides│   (Bubbletea)   │  Receives            │
│  │                 │  paths  │                 │  updates             │
│  └─────────────────┘         └────────┬────────┘                      │
│                                       │                                │
│                                       │ Renders                        │
│                                       ▼                                │
│                              ┌─────────────────┐                      │
│                              │                 │                      │
│                              │   View Layer    │                      │
│                              │   (Progress     │                      │
│                              │    Bars, Stats) │                      │
│                              │                 │                      │
│                              └─────────────────┘                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key Components

- **CLI Entry (cmd/copy-files/main.go)**: Application entry point, orchestrates initialization
- **Config Parser (internal/config)**: Parses command-line flags and configuration
- **TUI Layer (internal/tui)**: Bubbletea-based terminal UI with progress tracking
- **Sync Engine (internal/sync)**: Core synchronization logic with three phases:
  - Analysis: Scans source/destination and identifies differences
  - Deletion: Removes files from destination not in source
  - Sync: Copies new/modified files using worker pool
- **File Operations (pkg/fileops)**: Low-level file I/O, copying, and scanning utilities
- **Worker Pool**: Concurrent file copying with fixed or adaptive scaling
- **Status Tracker**: Real-time progress tracking with callbacks to TUI

