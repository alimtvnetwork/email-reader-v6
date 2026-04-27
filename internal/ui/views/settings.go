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
type SettingsOptions struct {
	Service *core.Settings
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
	return buildSettingsForm(svc, snapRes.Value())
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

// buildSettingsForm composes the main form. Inputs are laid out via
// widget.Form; status text + Save / Reset buttons sit beneath.
func buildSettingsForm(svc *core.Settings, snap core.SettingsSnapshot) fyne.CanvasObject {
	w := newSettingsWidgets(snap)
	status := widget.NewLabel("Loaded at " + time.Now().Format("15:04:05") + ".")
	status.Wrapping = fyne.TextWrapWord

	form := &widget.Form{Items: settingsFormItems(w)}
	actions := newSettingsActions(svc, w, status)
	heading := widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	paths := newSettingsPaths(snap)
	return container.NewVBox(
		heading, widget.NewSeparator(),
		form, actions, status,
		widget.NewSeparator(), paths,
	)
}

// settingsFormItems composes the labelled input rows. Split out of
// buildSettingsForm to keep it under the 15-statement linter cap once the
// maintenance knobs grew the row count.
func settingsFormItems(w *settingsWidgets) []*widget.FormItem {
	return []*widget.FormItem{
		{Text: "Theme", Widget: w.themeSelect, HintText: "Restart not required — repaints live."},
		{Text: "Poll interval (seconds)", Widget: w.pollEntry, HintText: "1–60. Applied to running watcher live."},
		{Text: "Chrome / Chromium path", Widget: w.chromeEntry, HintText: "Leave blank to auto-detect."},
		{Text: "Density", Widget: w.densitySelect, HintText: "Compact tightens paddings. Persists across restarts."},
		{Text: "Opened-URLs retention (days)", Widget: w.retentionEntry, HintText: "0–3650. 0 = never prune."},
		{Text: "Weekly VACUUM weekday", Widget: w.weekdaySelect, HintText: "Day of week for the weekly VACUUM."},
		{Text: "Weekly VACUUM hour (local)", Widget: w.vacHourEntry, HintText: "0–23. 24-hour clock; default 03."},
		{Text: "WAL checkpoint cadence (hours)", Widget: w.walHoursEntry, HintText: "1–168. Default 6h."},
		{Text: "Prune batch size (rows)", Widget: w.batchEntry, HintText: "100–50000. Default 5000."},
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
	w.themeSelect = widget.NewSelect([]string{"Dark", "Light", "System"}, nil)
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
	return container.NewHBox(saveBtn, resetBtn)
}

// newSettingsPaths returns a small read-only paths panel so users can see
// where their config and data live without leaving the screen.
func newSettingsPaths(snap core.SettingsSnapshot) fyne.CanvasObject {
	mk := func(label, value string) fyne.CanvasObject {
		k := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		v := widget.NewLabel(value)
		v.Wrapping = fyne.TextWrapWord
		return container.NewVBox(k, v)
	}
	return container.NewVBox(
		mk("Config path", snap.ConfigPath),
		mk("Data dir", snap.DataDir),
		mk("Email archive", snap.EmailArchiveDir),
	)
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
