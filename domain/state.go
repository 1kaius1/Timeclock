
package domain

import (
    "database/sql"
    "errors"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/yourname/timeclock/storage"
)

// AppState defines the in-memory state machine for Timeclock.
// States: Stopped | InProgress | Paused
type State int

const (
    Stopped State = iota
    InProgress
    Paused
)

var (
    ErrInvalidTransition = errors.New("invalid transition for current state")
    ErrNoOpenInterval    = errors.New("no open interval to close")
)

// AppState holds current UI/business state.
type AppState struct {
    mu sync.Mutex

    DB *sql.DB

    CurrentState State
    SessionID    string // UUID for current session
    // Snapshot set at session start (and carried through until STOP):
    Category    string // locked in InProgress/Paused
    Description string // locked in InProgress/Paused

    // Interval info:
    IntervalIndex int       // 0..n within the session
    IntervalStart time.Time // UTC time when current interval started

    // Preferences:
    RoundToNearestMinute bool // default true; UI toggle can change this
}

// NewAppState constructs an initial state (Stopped).
func NewAppState(db *sql.DB) *AppState {
    return &AppState{
        DB:                   db,
        CurrentState:         Stopped,
        RoundToNearestMinute: true,
    }
}

// StartWork starts a new session (from Stopped) or resumes (from Paused).
// When starting from Stopped: new session_id, index=0, open interval.
// When resuming from Paused: same session_id, index++, open interval.
func (s *AppState) StartWork(description, category string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    nowUTC := time.Now().UTC()

    switch s.CurrentState {
    case Stopped:
        // Validate required category
        if category == "" {
            return errors.New("category is required")
        }

        s.SessionID = uuid.NewString()
        s.IntervalIndex = 0
        s.Description = description
        s.Category = category
        s.IntervalStart = nowUTC
        s.CurrentState = InProgress

        // Log START event and open interval
        if err := storage.InsertEvent(s.DB, s.SessionID, nowUTC, "START", s.Category, s.Description); err != nil {
            return err
        }
        if err := storage.OpenInterval(s.DB, s.SessionID, s.IntervalIndex, s.IntervalStart, s.Category, s.Description); err != nil {
            return err
        }
        return nil

    case Paused:
        // Resume work: same session_id/category/description, index++
        s.IntervalIndex++
        s.IntervalStart = nowUTC
        s.CurrentState = InProgress

        if err := storage.InsertEvent(s.DB, s.SessionID, nowUTC, "RESUME", s.Category, s.Description); err != nil {
            return err
        }
        if err := storage.OpenInterval(s.DB, s.SessionID, s.IntervalIndex, s.IntervalStart, s.Category, s.Description); err != nil {
            return err
        }
        return nil

    case InProgress:
        return ErrInvalidTransition
    default:
        return ErrInvalidTransition
    }
}

// PauseWork pauses an in-progress session: closes the current interval and stays in the same session.
// Description/Category remain locked; Start becomes "Resume".
func (s *AppState) PauseWork() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.CurrentState != InProgress {
        return ErrInvalidTransition
    }

    nowUTC := time.Now().UTC()

    // Close current interval and write PAUSE event
    if err := storage.CloseOpenIntervalAndSliceDays(s.DB, s.SessionID, s.IntervalStart, nowUTC, s.Category, s.Description); err != nil {
        return err
    }
    if err := storage.InsertEvent(s.DB, s.SessionID, nowUTC, "PAUSE", s.Category, s.Description); err != nil {
        return err
    }

    s.CurrentState = Paused
    return nil
}

// StopWork finalizes the session: closes interval if open and logs STOP.
func (s *AppState) StopWork() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.CurrentState == Stopped {
        return ErrInvalidTransition
    }

    nowUTC := time.Now().UTC()

    // If we were InProgress, close the interval.
    if s.CurrentState == InProgress {
        if err := storage.CloseOpenIntervalAndSliceDays(s.DB, s.SessionID, s.IntervalStart, nowUTC, s.Category, s.Description); err != nil {
            return err
        }
    }

    // Write STOP event
    if err := storage.InsertEvent(s.DB, s.SessionID, nowUTC, "STOP", s.Category, s.Description); err != nil {
        return err
    }

    // Reset session data
    s.CurrentState = Stopped
    s.SessionID = ""
    s.IntervalIndex = 0
    s.IntervalStart = time.Time{}
    // Description & Category become editable again in UI (but we leave last values visible)
    return nil
}

// Elapsed returns the current interval elapsed (if InProgress).
func (s *AppState) Elapsed() time.Duration {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.CurrentState != InProgress || s.IntervalStart.IsZero() {
        return 0
    }
    return time.Since(s.IntervalStart)
}


