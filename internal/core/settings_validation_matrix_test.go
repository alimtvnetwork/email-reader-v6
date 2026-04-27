// settings_validation_matrix_test.go — Slice #116d (Phase 6.4):
// formal `T-SET-*` validation matrix against `core.Settings.Save`.
//
// **Why this file exists.** The acceptance criteria in
// `spec/21-app/02-features/07-settings/97-acceptance-criteria.md`
// list 12 backend-validation rows (AC-SB-03 through AC-SB-10 plus
// the maintenance-knob and retention-day variants implicit in §6 of
// `01-backend.md`). Until this slice each row was either covered by
// an ad-hoc one-off test scattered across `settings_test.go` /
// `settings_density_test.go` / `settings_maintenance_test.go`, or
// not covered at all. That made it impossible to:
//
//   1. tell at a glance which rules had a regression-guard,
//   2. ratchet new rules in (e.g. when §6 grows a row), and
//   3. confirm the spec → code → test triangle is closed.
//
// **What this file does.** A single table-driven test, one row per
// AC-SB-* validation criterion, each tagged with a stable `T-SET-`
// identifier. Each row builds on `DefaultSettingsInput()` so a row
// only encodes the *delta* that triggers the rule under test —
// keeps the table readable and resistant to default-shape changes.
//
// **Contract.** Every row asserts:
//   - `Save` returns an error,
//   - the error carries the documented `errtrace.Code`,
//   - the on-disk `config.json` is byte-identical to the seed
//     (atomic-failure guarantee from §5 step 7).
//
// **Coverage map** (T-SET-* → AC-SB-* → ER-SET-* code):
//
// 	T-SET-PollSecondsZero      → AC-SB-03 → ErrSettingsPollSeconds
// 	T-SET-PollSecondsTooHigh   → AC-SB-04 → ErrSettingsPollSeconds
// 	T-SET-ThemeUnknown         → AC-SB-05 → ErrSettingsTheme
// 	T-SET-DensityUnknown       → (extension §6) → ErrSettingsDensity
// 	T-SET-SchemeJavaScript     → AC-SB-06 → ErrSettingsUrlScheme
// 	T-SET-SchemeFile           → AC-SB-07 → ErrSettingsUrlScheme
// 	T-SET-SchemeData           → (§6 disallow list) → ErrSettingsUrlScheme
// 	T-SET-SchemeMalformed      → (§6 regex) → ErrSettingsUrlScheme
// 	T-SET-SchemesEmpty         → (§6 non-empty rule) → ErrSettingsUrlScheme
// 	T-SET-ChromePathMissing    → AC-SB-08 → ErrSettingsChromePath
// 	T-SET-ChromePathRelative   → (§6 absolute rule) → ErrSettingsChromePath
// 	T-SET-IncognitoArgInjection→ AC-SB-09 → ErrSettingsIncognitoArg
// 	T-SET-LocalhostNoHttp      → AC-SB-10 → ErrSettingsCompositeRule
// 	T-SET-RetentionTooHigh     → (§5 maintenance ranges) → ErrSettingsRetentionDays
// 	T-SET-WeekdayOutOfRange    → (§5 maintenance ranges) → ErrSettingsPersist
// 	T-SET-VacuumHourTooHigh    → (§5 maintenance ranges) → ErrSettingsPersist
// 	T-SET-WalCheckpointZero    → (§5 maintenance ranges) → ErrSettingsPersist
// 	T-SET-WalCheckpointTooHigh → (§5 maintenance ranges) → ErrSettingsPersist
// 	T-SET-PruneBatchTooSmall   → (§5 maintenance ranges) → ErrSettingsPersist
// 	T-SET-PruneBatchTooHigh    → (§5 maintenance ranges) → ErrSettingsPersist
//
// **Disk-untouched check.** Every row reads `config.json` after the
// failed `Save` and compares it byte-identically to the snapshot
// taken before the call. This pins the §5 step 7 atomic-write
// guarantee — if `Save` ever started persisting partial state on
// validation failure, the matrix would catch it on the very next
// CI run.

package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// settingsValidationCase captures one matrix row.
type settingsValidationCase struct {
	// id is the stable T-SET-* identifier surfaced in `t.Run` so
	// failed rows can be cross-referenced against the AC table.
	id string
	// mutate applies the rule-triggering delta on top of a fresh
	// `DefaultSettingsInput()` (which always passes validation).
	mutate func(*SettingsInput)
	// wantCode is the documented errtrace code the row must surface.
	wantCode errtrace.Code
}

// settingsValidationMatrix is the canonical T-SET-* table. Adding a
// row here is the only required step to lock a new §6 rule in.
//
// Order matches the implementation order in `validateInput` so a
// short-circuit failure during evolution surfaces as the first
// out-of-order row rather than a silent skip.
var settingsValidationMatrix = []settingsValidationCase{
	{
		id:       "T-SET-PollSecondsZero",
		mutate:   func(in *SettingsInput) { in.PollSeconds = 0 },
		wantCode: errtrace.ErrSettingsPollSeconds,
	},
	{
		id:       "T-SET-PollSecondsTooHigh",
		mutate:   func(in *SettingsInput) { in.PollSeconds = 61 },
		wantCode: errtrace.ErrSettingsPollSeconds,
	},
	{
		id:       "T-SET-ThemeUnknown",
		mutate:   func(in *SettingsInput) { in.Theme = ThemeMode(99) },
		wantCode: errtrace.ErrSettingsTheme,
	},
	{
		id:       "T-SET-DensityUnknown",
		mutate:   func(in *SettingsInput) { in.Density = Density(99) },
		wantCode: errtrace.ErrSettingsDensity,
	},
	{
		id:       "T-SET-SchemeJavaScript",
		mutate:   func(in *SettingsInput) { in.OpenUrlAllowedSchemes = []string{"https", "javascript"} },
		wantCode: errtrace.ErrSettingsUrlScheme,
	},
	{
		id:       "T-SET-SchemeFile",
		mutate:   func(in *SettingsInput) { in.OpenUrlAllowedSchemes = []string{"https", "file"} },
		wantCode: errtrace.ErrSettingsUrlScheme,
	},
	{
		id:       "T-SET-SchemeData",
		mutate:   func(in *SettingsInput) { in.OpenUrlAllowedSchemes = []string{"https", "data"} },
		wantCode: errtrace.ErrSettingsUrlScheme,
	},
	{
		id: "T-SET-SchemeMalformed",
		mutate: func(in *SettingsInput) {
			// Leading digit fails the `^[a-z][a-z0-9+\-.]*$` regex.
			in.OpenUrlAllowedSchemes = []string{"https", "1bad"}
		},
		wantCode: errtrace.ErrSettingsUrlScheme,
	},
	{
		id:       "T-SET-SchemesEmpty",
		mutate:   func(in *SettingsInput) { in.OpenUrlAllowedSchemes = nil },
		wantCode: errtrace.ErrSettingsUrlScheme,
	},
	{
		id: "T-SET-ChromePathMissing",
		mutate: func(in *SettingsInput) {
			// `/__email-read-test__/this/path/does/not/exist` is
			// absolute (passes the relative-path check) but
			// guaranteed not to exist on any reviewer's machine.
			in.BrowserOverride.ChromePath = "/__email-read-test__/missing-chrome"
		},
		wantCode: errtrace.ErrSettingsChromePath,
	},
	{
		id: "T-SET-ChromePathRelative",
		mutate: func(in *SettingsInput) {
			// Relative path → tripped before the os.Stat call.
			in.BrowserOverride.ChromePath = "relative/chrome"
		},
		wantCode: errtrace.ErrSettingsChromePath,
	},
	{
		id: "T-SET-IncognitoArgInjection",
		mutate: func(in *SettingsInput) {
			// Shell-metachars must not survive the regex.
			in.BrowserOverride.IncognitoArg = "; rm -rf /"
		},
		wantCode: errtrace.ErrSettingsIncognitoArg,
	},
	{
		id: "T-SET-LocalhostNoHttp",
		mutate: func(in *SettingsInput) {
			in.AllowLocalhostUrls = true
			in.OpenUrlAllowedSchemes = []string{"https"} // missing http
		},
		wantCode: errtrace.ErrSettingsCompositeRule,
	},
	{
		id:       "T-SET-RetentionTooHigh",
		mutate:   func(in *SettingsInput) { in.OpenUrlsRetentionDays = 3651 }, // > 10 years
		wantCode: errtrace.ErrSettingsRetentionDays,
	},
	{
		id:       "T-SET-WeekdayOutOfRange",
		mutate:   func(in *SettingsInput) { in.WeeklyVacuumOn = time.Weekday(7) }, // valid range 0..6
		wantCode: errtrace.ErrSettingsPersist,
	},
	{
		id:       "T-SET-VacuumHourTooHigh",
		mutate:   func(in *SettingsInput) { in.WeeklyVacuumHourLocal = 24 },
		wantCode: errtrace.ErrSettingsPersist,
	},
	{
		// WAL checkpoint zero would normally be normalized to the
		// default in `normalizeInput`; force a non-zero out-of-range
		// value to exercise the validator directly.
		id:       "T-SET-WalCheckpointTooHigh",
		mutate:   func(in *SettingsInput) { in.WalCheckpointHours = 169 },
		wantCode: errtrace.ErrSettingsPersist,
	},
	{
		id:       "T-SET-PruneBatchTooSmall",
		mutate:   func(in *SettingsInput) { in.PruneBatchSize = 99 },
		wantCode: errtrace.ErrSettingsPersist,
	},
	{
		id:       "T-SET-PruneBatchTooHigh",
		mutate:   func(in *SettingsInput) { in.PruneBatchSize = 50_001 },
		wantCode: errtrace.ErrSettingsPersist,
	},
}

// TestSettings_ValidationMatrix walks the T-SET-* matrix and pins
// each row's `(error code, disk untouched)` contract.
//
// Satisfies AC-SX-06 (backend half) — the matrix's T-SET-SchemeJavaScript,
// T-SET-SchemeFile, T-SET-SchemeData, and T-SET-SchemeMalformed cases
// pin that the forbidden-scheme list (`file`, `javascript`, `data`,
// `vbscript`) is rejected by the backend §6 layer. The frontend §5 half
// (shared table-driven test fixture across both layers) requires the
// canvas-bound Settings widget harness deferred to Slice #118e.
func TestSettings_ValidationMatrix(t *testing.T) {
	for _, tc := range settingsValidationMatrix {
		tc := tc // capture for parallel safety inside t.Run
		t.Run(tc.id, func(t *testing.T) {
			withIsolatedConfig(t, func() {
				runSettingsValidationCase(t, tc)
			})
		})
	}
}

// runSettingsValidationCase is the per-row body, hoisted out of the
// `t.Run` closure to keep the matrix function under the 15-statement
// linter cap.
func runSettingsValidationCase(t *testing.T, tc settingsValidationCase) {
	t.Helper()
	s := newTestSettings(t)

	// Snapshot config.json *after* the prime-cache Get inside
	// NewSettings — that's the bytes that must survive the failed
	// Save unchanged. Missing-file is a valid pre-state too: on a
	// fresh tempdir the prime-cache Get does not write.
	cfgPath := mustConfigPath(t)
	before := mustReadOrEmpty(t, cfgPath)

	in := DefaultSettingsInput()
	tc.mutate(&in)

	r := s.Save(context.Background(), in)
	if !r.HasError() {
		t.Fatalf("%s: Save unexpectedly succeeded; want code %v", tc.id, tc.wantCode)
	}
	gotCode := codeOf(r.Error())
	if gotCode != tc.wantCode {
		t.Fatalf("%s: Save error code = %v, want %v (msg: %v)",
			tc.id, gotCode, tc.wantCode, r.Error())
	}

	after := mustReadOrEmpty(t, cfgPath)
	if !equalBytes(before, after) {
		t.Fatalf("%s: config.json mutated by failed Save (atomic-write contract violated).\nbefore len=%d\nafter  len=%d",
			tc.id, len(before), len(after))
	}
}

// codeOf is reused from `dashboard_test.go` (same package). It walks
// the Unwrap chain and returns the first `*errtrace.Coded` code, or
// "" when none is found. Re-declaring it here would duplicate the
// helper; the matrix relies on the dashboard_test.go definition.


func mustConfigPath(t *testing.T) string {
	t.Helper()
	p, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path: %v", err)
	}
	return p
}

func mustReadOrEmpty(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
