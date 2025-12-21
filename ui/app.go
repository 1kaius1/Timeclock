package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"github.com/1kaius1/Timeclock/domain"
	"github.com/1kaius1/Timeclock/reporting"
	"github.com/1kaius1/Timeclock/storage"
)

// RunApp launches the Fyne GUI.
func RunApp(state *domain.AppState, dbPath string, scale float32, appVersion string, scaleForced bool) {
	a := app.NewWithID("com.example.timeclock")
	w := a.NewWindow("Timeclock")

	// Load settings from database
	exactDurationsStr := storage.GetSetting(state.DB, "exact_durations", "false")
	state.RoundToNearestMinute = (exactDurationsStr != "true")

	savedScaleStr := storage.GetSetting(state.DB, "scale", "1.0")
	savedScale, _ := strconv.ParseFloat(savedScaleStr, 32)
	if savedScale < 0.5 || savedScale > 3.0 {
		savedScale = 1.0
	}

	// --- Controls (declare first) ---
	descEntry := widget.NewEntry()
	descEntry.PlaceHolder = "Description of work..."
	
	// If state was restored, populate the description field
	if state.CurrentState != domain.Stopped {
		descEntry.SetText(state.Description)
	}

	categoryOpts := []string{"Task", "Project", "Training", "Mentoring", "Incident", "Major Incident"}
	categorySelect := widget.NewSelect(categoryOpts, func(string) {})
	categorySelect.PlaceHolder = "Select category"
	
	// If state was restored, select the category
	if state.CurrentState != domain.Stopped {
		categorySelect.SetSelected(state.Category)
	}

	// Declare buttons up-front so closures can capture them
	var startBtn *widget.Button
	var pauseBtn *widget.Button
	var stopBtn *widget.Button

	// Bindings for labels (idiomatic Fyne)
	stateBind := binding.NewString()
	_ = stateBind.Set("State: Stopped")
	stateLabel := widget.NewLabelWithData(stateBind)

	elapsedBind := binding.NewString()
	_ = elapsedBind.Set("Elapsed: 00m")
	elapsedLabel := widget.NewLabelWithData(elapsedBind)

	// Recent events list - shows last 5 state changes
	recentEventsList := widget.NewList(
		func() int { return 0 }, // will be updated dynamically
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			// will be updated dynamically
		},
	)

	// Function to refresh recent events from database
	refreshRecentEvents := func() {
		rows, err := state.DB.Query(`
SELECT timestamp_utc, action, category, description
FROM events
ORDER BY id DESC
LIMIT 5;
`)
		if err != nil {
			return
		}
		defer rows.Close()

		var events []string
		for rows.Next() {
			var timestampUTC int64
			var action, category, description string
			if err := rows.Scan(&timestampUTC, &action, &category, &description); err != nil {
				continue
			}
			t := time.Unix(timestampUTC, 0).Local()
			timeStr := t.Format("2006-01-02 15:04:05")
			desc := description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			events = append(events, fmt.Sprintf("%s  %s  %s  %s", timeStr, action, category, desc))
		}

		// Update list
		recentEventsList.Length = func() int { return len(events) }
		recentEventsList.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(events) {
				obj.(*widget.Label).SetText(events[id])
			}
		}
		recentEventsList.Refresh()
	}

	// Reports widgets
	fromEntry := widget.NewEntry()
	fromEntry.PlaceHolder = "From (YYYY-MM-DD)"
	toEntry := widget.NewEntry()
	toEntry.PlaceHolder = "To (YYYY-MM-DD)"
	var runReportBtn *widget.Button

	// Use Labels instead of MultiLineEntry for output
	reportOutput := widget.NewLabel("Totals per category will appear here...")
	reportOutput.Wrapping = fyne.TextWrapWord

	presenceOutput := widget.NewLabel("Presence days will appear here...")
	presenceOutput.Wrapping = fyne.TextWrapWord

	// Wrap in scroll containers so long reports are scrollable
	reportScroll := container.NewScroll(reportOutput)
	reportScroll.SetMinSize(fyne.NewSize(400, 150))

	presenceScroll := container.NewScroll(presenceOutput)
	presenceScroll.SetMinSize(fyne.NewSize(400, 80))

	// --- Settings Tab Widgets ---
	
	// Exact durations checkbox
	exactDurationsCheck := widget.NewCheck("Show exact durations (seconds)", func(checked bool) {
		state.RoundToNearestMinute = !checked
		if err := storage.SetSetting(state.DB, "exact_durations", fmt.Sprintf("%t", checked)); err != nil {
			notifyError(w, "Failed to save setting", err)
		}
	})
	exactDurationsCheck.SetChecked(exactDurationsStr == "true")

	// Scale slider and entry
	scaleValueLabel := widget.NewLabel(fmt.Sprintf("%.2f", savedScale))
	scaleEntry := widget.NewEntry()
	scaleEntry.SetText(fmt.Sprintf("%.2f", savedScale))

	scaleSlider := widget.NewSlider(0.5, 3.0)
	scaleSlider.SetValue(savedScale)
	scaleSlider.Step = 0.05

	// Scale slider callback
	scaleSlider.OnChanged = func(value float64) {
		scaleValueLabel.SetText(fmt.Sprintf("%.2f", value))
		scaleEntry.SetText(fmt.Sprintf("%.2f", value))
	}

	// Scale entry callback
	scaleEntry.OnChanged = func(text string) {
		if val, err := strconv.ParseFloat(text, 64); err == nil {
			if val >= 0.5 && val <= 3.0 {
				scaleSlider.SetValue(val)
				scaleValueLabel.SetText(fmt.Sprintf("%.2f", val))
			}
		}
	}

	// Save scale button with message label
	saveScaleMessage := widget.NewLabel("")
	saveScaleBtn := widget.NewButton("Save Scale", func() {
		val, err := strconv.ParseFloat(scaleEntry.Text, 64)
		if err != nil || val < 0.5 || val > 3.0 {
			notifyError(w, "Invalid scale", fmt.Errorf("scale must be between 0.5 and 3.0"))
			return
		}
		if err := storage.SetSetting(state.DB, "scale", fmt.Sprintf("%.2f", val)); err != nil {
			notifyError(w, "Failed to save scale", err)
			return
		}
		saveScaleMessage.SetText("Scale saved. Restart the application for changes to take effect.")
		time.AfterFunc(5*time.Second, func() {
			saveScaleMessage.SetText("")
		})
	})

	// Scale status information
	var scaleStatusText string
	if scaleForced {
		scaleStatusText = fmt.Sprintf("Current scale: %.2f (forced by -scale flag)\nSaved scale: %.2f (will be used when flag is not provided)", scale, savedScale)
	} else {
		scaleStatusText = fmt.Sprintf("Current scale: %.2f (from database)\nNo -scale flag provided", scale)
	}
	scaleStatus := widget.NewLabel(scaleStatusText)
	scaleStatus.Wrapping = fyne.TextWrapWord

	// Database path (read-only)
	dbPathLabel := widget.NewLabel(fmt.Sprintf("Database: %s", dbPath))
	dbPathLabel.Wrapping = fyne.TextWrapWord

	// --- Wire up handlers AFTER widgets exist ---

	startBtn = widget.NewButton("Start Work", func() {
		if err := state.StartWork(strings.TrimSpace(descEntry.Text), categorySelect.Selected); err != nil {
			notifyError(w, "Start/Resume error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
		refreshRecentEvents()
		// Optional immediate state label update (not required; ticker will update in <1s)
		switch state.CurrentState {
		case domain.Stopped:
			_ = stateBind.Set("State: Stopped")
		case domain.InProgress:
			_ = stateBind.Set("State: In-Progress")
		case domain.Paused:
			_ = stateBind.Set("State: Paused")
		}
	})

	pauseBtn = widget.NewButton("Pause Work", func() {
		if err := state.PauseWork(); err != nil {
			notifyError(w, "Pause error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
		refreshRecentEvents()
		switch state.CurrentState {
		case domain.Stopped:
			_ = stateBind.Set("State: Stopped")
		case domain.InProgress:
			_ = stateBind.Set("State: In-Progress")
		case domain.Paused:
			_ = stateBind.Set("State: Paused")
		}
	})

	stopBtn = widget.NewButton("Stop Work", func() {
		if err := state.StopWork(); err != nil {
			notifyError(w, "Stop error", err)
			return
		}
		updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
		refreshRecentEvents()
		switch state.CurrentState {
		case domain.Stopped:
			_ = stateBind.Set("State: Stopped")
		case domain.InProgress:
			_ = stateBind.Set("State: In-Progress")
		case domain.Paused:
			_ = stateBind.Set("State: Paused")
		}
	})

	// Ticker to update elapsed while InProgress (binding handles UI thread safely)
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
			_ = elapsedBind.Set(txt)

			// Reflect current state label
			switch state.CurrentState {
			case domain.Stopped:
				_ = stateBind.Set("State: Stopped")
			case domain.InProgress:
				_ = stateBind.Set("State: In-Progress")
			case domain.Paused:
				_ = stateBind.Set("State: Paused")
			}
		}
	}()

	// Reports: run button handler
	runReportBtn = widget.NewButton("Run Report", func() {
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
		if len(days) == 0 {
			presenceOutput.SetText("Days with any work:\n(none)")
		} else {
			presenceOutput.SetText("Days with any work:\n" + strings.Join(days, ", "))
		}
	})

	// Layout panes - Track tab with recent events
	controlsTop := container.NewVBox(
		widget.NewLabel("Work Details"),
		descEntry,
		categorySelect,
		container.NewHBox(startBtn, pauseBtn, stopBtn),
		container.NewHBox(stateLabel, widget.NewSeparator(), elapsedLabel),
	)

	recentEventsSection := container.NewBorder(
		widget.NewLabel("Recent Activity"),
		nil, nil, nil,
		recentEventsList,
	)

	controls := container.NewBorder(
		controlsTop,
		nil, nil, nil,
		recentEventsSection,
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
		reportScroll,
		widget.NewLabel("Presence"),
		presenceScroll,
	)

	// Settings tab layout
	settings := container.NewVBox(
		widget.NewLabel("Settings"),
		widget.NewSeparator(),
		
		widget.NewLabel("Display Options"),
		exactDurationsCheck,
		
		widget.NewSeparator(),
		widget.NewLabel("UI Scale (0.5 - 3.0)"),
		scaleStatus,
		container.NewBorder(nil, nil, widget.NewLabel("Scale:"), scaleValueLabel, scaleSlider),
		container.NewBorder(nil, nil, widget.NewLabel("Value:"), nil, scaleEntry),
		saveScaleBtn,
		saveScaleMessage,
		
		widget.NewSeparator(),
		widget.NewLabel("Database Location"),
		dbPathLabel,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Track", controls),
		container.NewTabItem("Reports", reports),
		container.NewTabItem("Settings", settings),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Status line at bottom
	statusLine := container.NewBorder(
		nil, nil,
		widget.NewLabel(fmt.Sprintf("DB: %s", dbPath)),
		widget.NewLabel(fmt.Sprintf("v%s", appVersion)),
		widget.NewLabel(fmt.Sprintf("Scale: %d%%", int(scale*100))),
	)

	// Main content with status line at bottom
	mainContent := container.NewBorder(
		nil,
		statusLine,
		nil, nil,
		tabs,
	)

	// Initial UI state
	updateUIForState(state, startBtn, pauseBtn, stopBtn, descEntry, categorySelect)
	refreshRecentEvents()

	w.SetContent(mainContent)
	w.Resize(fyne.NewSize(700, 500))
	// Optional: this code is run before the window closes.
	w.SetCloseIntercept(func() {
		// This example code checks the state of work, and if work is in-progress it prints a warning.
		if state.CurrentState == domain.InProgress {
			fmt.Println("!! WARNING - Work is In-Progress and being tracked even if Timeclock is not running.")
		}
		
		// Actually close the window
		w.Close()
	})

	w.ShowAndRun()
}

// updateUIForState keeps its original signature (no bindings here)
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
	// Minimal notify; Phase 3 can add dialog boxes.
	fmt.Printf("%s: %v\n", title, err)
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

