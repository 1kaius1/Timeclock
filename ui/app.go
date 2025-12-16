package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/yourname/timeclock/domain"
	"github.com/yourname/timeclock/reporting"
)

// RunApp launches the Fyne GUI.
func RunApp(state *domain.AppState, dbPath string) {
	a := app.NewWithID("com.example.timeclock")
	w := a.NewWindow("Timeclock")

	// --- Controls ---
	descEntry := widget.NewEntry()
	descEntry.PlaceHolder = "Description of work..."
	categoryOpts := []string{"Task", "Project", "Training", "Mentoring", "Incident", "Major Incident"}
	categorySelect := widget.NewSelect(categoryOpts, func(string) {})
	categorySelect.PlaceHolder = "Select category"

	startBtn := widget.NewButton("Start Work", func() {
		// Start from Stopped or Resume from Paused; use current desc/category snapshot
		if err := state.StartWork(strings.TrimSpace(descEntry.Text), categorySelect.Selected); err != nil {
			notifyError(w, "Start/Resume error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
	})
	pauseBtn := widget.NewButton("Pause Work", func() {
		if err := state.PauseWork(); err != nil {
			notifyError(w, "Pause error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
	})
	stopBtn := widget.NewButton("Stop Work", func() {
		if err := state.StopWork(); err != nil {
			notifyError(w, "Stop error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
	})

	// Preferences: rounding toggle
	roundToggle := widget.NewCheck("Show exact durations (seconds)", func(on bool) {
		state.RoundToNearestMinute = !on // exact seconds ON => round OFF
	})
	roundToggle.SetChecked(false) // default nearest minute (toggle OFF)

	// Status bar: state + elapsed + last action time
	stateLabel := widget.NewLabel("State: Stopped")
	elapsedLabel := widget.NewLabel("Elapsed: 00m")
	lastActionLabel := widget.NewLabel("")

	statusBar := container.NewHBox(stateLabel, widget.NewSeparator(), elapsedLabel, widget.NewSeparator(), lastActionLabel)

	// Ticker to update elapsed while InProgress
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			el := state.Elapsed()
			// Format elapsed according to rounding preference
			var txt string
			if state.RoundToNearestMinute {
				// Round to nearest minute
				mins := int((el + 30*time.Second) / time.Minute)
				txt = fmt.Sprintf("Elapsed: %dm", mins)
			} else {
				h := int(el / time.Hour)
				m := int((el % time.Hour) / time.Minute)
				s := int((el % time.Minute) / time.Second)
				if h > 0 {
					txt = fmt.Sprintf("Elapsed: %dh %dm %ds", h, m, s)
				} else {
					txt = fmt.Sprintf("Elapsed: %dm %ds", m, s)
				}
			}
			elapsedLabel.SetText(txt)

			// Update state label (in case of transitions)
			switch state.CurrentState {
			case domain.Stopped:
				stateLabel.SetText("State: Stopped")
			case domain.InProgress:
				stateLabel.SetText("State: In-Progress")
			case domain.Paused:
				stateLabel.SetText("State: Paused")
			}
		}
	}()

	// Reports view (Phase 1 basic): date range & totals per category
	fromEntry := widget.NewEntry()
	fromEntry.PlaceHolder = "From (YYYY-MM-DD)"
	toEntry := widget.NewEntry()
	toEntry.PlaceHolder = "To (YYYY-MM-DD)"
	runReportBtn := widget.NewButton("Run Report", func() {
		from := strings.TrimSpace(fromEntry.Text)
		to := strings.TrimSpace(toEntry.Text)
		if !isYYYYMMDD(from) || !isYYYYMMDD(to) {
			notifyError(w, "Invalid date", fmt.Errorf("dates must be YYYY-MM-DD"))
			return
		}
		results, err := reporting.TotalsByCategory(state.DB, from, to)
		if err != nil {
			notifyError(w, "Report error", err)
			return
		}
		var lines []string
		for _, r := range results {
			if state.RoundToNearestMinute {
				mins := int((time.Duration(r.TotalSeconds)*time.Second + 30*time.Second) / time.Minute)
				lines = append(lines, fmt.Sprintf("%-14s : %3dm", r.Category, mins))
			} else {
				d := time.Duration(r.TotalSeconds) * time.Second
				h := int(d / time.Hour)
				m := int((d % time.Hour) / time.Minute)
				s := int((d % time.Minute) / time.Second)
				if h > 0 {
					lines = append(lines, fmt.Sprintf("%-14s : %2dh %2dm %2ds", r.Category, h, m, s))
				} else {
					lines = append(lines, fmt.Sprintf("%-14s : %2dm %2ds", r.Category, m, s))
				}
			}
		}
		if len(lines) == 0 {
			lines = append(lines, "(No results)")
		}
		reportOutput.SetText(strings.Join(lines, "\n"))

		// Presence days
		days, err := reporting.PresenceDays(state.DB, from, to)
		if err != nil {
			notifyError(w, "Presence error", err)
			return
		}
		presenceOutput.SetText("Days with any work:\n" + strings.Join(days, ", "))
	})
	reportOutput := widget.NewMultiLineEntry()
	reportOutput.SetPlaceHolder("Totals per category will appear here...")
	reportOutput.Disable() // read-only feel
	presenceOutput := widget.NewMultiLineEntry()
	presenceOutput.SetPlaceHolder("Presence days will appear here...")
	presenceOutput.Disable()

	// Layout panes
	controls := container.NewVBox(
		widget.NewLabel("Work Details"),
		descEntry,
		categorySelect,
		container.NewHBox(startBtn, pauseBtn, stopBtn),
		roundToggle,
		statusBar,
	)

	reports := container.NewVBox(
		widget.NewLabel("Reports (ISO week, local dates)"),
		container.NewGridWithColumns(2,
			container.NewVBox(widget.NewLabel("From"), fromEntry),
			container.NewVBox(widget.NewLabel("To"), toEntry),
		),
		runReportBtn,
		widget.NewSeparator(),
		widget.NewLabel("Totals per category"),
		reportOutput,
		widget.NewLabel("Presence"),
		presenceOutput,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Track", controls),
		container.NewTabItem("Reports", reports),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Set initial UI state
	updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
	lastActionLabel.SetText(fmt.Sprintf("DB: %s", dbPath))

	// TODO: Keyboard shortcuts.
	// Fyne’s global shortcut API varies by version; we’ll add adaptive shortcuts
	// in Phase 3 to ensure cross-desktop reliability for X11/Wayland/macOS/Windows.

	w.SetContent(tabs)
	w.Resize(fyne.NewSize(700, 500))
	w.SetCloseIntercept(func() {
		// Optional: warn if an interval is in progress before closing.
		if state.CurrentState == domain.InProgress {
			dialog := widget.NewLabel("Work is In-Progress. Stop or Pause before closing to ensure proper logging.")
			// Very basic notice; for Phase 3 we can add confirmation dialog.
			fmt.Println(dialog.Text)
		}
		w.Close()
	})
	w.ShowAndRun()
}

func updateUIForState(state *domain.AppState, startBtn, pauseBtn, stopBtn *widget.Button, descEntry *widget.Entry, category *widget.Select) {
	switch state.CurrentState {
	case domain.Stopped:
		startBtn.Enable()
		startBtn.SetText("Start Work")
		pauseBtn.Disable()
		stopBtn.Disable()

		descEntry.Enable()
		category.Enable()
	case domain.InProgress:
		startBtn.Disable()
		startBtn.SetText("Start Work")
		pauseBtn.Enable()
		stopBtn.Enable()

		descEntry.Disable()
		category.Disable()
	case domain.Paused:
		startBtn.Enable()
		startBtn.SetText("Resume Work")
		pauseBtn.Disable() // cannot pause when already paused
		stopBtn.Enable()

		descEntry.Disable()
		category.Disable()
	}
}

func notifyError(w fyne.Window, title string, err error) {
	// Simple console + color change; Phase 3 can add dialog.
	fmt.Printf("%s: %v\n", title, err)
	w.Canvas().SetOnTypedRune(func(r rune) {}) // NOP: placeholder to keep UI responsive
}

// isYYYYMMDD validates a date string in the form YYYY-MM-DD.
func isYYYYMMDD(s string) bool {
	if len(s) != 10 {
		return false
	}
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	yr, errY := strconv.Atoi(s[0:4])
	mo, errM := strconv.Atoi(s[5:7])
	da, errD := strconv.Atoi(s[8:10])
	if errY != nil || errM != nil || errD != nil {
		return false
	}
	if yr < 1970 || yr > 9999 {
		return false
	}
	if mo < 1 || mo > 12 {
		return false
	}
	if da < 1 || da > 31 {
		return false
	}
	return true
}
