# Timeclock

A simple, cross-platform time tracking application built with Go and Fyne. Track work sessions with automatic day-boundary handling and generate detailed reports.

## Features

- **Session Management**: Start, pause, resume, and stop work sessions
- **Category Tracking**: Organize work by categories (Task, Project, Training, Mentoring, Incident, Major Incident)
- **Automatic Day Slicing**: Work sessions that span midnight are automatically split into appropriate days
- **Flexible Reporting**: Generate reports by date range with totals per category
- **Presence Tracking**: View which days had any work activity
- **Local SQLite Storage**: All data stored locally in a SQLite database
- **Cross-Platform**: Runs on Linux, Windows, and macOS (Apple Silicon)
- **Configurable UI Scaling**: Adjust interface scale for different display resolutions

## Screenshots

*(insert screenshots here)*

## Installation

### Pre-built Binaries

Pre-built binaries aren't currently available, but if the project becomes popular enough I'll add them.

### Building from Source

#### Prerequisites

- Go 1.21 or later
- Platform-specific build dependencies (see below)

#### Linux

```bash
# Install dependencies
make deps-linux

# Build
make build-linux

# Binary will be at: bin/timeclock-linux-amd64
```

#### macOS (Apple Silicon)

```bash
# Install dependencies (requires Homebrew)
make deps-darwin

# Build
make build-darwin

# Binary will be at: bin/timeclock-darwin-arm64
```

#### Windows (cross-compile from Linux)

```bash
# Install cross-compilation tools
make deps-win32

# Build
make build-windows

# Binary will be at: bin/timeclock-windows-amd64.exe
```

## Usage

### Basic Usage

```bash
# Run with default settings
./timeclock

# Specify custom database location
./timeclock -db /path/to/tracker.db

# Set UI scale (0.5 to 3.0)
./timeclock -scale 1.5
```

### Command-Line Options

- `-db <path>` - Path to SQLite database file (default: OS-specific)
  - Linux: `~/.Timeclock/tracker.db`
  - macOS: `~/Library/Application Support/Timeclock/tracker.db`
  - Windows: `%AppData%\Timeclock\tracker.db`
- `-scale <float>` - UI scale factor, range 0.5-3.0 (default: 1.0)

### Workflow

1. **Start Work**: Enter a description and select a category, then click "Start Work"
2. **Pause**: Click "Pause Work" to temporarily stop the timer (session remains active)
3. **Resume**: Click "Resume Work" to continue the same session
4. **Stop**: Click "Stop Work" to end the session completely
5. **Reports**: Switch to the Reports tab to view time totals by category and presence days

## Database Schema

Timeclock uses SQLite with three main tables:

- **events**: Audit log of all state changes (START, PAUSE, RESUME, STOP)
- **intervals**: Time intervals with start/end timestamps
- **interval_days**: Materialized view of intervals split by local date for fast reporting

## Development

### Project Structure

```
Timeclock/
├── cmd/timeclock/     # Main application entry point
│   └── main.go
├── domain/            # Business logic and state management
│   └── state.go
├── storage/           # Database operations and migrations
│   └── db.go
├── ui/                # Fyne GUI implementation
│   └── app.go
├── reporting/         # Report generation
│   └── report.go
└── packaging/         # Debian packaging files
    └── debian/
```

### Building Debian Package

```bash
make deb
```

This creates `timeclock_1.0.0_amd64.deb` in the project root.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

If you encounter any issues or have questions, please [open an issue](https://github.com/1kaius1/Timeclock/issues) on GitHub.


