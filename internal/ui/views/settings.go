// settings.go renders the Settings form: theme dropdown, poll-interval
// entry, Chrome-path picker, and the Density toggle. The form is wired to
// core.Settings (Get / Save / ResetToDefaults) and to theme.SetDensity for
// the visual density preference.
//
// Spec: spec/21-app/02-features/07-settings/02-frontend.md (sections 1–8).
//
// Density is intentionally kept process-local (not persisted in
// SettingsInput) — the spec marks the persisted-density story as deferred
// (§8). When persistence lands, swap the OnChanged handler for a Settings
// field write — no other callers depend on the local in-memory toggle.
//go:build !nofyne

package views

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
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
// or load. Keeps the shell navigable.
func settingsFatal(err error) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := widget.NewLabel("⚠ Settings unavailable: " + err.Error())
	body.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), body)
}

// buildSettingsForm composes the main form. Inputs are laid out via
// widget.Form; status text + Save / Reset buttons sit beneath.
func buildSettingsForm(svc *core.Settings, snap core.SettingsSnapshot) fyne.CanvasObject {
	w := newSettingsWidgets(snap)
	status := widget.NewLabel("Loaded at " + time.Now().Format("15:04:05") + ".")
	status.Wrapping = fyne.TextWrapWord

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Theme", Widget: w.themeSelect, HintText: "Restart not required — repaints live."},
			{Text: "Poll interval (seconds)", Widget: w.pollEntry, HintText: "1–60. Applied to running watcher live."},
			{Text: "Chrome / Chromium path", Widget: w.chromeEntry, HintText: "Leave blank to auto-detect."},
			{Text: "Density", Widget: w.densitySelect, HintText: "Compact tightens paddings (process-local)."},
		},
	}
	actions := newSettingsActions(svc, w, status)
	heading := widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	paths := newSettingsPaths(snap)
	return container.NewVBox(
		heading, widget.NewSeparator(),
		form, actions, status,
		widget.NewSeparator(), paths,
	)
}

// settingsWidgets bundles the editable inputs so action handlers can read
// them without a long parameter list.
type settingsWidgets struct {
	themeSelect   *widget.Select
	pollEntry     *widget.Entry
	chromeEntry   *widget.Entry
	densitySelect *widget.Select
	initial       core.SettingsSnapshot
}

// newSettingsWidgets constructs the four input widgets pre-populated from
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
		applyDensity(v)
	})
	w.densitySelect.SetSelected(densityLabel(theme.ActiveDensity()))
	return w
}

// newSettingsActions builds the Save + Reset button row and wires their
// click handlers to mutate Settings.
func newSettingsActions(svc *core.Settings, w *settingsWidgets, status *widget.Label) *fyne.Container {
	saveBtn := widget.NewButton("Save", func() {
		in, err := readSettingsInput(w)
		if err != nil {
			status.SetText("⚠ " + err.Error())
			return
		}
		res := svc.Save(context.Background(), in)
		if res.HasError() {
			status.SetText("⚠ Save failed: " + res.Error().Error())
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
// Returns a friendly error for the status line when poll seconds are out
// of range or non-numeric.
func readSettingsInput(w *settingsWidgets) (core.SettingsInput, error) {
	poll, err := strconv.Atoi(w.pollEntry.Text)
	if err != nil || poll < 1 || poll > 60 {
		return core.SettingsInput{}, fmt.Errorf("poll interval must be 1–60 seconds")
	}
	mode, _ := core.ParseThemeMode(w.themeSelect.Selected)
	return core.SettingsInput{
		PollSeconds: uint16(poll),
		Theme:       mode,
		BrowserOverride: core.BrowserOverride{
			ChromePath:   w.chromeEntry.Text,
			IncognitoArg: w.initial.BrowserOverride.IncognitoArg,
		},
		OpenUrlAllowedSchemes: w.initial.OpenUrlAllowedSchemes,
		AllowLocalhostUrls:    w.initial.AllowLocalhostUrls,
		AutoStartWatch:        w.initial.AutoStartWatch,
	}, nil
}

// repopulateWidgets refreshes the form after a Reset so the user sees the
// applied defaults instead of their previous (now-discarded) edits.
func repopulateWidgets(w *settingsWidgets, snap core.SettingsSnapshot) {
	w.initial = snap
	w.themeSelect.SetSelected(snap.Theme.String())
	w.pollEntry.SetText(strconv.Itoa(int(snap.PollSeconds)))
	w.chromeEntry.SetText(snap.BrowserOverride.ChromePath)
}

// applyDensity translates the Select label into a theme.Density and pushes
// it through theme.SetDensity. Unknown labels are no-ops.
func applyDensity(label string) {
	switch label {
	case "Compact":
		theme.SetDensity(theme.DensityCompact)
	case "Comfortable":
		theme.SetDensity(theme.DensityComfortable)
	}
}

// densityLabel renders a theme.Density as the matching Select label.
func densityLabel(d theme.Density) string {
	if d == theme.DensityCompact {
		return "Compact"
	}
	return "Comfortable"
}
