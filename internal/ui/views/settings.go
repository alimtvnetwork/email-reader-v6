// settings.go renders the Settings form: theme dropdown, poll-interval
// entry, Chrome-path picker, and the Density toggle. The form is wired to
// core.Settings (Get / Save / ResetToDefaults) and to theme.SetDensity for
// the visual density preference.
//
// Spec: spec/21-app/02-features/07-settings/02-frontend.md (sections 1–8).
//
// Density is now persisted to core.SettingsInput.Density (Slice #42). The
// Select pre-fills from the loaded snapshot, OnChanged still applies the
// theme-level density immediately for live preview, and Save round-trips
// it through SettingsInput so the next cold start restores the choice.
//go:build !nofyne

package views

import (
	"context"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/ui/errlog"
	"github.com/lovable/email-read/internal/ui/theme"
)

// SettingsOptions wires the view to its dependencies. Leaving Service nil
// triggers a default core.NewSettings construction, matching the pattern
// used by every other view in this package.
//
// Slice #212 (Settings redesign Phase 1) added the path-panel seams:
//   - Clipboard receives the path text on Copy. Production wiring
//     (`app.go::viewFor`) injects `fyne.CurrentApp().Clipboard()`;
//     headless tests inject a recorder.
//   - OpenPath opens the **directory** in the OS file manager. nil →
//     button stays enabled but the inline status reads "not wired".
//   - RevealPath reveals (and selects) the **file** in the OS file
//     manager. Only used by the Config row (the only path that points
//     at a single file rather than a directory).
type SettingsOptions struct {
	Service *core.Settings
	// Clipboard receives clipboard writes from the path panel's Copy
	// buttons. Mirrors the same seam used by error_log.go so headless
	// tests don't need a Fyne app to verify Copy fires.
	Clipboard fyne.Clipboard
	// OpenPath, when non-nil, opens `path` in the OS default handler
	// (Finder / Explorer / xdg-open). Production wires this to
	// app.go::openLogFileWithFyne; tests substitute a recorder.
	OpenPath func(path string) error
	// RevealPath, when non-nil, reveals `path` (a single file) in the
	// OS file manager with the file selected. Only the Config row uses
	// it — Data dir and Email archive are directories, so Open is
	// sufficient. Production wires this to revealPathWithFyne; tests
	// substitute a recorder.
	RevealPath func(path string) error
}

// BuildSettings returns the Settings page widget. On any backend failure
// it falls back to a single error label so the rest of the shell stays
// usable.
func BuildSettings(opts SettingsOptions) fyne.CanvasObject {
	svc := opts.Service
	if svc == nil {
		r := core.NewSettings(time.Now)
		if r.HasError() {
			return settingsFatal(r.Error())
		}
		svc = r.Value()
	}
	snapRes := svc.Get(context.Background())
	if snapRes.HasError() {
		return settingsFatal(snapRes.Error())
	}
	return buildSettingsForm(svc, snapRes.Value(), opts)
}

// settingsFatal renders a single error pane when Settings cannot construct
// or load. Keeps the shell navigable. The trace is routed through errlog
// so the Diagnostics → Error Log view shows the file:line chain.
func settingsFatal(err error) fyne.CanvasObject {
	errlog.ReportError("settings", err)
	heading := widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := widget.NewLabel("⚠ Settings unavailable: " + err.Error() + " — see Diagnostics → Error Log")
	body.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), body)
}

// buildSettingsForm composes the main page. Inputs are grouped into three
// cards (Appearance, Watcher, Database maintenance — Slice #212). Status
// text + Save / Reset sit beneath, then the path panel card.
func buildSettingsForm(svc *core.Settings, snap core.SettingsSnapshot, opts SettingsOptions) fyne.CanvasObject {
	w := newSettingsWidgets(snap)
	status := widget.NewLabel("Loaded at " + time.Now().Format("15:04:05") + ".")
	status.Wrapping = fyne.TextWrapWord

	cards := newSettingsCards(w)
	actions := newSettingsActions(svc, w, status)
	heading := widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Configure how email-read looks, polls, and maintains its database.")
	subtitle.Wrapping = fyne.TextWrapWord
	paths := newSettingsPathsCard(snap, opts)
	body := container.NewVBox(
		heading, subtitle, widget.NewSeparator(),
		cards.appearance, cards.watcher, cards.maintenance,
		actions, status,
		paths,
	)
	return container.NewVScroll(body)
}

// settingsCards bundles the three grouped cards so buildSettingsForm
// stays under the 15-statement linter cap.
type settingsCards struct {
	appearance  *widget.Card
	watcher     *widget.Card
	maintenance *widget.Card
}

// newSettingsCards groups the 9 input widgets into three themed cards.
// The grouping is locked-in (Phase 1 user choice 2026-04-29):
//
//	Appearance  — Theme, Density
//	Watcher     — Poll interval, Chrome / Chromium path
//	Maintenance — Retention, weekday, hour, WAL hours, prune batch
func newSettingsCards(w *settingsWidgets) settingsCards {
	appearance := &widget.Form{Items: []*widget.FormItem{
		{Text: "Theme", Widget: w.themeSelect, HintText: "Restart not required — repaints live."},
		{Text: "Density", Widget: w.densitySelect, HintText: "Compact tightens paddings. Persists across restarts."},
	}}
	watcher := &widget.Form{Items: []*widget.FormItem{
		{Text: "Poll interval (seconds)", Widget: w.pollEntry, HintText: "1–60 seconds. Default 5."},
		{Text: "Chrome / Chromium path", Widget: w.chromeEntry, HintText: "Leave blank to auto-detect."},
	}}
	maintenance := &widget.Form{Items: []*widget.FormItem{
		{Text: "Opened-URLs retention (days)", Widget: w.retentionEntry, HintText: "0–3650. 0 = never prune."},
		{Text: "Weekly VACUUM weekday", Widget: w.weekdaySelect, HintText: "Day of week for the weekly VACUUM."},
		{Text: "Weekly VACUUM hour (local)", Widget: w.vacHourEntry, HintText: "0–23. 24-hour clock; default 03."},
		{Text: "WAL checkpoint cadence (hours)", Widget: w.walHoursEntry, HintText: "1–168. Default 6h."},
		{Text: "Prune batch size (rows)", Widget: w.batchEntry, HintText: "100–50000. Default 5000."},
	}}
	return settingsCards{
		appearance:  widget.NewCard("Appearance", "Theme and density. Live preview on change.", appearance),
		watcher:     widget.NewCard("Watcher", "How often we poll IMAP and which browser opens links.", watcher),
		maintenance: widget.NewCard("Database maintenance", "Retention, VACUUM cadence, and WAL housekeeping.", maintenance),
	}
}

// settingsWidgets bundles the editable inputs so action handlers can read
// them without a long parameter list.
type settingsWidgets struct {
	themeSelect    *widget.Select
	pollEntry      *widget.Entry
	chromeEntry    *widget.Entry
	densitySelect  *widget.Select
	retentionEntry *widget.Entry
	weekdaySelect  *widget.Select
	vacHourEntry   *widget.Entry
	walHoursEntry  *widget.Entry
	batchEntry     *widget.Entry
	initial        core.SettingsSnapshot
}

// newSettingsWidgets constructs the input widgets pre-populated from
// the loaded snapshot.
func newSettingsWidgets(snap core.SettingsSnapshot) *settingsWidgets {
	w := &settingsWidgets{initial: snap}
	// Live theme preview: re-apply the Fyne theme on every dropdown
	// change (mirrors how Density already live-previews on line 132).
	// Without this, picking "Light" only updates the dropdown's
	// internal state — the user has to click Save before the palette
	// flips, which contradicts the "Restart not required — repaints
	// live." hint shown next to the field. Save still persists the
	// choice (via core.Settings.Save → SettingsEvent → forwardThemeEvents
	// in app.go), so this is purely a UX-now/persist-on-save change.
	w.themeSelect = widget.NewSelect([]string{"Dark", "Light", "System"}, func(label string) {
		if mode, ok := core.ParseThemeMode(label); ok {
			if r := theme.ApplyToFyne(mode); r.HasError() {
				// Best-effort live preview — Save still rejects invalid
				// modes via the same ParseThemeMode path, so a transient
				// preview failure here is recoverable.
				_ = r
			}
		}
	})
	w.themeSelect.SetSelected(snap.Theme.String())

	w.pollEntry = widget.NewEntry()
	w.pollEntry.SetText(strconv.Itoa(int(snap.PollSeconds)))

	w.chromeEntry = widget.NewEntry()
	w.chromeEntry.SetPlaceHolder("auto-detect")
	w.chromeEntry.SetText(snap.BrowserOverride.ChromePath)

	w.densitySelect = widget.NewSelect([]string{"Comfortable", "Compact"}, func(v string) {
		theme.SetDensity(theme.Density(CoreDensityToThemeDensity(ParseDensityChoice(v))))
	})
	w.densitySelect.SetSelected(DensityLabelFor(int(snap.Density)))

	w.retentionEntry = widget.NewEntry()
	w.retentionEntry.SetText(strconv.Itoa(int(snap.OpenUrlsRetentionDays)))

	populateMaintenanceWidgets(w, snap)
	return w
}

// populateMaintenanceWidgets fills the four §5 maintenance inputs. Split
// out so newSettingsWidgets stays under the 15-statement linter cap.
func populateMaintenanceWidgets(w *settingsWidgets, snap core.SettingsSnapshot) {
	w.weekdaySelect = widget.NewSelect(WeekdayLabels(), nil)
	w.weekdaySelect.SetSelected(snap.WeeklyVacuumOn.String())

	w.vacHourEntry = widget.NewEntry()
	w.vacHourEntry.SetText(strconv.Itoa(int(snap.WeeklyVacuumHourLocal)))

	w.walHoursEntry = widget.NewEntry()
	w.walHoursEntry.SetText(strconv.Itoa(int(snap.WalCheckpointHours)))

	w.batchEntry = widget.NewEntry()
	w.batchEntry.SetText(strconv.Itoa(int(snap.PruneBatchSize)))
}

// newSettingsActions builds the Save + Reset button row and wires their
// click handlers to mutate Settings.
func newSettingsActions(svc *core.Settings, w *settingsWidgets, status *widget.Label) *fyne.Container {
	saveBtn := widget.NewButton("Save", func() {
		in, err := readSettingsInput(w)
		if err != nil {
			errlog.ReportError("settings.validate", err)
			status.SetText("⚠ " + err.Error() + " — see Diagnostics → Error Log")
			return
		}
		res := svc.Save(context.Background(), in)
		if res.HasError() {
			errlog.ReportError("settings.save", res.Error())
			status.SetText("⚠ Save failed: " + res.Error().Error() + " — see Diagnostics → Error Log")
			return
		}
		status.SetText("Saved at " + time.Now().Format("15:04:05") + ".")
	})
	saveBtn.Importance = widget.HighImportance
	resetBtn := widget.NewButton("Reset to defaults", func() {
		res := svc.ResetToDefaults(context.Background())
		if res.HasError() {
			status.SetText("⚠ Reset failed: " + res.Error().Error())
			return
		}
		repopulateWidgets(w, res.Value())
		status.SetText("Reset to defaults at " + time.Now().Format("15:04:05") + ".")
	})
	// Pad between buttons so they don't visually merge at default density.
	return container.NewHBox(saveBtn, widget.NewLabel(" "), resetBtn)
}

// newSettingsPathsCard renders the Config / Data dir / Email archive
// rows inside a `widget.Card`. Each row shows a monospace path plus
// inline action buttons (Copy, Open; + Reveal on the Config row).
// Slice #212 — replaces the old plain-label `newSettingsPaths`.
func newSettingsPathsCard(snap core.SettingsSnapshot, opts SettingsOptions) fyne.CanvasObject {
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	status.Importance = widget.LowImportance

	rows := container.NewVBox(
		newPathRow("Config path", snap.ConfigPath, true /*reveal*/, opts, status),
		widget.NewSeparator(),
		newPathRow("Data dir", snap.DataDir, false, opts, status),
		widget.NewSeparator(),
		newPathRow("Email archive", snap.EmailArchiveDir, false, opts, status),
		status,
	)
	return widget.NewCard("Filesystem locations", "Where your config, database, and email archive live.", rows)
}

// newPathRow builds a single Copy/Open(/Reveal) row for one filesystem
// path. `withReveal=true` adds the Reveal button (Config row only —
// Data dir and Email archive are directories, so Open is sufficient).
// All button handlers report into the shared `status` label so the user
// gets visible feedback without a popup.
func newPathRow(label, value string, withReveal bool, opts SettingsOptions, status *widget.Label) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	val := widget.NewLabel(value)
	val.Wrapping = fyne.TextWrapBreak
	val.TextStyle = fyne.TextStyle{Monospace: true}

	copyBtn := widget.NewButton("Copy", func() {
		status.SetText(handleCopyPath(value, opts.Clipboard, label))
	})
	openBtn := widget.NewButton("Open", func() {
		status.SetText(handleOpenPath(value, opts.OpenPath, label))
	})
	buttons := container.NewHBox(copyBtn, openBtn)
	if withReveal {
		revealBtn := widget.NewButton("Reveal", func() {
			status.SetText(handleRevealPath(value, opts.RevealPath, label))
		})
		buttons.Add(revealBtn)
	}
	return container.NewVBox(title, val, buttons)
}

// handleCopyPath / handleOpenPath / handleRevealPath are the pure-Go
// helpers that back the path-row button handlers. Pulled out as
// standalone funcs so settings_paths_test.go can verify the status
// strings + seam invocation without spinning up a Fyne app.
func handleCopyPath(value string, cb fyne.Clipboard, label string) string {
	if value == "" {
		return label + ": empty path."
	}
	if cb == nil {
		return label + ": clipboard not wired."
	}
	cb.SetContent(value)
	return "Copied " + label + " to clipboard."
}

func handleOpenPath(value string, open func(string) error, label string) string {
	if value == "" {
		return label + ": empty path."
	}
	if open == nil {
		return label + ": open handler not wired."
	}
	if err := open(value); err != nil {
		return label + ": open failed — " + err.Error()
	}
	return "Opened " + label + "."
}

func handleRevealPath(value string, reveal func(string) error, label string) string {
	if value == "" {
		return label + ": empty path."
	}
	if reveal == nil {
		return label + ": reveal handler not wired."
	}
	if err := reveal(value); err != nil {
		return label + ": reveal failed — " + err.Error()
	}
	return "Revealed " + label + " in file manager."
}

// readSettingsInput validates + projects widget state into a SettingsInput.
// Returns a friendly error for the status line when any numeric field is out
// of range or non-numeric. Delegates pure logic to the Parse* helpers and
// ProjectSettingsInput so the headless test suite can exercise both.
func readSettingsInput(w *settingsWidgets) (core.SettingsInput, error) {
	poll, err := ParsePollSeconds(w.pollEntry.Text)
	if err != nil {
		return core.SettingsInput{}, err
	}
	retention, err := ParseRetentionDays(w.retentionEntry.Text)
	if err != nil {
		return core.SettingsInput{}, err
	}
	maint, err := readMaintenanceFields(w)
	if err != nil {
		return core.SettingsInput{}, err
	}
	return ProjectSettingsInput(w.themeSelect.Selected, poll, w.chromeEntry.Text, retention, maint, w.initial, w.densitySelect.Selected), nil
}

// readMaintenanceFields parses the four §5 inputs as a unit. Pulled out
// so readSettingsInput stays under the 15-statement cap.
func readMaintenanceFields(w *settingsWidgets) (MaintenanceFields, error) {
	hour, err := ParseVacuumHourLocal(w.vacHourEntry.Text)
	if err != nil {
		return MaintenanceFields{}, err
	}
	wal, err := ParseWalCheckpointHours(w.walHoursEntry.Text)
	if err != nil {
		return MaintenanceFields{}, err
	}
	batch, err := ParsePruneBatchSize(w.batchEntry.Text)
	if err != nil {
		return MaintenanceFields{}, err
	}
	return MaintenanceFields{
		WeekdayLabel:   w.weekdaySelect.Selected,
		HourLocal:      hour,
		WalHours:       wal,
		PruneBatchSize: batch,
	}, nil
}

// repopulateWidgets refreshes the form after a Reset so the user sees the
// applied defaults instead of their previous (now-discarded) edits.
func repopulateWidgets(w *settingsWidgets, snap core.SettingsSnapshot) {
	w.initial = snap
	w.themeSelect.SetSelected(snap.Theme.String())
	w.pollEntry.SetText(strconv.Itoa(int(snap.PollSeconds)))
	w.chromeEntry.SetText(snap.BrowserOverride.ChromePath)
	w.densitySelect.SetSelected(DensityLabelFor(int(snap.Density)))
	w.retentionEntry.SetText(strconv.Itoa(int(snap.OpenUrlsRetentionDays)))
	w.weekdaySelect.SetSelected(snap.WeeklyVacuumOn.String())
	w.vacHourEntry.SetText(strconv.Itoa(int(snap.WeeklyVacuumHourLocal)))
	w.walHoursEntry.SetText(strconv.Itoa(int(snap.WalCheckpointHours)))
	w.batchEntry.SetText(strconv.Itoa(int(snap.PruneBatchSize)))
}
