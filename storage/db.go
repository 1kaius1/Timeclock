
package storage

import (
    "database/sql"
    "fmt"
    "path/filepath"
    "time"

    _ "modernc.org/sqlite"
)

// OpenAndMigrate opens SQLite database and runs migrations.
// It sets PRAGMA user_version for schema versioning.
func OpenAndMigrate(dbPath string) (*sql.DB, error) {
    // Modernc sqlite uses file path as DSN; ensure absolute path for clarity.
    abs := dbPath
    if !filepath.IsAbs(dbPath) {
        var err error
        abs, err = filepath.Abs(dbPath)
        if err != nil {
            return nil, fmt.Errorf("cannot resolve absolute path: %w", err)
        }
    }

    db, err := sql.Open("sqlite", abs)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }

    if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
        return nil, fmt.Errorf("enable foreign keys: %w", err)
    }

    // Run migrations
    if err := migrate(db); err != nil {
        db.Close()
        return nil, err
    }
    return db, nil
}

func migrate(db *sql.DB) error {
    // Read current version
    var userVersion int
    if err := db.QueryRow(`PRAGMA user_version;`).Scan(&userVersion); err != nil {
        return fmt.Errorf("read user_version: %w", err)
    }

    // Version 1: create events, intervals, interval_days
    if userVersion < 1 {
        tx, err := db.Begin()
        if err != nil {
            return err
        }
        defer tx.Rollback()

        // Event log: ground truth audit
        if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT NOT NULL,
    timestamp_utc  INTEGER NOT NULL, -- epoch seconds
    action         TEXT NOT NULL CHECK (action IN ('START','PAUSE','RESUME','STOP')),
    category       TEXT NOT NULL,
    description    TEXT,
    user_tz        TEXT
);`); err != nil {
            return fmt.Errorf("create events: %w", err)
        }

        // Intervals: open/close slices
        if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS intervals (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id       TEXT NOT NULL,
    interval_index   INTEGER NOT NULL,
    start_utc        INTEGER NOT NULL,
    end_utc          INTEGER,            -- NULL until closed
    category         TEXT NOT NULL,
    description      TEXT,
    duration_seconds INTEGER             -- set when closed
);`); err != nil {
            return fmt.Errorf("create intervals: %w", err)
        }

        // Daily materialization: fast reporting by day/week/month
        if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS interval_days (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    interval_id      INTEGER NOT NULL,
    session_id       TEXT NOT NULL,
    date_local       TEXT NOT NULL,      -- 'YYYY-MM-DD'
    category         TEXT NOT NULL,
    description      TEXT,
    duration_seconds INTEGER NOT NULL,
    FOREIGN KEY (interval_id) REFERENCES intervals(id) ON DELETE CASCADE
);`); err != nil {
            return fmt.Errorf("create interval_days: %w", err)
        }

        if _, err := tx.Exec(`PRAGMA user_version = 1;`); err != nil {
            return fmt.Errorf("set user_version: %w", err)
        }

        if err := tx.Commit(); err != nil {
            return fmt.Errorf("commit migration v1: %w", err)
        }
    }

    return nil
}

// InsertEvent writes an event row.
// We store user_tz as best-effort (system tz name) for debugging. Not required for logic.
func InsertEvent(db *sql.DB, sessionID string, whenUTC time.Time, action, category, description string) error {
    userTZName := time.Local.String() // e.g., "Local" or a location name depending on system config

    _, err := db.Exec(`
INSERT INTO events (session_id, timestamp_utc, action, category, description, user_tz)
VALUES (?, ?, ?, ?, ?, ?);
`, sessionID, whenUTC.Unix(), action, category, description, userTZName)
    return err
}

// OpenInterval inserts a new open interval row.
func OpenInterval(db *sql.DB, sessionID string, intervalIndex int, startUTC time.Time, category, description string) error {
    _, err := db.Exec(`
INSERT INTO intervals (session_id, interval_index, start_utc, category, description)
VALUES (?, ?, ?, ?, ?);
`, sessionID, intervalIndex, startUTC.Unix(), category, description)
    return err
}

// CloseOpenIntervalAndSliceDays finds the open interval for the given session, closes it,
// writes duration, and slices into interval_days across local midnight boundaries.
// If multiple open intervals exist (shouldn't), it closes the latest one.
func CloseOpenIntervalAndSliceDays(db *sql.DB, sessionID string, startUTC, endUTC time.Time, category, description string) error {
    // Close the open interval: set end_utc and duration_seconds.
    // Find the interval id by session_id and end_utc IS NULL and start_utc == startUTC.
    var intervalID int64
    err := db.QueryRow(`
SELECT id FROM intervals
WHERE session_id = ? AND end_utc IS NULL
ORDER BY id DESC
LIMIT 1;
`, sessionID).Scan(&intervalID)
    if err != nil {
        return fmt.Errorf("find open interval: %w", err)
    }

    durationSeconds := int64(endUTC.Sub(startUTC).Seconds())
    if durationSeconds < 0 {
        durationSeconds = 0
    }

    if _, err := db.Exec(`
UPDATE intervals
SET end_utc = ?, duration_seconds = ?
WHERE id = ?;`, endUTC.Unix(), durationSeconds, intervalID); err != nil {
        return fmt.Errorf("close interval: %w", err)
    }

    // Slice into interval_days using system local timezone at close time.
    if err := sliceIntervalIntoDays(db, intervalID, sessionID, startUTC, endUTC, category, description, time.Local); err != nil {
        return fmt.Errorf("slice interval days: %w", err)
    }

    return nil
}

// sliceIntervalIntoDays splits [startUTC, endUTC) across local date boundaries
// and inserts rows into interval_days. Durations are computed using UTC differences
// for accuracy across DST, but dates are labeled in local ('YYYY-MM-DD').
func sliceIntervalIntoDays(db *sql.DB, intervalID int64, sessionID string, startUTC, endUTC time.Time, category, description string, loc *time.Location) error {
    if !startUTC.Before(endUTC) {
        // Zero or negative duration; still record presence on start day with 0?
        // We'll skip inserting zero rows to avoid noise.
        return nil
    }

    startLocal := startUTC.In(loc)
    endLocal := endUTC.In(loc)

    // Compute the first midnight after startLocal
    // Build boundary at start of next day
    nextMidnight := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day()+1, 0, 0, 0, 0, loc)

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    curStartLocal := startLocal
    for curStartLocal.Before(endLocal) {
        segmentEndLocal := endLocal
        if nextMidnight.Before(endLocal) {
            segmentEndLocal = nextMidnight
        }

        // Convert segment bounds to UTC for accurate duration seconds
        segmentStartUTC := curStartLocal.In(time.UTC)
        segmentEndUTC := segmentEndLocal.In(time.UTC)
        segDuration := int64(segmentEndUTC.Sub(segmentStartUTC).Seconds())
        if segDuration < 0 {
            segDuration = 0
        }

        dateLocal := curStartLocal.Format("2006-01-02")

        if segDuration > 0 {
            if _, err := tx.Exec(`
INSERT INTO interval_days (interval_id, session_id, date_local, category, description, duration_seconds)
VALUES (?, ?, ?, ?, ?, ?);`,
                intervalID, sessionID, dateLocal, category, description, segDuration); err != nil {
                return fmt.Errorf("insert interval_day: %w", err)
            }
        }

        // Advance to next segment
        curStartLocal = segmentEndLocal
        nextMidnight = time.Date(curStartLocal.Year(), curStartLocal.Month(), curStartLocal.Day()+1, 0, 0, 0, 0, loc)
    }

    if err := tx.Commit(); err != nil {
        return err
    }
    return nil
}

