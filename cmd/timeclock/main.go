package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/1kaius1/Timeclock/domain"
	"github.com/1kaius1/Timeclock/storage"
	"github.com/1kaius1/Timeclock/ui"
)

const (
	appName    = "Timeclock"
	appVersion = "1.0.0"
)

// resolveDefaultDBPath returns the OS-specific default path for Timeclock's tracker.db.
// Linux:   ~/.Timeclock/tracker.db
// macOS:   ~/Library/Application Support/Timeclock/tracker.db
// Windows: %AppData%\Timeclock\tracker.db
func resolveDefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".Timeclock", "tracker.db"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Timeclock", "tracker.db"), nil
	case "windows":
		appData := os.Getenv("AppData")
		if appData == "" {
			// Fallback to home dir if AppData missing
			return filepath.Join(home, ".Timeclock", "tracker.db"), nil
		}
		return filepath.Join(appData, "Timeclock", "tracker.db"), nil
	default:
		// Fallback for other OSes
		return filepath.Join(home, ".Timeclock", "tracker.db"), nil
	}
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0o755)
}

func main() {
	// CLI flags
	dbFlag := flag.String("db", "", "Path to tracker.db (overrides default).")
	scaleFlag := flag.Float64("scale", 0, "UI scale factor (0.5 to 3.0, overrides database setting, 0 = use database)")
	versionFlag := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	defaultPath, err := resolveDefaultDBPath()
	if err != nil {
		log.Fatalf("error resolving default db path: %v", err)
	}

	dbPath := defaultPath
	if dbFlag != nil && *dbFlag != "" {
		dbPath = *dbFlag
	}

	if err := ensureDir(dbPath); err != nil {
		log.Fatalf("failed to create db directory: %v", err)
	}

	// Open DB and run migrations
	db, err := storage.OpenAndMigrate(dbPath)
	if err != nil {
		log.Fatalf("failed to open/migrate db: %v", err)
	}
	defer db.Close()

	// Initialize domain state
	appState := domain.NewAppState(db)

	// Determine scale: flag overrides database
	var scale float32
	var scaleForced bool

	if *scaleFlag > 0 {
		// Scale provided via flag
		scale = float32(*scaleFlag)
		if scale < 0.5 || scale > 3.0 {
			log.Fatalf("scale must be between 0.5 and 3.0, got: %.2f", scale)
		}
		scaleForced = true
	} else {
		// Load scale from database
		scaleStr := storage.GetSetting(db, "scale", "1.0")
		scaleFloat, err := strconv.ParseFloat(scaleStr, 32)
		if err != nil || scaleFloat < 0.5 || scaleFloat > 3.0 {
			scaleFloat = 1.0
		}
		scale = float32(scaleFloat)
		scaleForced = false
	}
	
	// Set FYNE_SCALE environment variable before creating the app
	os.Setenv("FYNE_SCALE", fmt.Sprintf("%.2f", scale))

	// Launch Fyne UI with scale parameter
	ui.RunApp(appState, dbPath, scale, appVersion, scaleForced)
}

