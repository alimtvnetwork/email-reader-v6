// settings.go is the Settings service: loads/persists the four owned
// objects in config.json (Watch, Browser, Ui, plus the new
// OpenUrlAllowedSchemes / AllowLocalhostUrls extension fields) and fans out
// SettingsEvent on Subscribe channels for live-reload consumers.
//
// Spec: spec/21-app/02-features/07-settings/01-backend.md
//
// Scope notes vs spec:
//   - The on-disk schema keeps its legacy camelCase keys for backward
//     compatibility with existing user config.json files. PascalCase JSON
//     migration is tracked under Delta #1.
//   - OpenUrlAllowedSchemes / AllowLocalhostUrls / AutoStartWatch / Theme
//     are stored under new optional keys: when missing from disk, the
//     defaults from DefaultSettingsInput() are returned by Get.
//   - DetectChrome reuses the probe order documented in §7 and surfaces a
//     ChromeDetection value but does not consult the env var prefix
//     MAILPULSE_* — this codebase ships as `email-read`, so we honour
//     EMAIL_READ_CHROME (already used by internal/browser).
package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// settingsExtension holds the new fields we layer on top of the existing
// camelCase config.json. They live under a dedicated "settings" object so
// older config files (which lack the key) still parse cleanly.
type settingsExtension struct {
	Theme                 string   `json:"theme"`
	Density               string   `json:"density"`
	OpenUrlAllowedSchemes []string `json:"openUrlAllowedSchemes"`
	AllowLocalhostUrls    bool     `json:"allowLocalhostUrls"`
	AutoStartWatch        bool     `json:"autoStartWatch"`
	OpenUrlsRetentionDays uint16   `json:"openUrlsRetentionDays"`
	WeeklyVacuumOn        string   `json:"weeklyVacuumOn"`        // "Sunday".."Saturday"
	WeeklyVacuumHourLocal uint8    `json:"weeklyVacuumHourLocal"` // 0..23
	WalCheckpointHours    uint8    `json:"walCheckpointHours"`    // 1..168
	PruneBatchSize        uint32   `json:"pruneBatchSize"`        // 100..50000
	UpdatedAt             string   `json:"updatedAt"`             // RFC3339; "" when never written
}

// rawConfigWithSettings extends the on-disk JSON with the optional settings
// block. We marshal/unmarshal the raw bytes ourselves so we can preserve
// unknown top-level keys byte-for-byte (TODO: full preservation is part of
// the M004 migration; today we round-trip via config.Config which already
// covers Accounts, Rules, Watch, Browser).
type rawConfigWithSettings struct {
	cfg *config.Config
	ext settingsExtension
}

// Settings is the service wrapper. Construct via NewSettings.
type Settings struct {
	clock func() time.Time

	// saveMu serializes Save / ResetToDefaults so two writers never
	// interleave the load → mutate → write window.
	saveMu sync.Mutex

	// mu guards lastApplied + subscribers.
	mu          sync.RWMutex
	lastApplied SettingsSnapshot
	subscribers []chan SettingsEvent
}

// NewSettings builds a Settings service. The clock is injectable so tests
// can pin time.Now(); pass nil to use time.Now.
func NewSettings(clock func() time.Time) errtrace.Result[*Settings] {
	if clock == nil {
		clock = time.Now
	}
	s := &Settings{clock: clock}
	// Prime the cache so Get is zero-IO after construction.
	if r := s.Get(context.Background()); r.HasError() {
		return errtrace.Err[*Settings](errtrace.WrapCode(r.Error(),
			errtrace.ErrSettingsConstruct, "prime settings cache"))
	}
	return errtrace.Ok(s)
}

// Get returns the current persisted snapshot. Read-only path fields are
// freshly resolved on every call so display values track the running
// executable's location.
func (s *Settings) Get(ctx context.Context) errtrace.Result[SettingsSnapshot] {
	raw, err := loadRaw()
	if err != nil {
		return errtrace.Err[SettingsSnapshot](err)
	}
	snap, err := snapshotFromRaw(raw)
	if err != nil {
		return errtrace.Err[SettingsSnapshot](err)
	}
	s.mu.Lock()
	s.lastApplied = snap
	s.mu.Unlock()
	return errtrace.Ok(snap)
}

// Save validates, persists, caches, and publishes a Settings event.
func (s *Settings) Save(ctx context.Context, in SettingsInput) errtrace.Result[SettingsSnapshot] {
	return s.persist(ctx, in, SettingsSaved)
}

// ResetToDefaults overwrites Theme, BrowserOverride, PollSeconds,
// OpenUrlAllowedSchemes, AllowLocalhostUrls, AutoStartWatch with the spec
// defaults. Accounts and Rules are untouched.
func (s *Settings) ResetToDefaults(ctx context.Context) errtrace.Result[SettingsSnapshot] {
	return s.persist(ctx, DefaultSettingsInput(), SettingsResetApplied)
}

// persist runs steps 1-7 of the §5 Save pipeline.
func (s *Settings) persist(ctx context.Context, in SettingsInput, kind SettingsEventKind) errtrace.Result[SettingsSnapshot] {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	in = normalizeInput(in)
	if err := validateInput(in); err != nil {
		return errtrace.Err[SettingsSnapshot](err)
	}
	snap, err := s.persistLocked(in)
	if err != nil {
		return errtrace.Err[SettingsSnapshot](err)
	}
	s.mu.Lock()
	s.lastApplied = snap
	s.mu.Unlock()
	s.publish(SettingsEvent{Kind: kind, Snapshot: snap, At: s.clock()})
	return errtrace.Ok(snap)
}

// persistLocked runs the on-disk Load+apply+Save under the
// process-wide config write lock so it cannot interleave with
// AddAccount/RemoveAccount (CF-A2). Split out so persist stays
// under the 15-statement linter limit (AC-PROJ-20).
func (s *Settings) persistLocked(in SettingsInput) (SettingsSnapshot, error) {
	var (
		snap SettingsSnapshot
		err  error
	)
	config.WithWriteLock(func() {
		var raw *rawConfigWithSettings
		raw, err = loadRaw()
		if err != nil {
			return
		}
		applyInputToRaw(raw, in, s.clock())
		if sErr := saveRaw(raw); sErr != nil {
			err = errtrace.WrapCode(sErr, errtrace.ErrSettingsPersist, "persist settings")
			return
		}
		snap, err = snapshotFromRaw(raw)
	})
	return snap, err
}

// Subscribe returns a buffered channel (cap 4) and a cancel func. Per spec
// §9 the cancel func unsubscribes silently — events emitted afterwards are
// dropped.
func (s *Settings) Subscribe(ctx context.Context) (<-chan SettingsEvent, func()) {
	ch := make(chan SettingsEvent, 4)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	cancel := func() { s.unsubscribe(ch) }
	if ctx != nil {
		go func() {
			<-ctx.Done()
			cancel()
		}()
	}
	return ch, cancel
}

// unsubscribe removes ch from the subscriber list and closes it. Safe to
// call multiple times.
func (s *Settings) unsubscribe(ch chan SettingsEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// publish does a non-blocking send to every subscriber. A full buffer is
// dropped (with a sentinel error code logged at the call site if a logger
// is wired in later). Never fails Save.
func (s *Settings) publish(ev SettingsEvent) {
	s.mu.RLock()
	subs := append([]chan SettingsEvent(nil), s.subscribers...)
	s.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// Buffer full → drop. ErrSettingsEventDropped is reserved for
			// the upcoming structured logger.
		}
	}
}

// DetectChrome runs the probe order from §7 (config → env → OS defaults →
// PATH → not found). Pure aside from os.Stat / exec.LookPath.
func (s *Settings) DetectChrome(ctx context.Context) errtrace.Result[ChromeDetection] {
	// Step 1: current snapshot's config override.
	snap := s.cachedSnapshot()
	if p := strings.TrimSpace(snap.BrowserOverride.ChromePath); p != "" {
		if fileExistsExec(p) {
			return errtrace.Ok(ChromeDetection{Path: p, Source: ChromeFromConfig})
		}
	}
	// Step 2: env var.
	if p := strings.TrimSpace(os.Getenv("EMAIL_READ_CHROME")); p != "" {
		if fileExistsExec(p) {
			return errtrace.Ok(ChromeDetection{Path: p, Source: ChromeFromEnv})
		}
	}
	// Step 3: OS-default well-known locations.
	for _, p := range osDefaultChromePaths() {
		if fileExistsExec(p) {
			return errtrace.Ok(ChromeDetection{Path: p, Source: ChromeFromOsDefault})
		}
	}
	// Step 4: PATH lookup, in fallback order.
	for _, name := range []string{"google-chrome", "chromium", "microsoft-edge", "brave-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return errtrace.Ok(ChromeDetection{Path: p, Source: ChromeFromPath})
		}
	}
	return errtrace.Ok(ChromeDetection{Source: ChromeNotFound})
}

// cachedSnapshot returns the last cached snapshot under RLock. Falls back
// to a freshly loaded one if nothing is cached yet.
func (s *Settings) cachedSnapshot() SettingsSnapshot {
	s.mu.RLock()
	snap := s.lastApplied
	s.mu.RUnlock()
	if snap.PollSeconds != 0 {
		return snap
	}
	r := s.Get(context.Background())
	if r.HasError() {
		return SettingsSnapshot{}
	}
	return r.Value()
}

// ---------------------------------------------------------------------------
// Raw config IO — wraps internal/config to add the settings extension block.
// ---------------------------------------------------------------------------

// loadRaw reads config.json into a Config + extension. Missing extension
// keys yield zero-value defaults (caller layers DefaultSettingsInput).
func loadRaw() (*rawConfigWithSettings, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.WrapCode(err, errtrace.ErrConfigOpen, "load config")
	}
	// Re-read the file to extract the optional "settings" block. config.Load
	// silently drops unknown keys, so we have to round-trip through os.ReadFile
	// + a permissive struct.
	ext, err := loadExtension()
	if err != nil {
		return nil, err
	}
	return &rawConfigWithSettings{cfg: cfg, ext: ext}, nil
}

// saveRaw writes both the typed config fields and the extension block in a
// single atomic file write. We use a generic map round-trip so unknown
// top-level keys (anything other than accounts/rules/watch/browser/settings)
// survive untouched — important once additional consumers start adding
// their own top-level objects.
func saveRaw(raw *rawConfigWithSettings) error {
	p, err := config.Path()
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigOpen, "save raw path")
	}
	root, err := readConfigAsMap(p)
	if err != nil {
		return errtrace.Wrap(err, "saveRaw: readConfigAsMap")
	}
	// Marshal the typed config and merge its top-level keys into root so we
	// preserve unknown keys.
	cfgBytes, err := json.Marshal(raw.cfg)
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigEncode, "encode typed config")
	}
	var typed map[string]any
	if err := json.Unmarshal(cfgBytes, &typed); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrConfigEncode, "round-trip typed config")
	}
	for k, v := range typed {
		root[k] = v
	}
	root["settings"] = raw.ext
	return writeConfigMap(p, root)
}

// applyInputToRaw mutates raw with the values from in. The on-disk
// camelCase fields (Watch.PollSeconds, Browser.*) are updated; the new
// fields land in the extension block.
func applyInputToRaw(raw *rawConfigWithSettings, in SettingsInput, now time.Time) {
	raw.cfg.Watch.PollSeconds = int(in.PollSeconds)
	raw.cfg.Browser.ChromePath = in.BrowserOverride.ChromePath
	raw.cfg.Browser.IncognitoArg = in.BrowserOverride.IncognitoArg
	raw.ext.Theme = in.Theme.String()
	raw.ext.Density = in.Density.String()
	raw.ext.OpenUrlAllowedSchemes = append([]string(nil), in.OpenUrlAllowedSchemes...)
	raw.ext.AllowLocalhostUrls = in.AllowLocalhostUrls
	raw.ext.AutoStartWatch = in.AutoStartWatch
	raw.ext.OpenUrlsRetentionDays = in.OpenUrlsRetentionDays
	raw.ext.WeeklyVacuumOn = in.WeeklyVacuumOn.String()
	raw.ext.WeeklyVacuumHourLocal = in.WeeklyVacuumHourLocal
	raw.ext.WalCheckpointHours = in.WalCheckpointHours
	raw.ext.PruneBatchSize = in.PruneBatchSize
	raw.ext.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
}

// snapshotFromRaw builds a SettingsSnapshot from the loaded raw form,
// applying defaults for any missing extension fields.
func snapshotFromRaw(raw *rawConfigWithSettings) (SettingsSnapshot, error) {
	paths, err := resolveSnapshotPaths()
	if err != nil {
		return SettingsSnapshot{}, err
	}
	ext := projectExtension(raw.ext)
	updatedAt, _ := time.Parse(time.RFC3339Nano, raw.ext.UpdatedAt)
	return SettingsSnapshot{
		PollSeconds: clampPollSeconds(raw.cfg.Watch.PollSeconds),
		BrowserOverride: BrowserOverride{
			ChromePath:   raw.cfg.Browser.ChromePath,
			IncognitoArg: raw.cfg.Browser.IncognitoArg,
		},
		Theme:                 ext.theme,
		Density:               ext.density,
		OpenUrlAllowedSchemes: ext.schemes,
		AllowLocalhostUrls:    raw.ext.AllowLocalhostUrls,
		AutoStartWatch:        ext.autoStart,
		OpenUrlsRetentionDays: ext.retention,
		WeeklyVacuumOn:        ext.weekday,
		WeeklyVacuumHourLocal: ext.vacHour,
		WalCheckpointHours:    ext.walHours,
		PruneBatchSize:        ext.batchSize,
		ConfigPath:            paths.cfg,
		DataDir:               paths.data,
		EmailArchiveDir:       paths.email,
		UpdatedAt:             updatedAt,
	}, nil
}

type snapshotPaths struct{ cfg, data, email string }

// resolveSnapshotPaths fetches the three absolute display paths surfaced
// on every Get. Pulled out of snapshotFromRaw to keep that function under
// the 15-statement linter cap.
func resolveSnapshotPaths() (snapshotPaths, error) {
	cfgPath, err := config.Path()
	if err != nil {
		return snapshotPaths{}, errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "resolve config path")
	}
	dataDir, err := config.DataDir()
	if err != nil {
		return snapshotPaths{}, errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "resolve data dir")
	}
	emailDir, err := config.EmailDir()
	if err != nil {
		return snapshotPaths{}, errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "resolve email dir")
	}
	return snapshotPaths{cfg: cfgPath, data: dataDir, email: emailDir}, nil
}

type projectedExtension struct {
	theme     ThemeMode
	density   Density
	schemes   []string
	autoStart bool
	retention uint16
	weekday   time.Weekday
	vacHour   uint8
	walHours  uint8
	batchSize uint32
}

// projectThemeDensitySchemes resolves the §-presentation fields (theme,
// density, allowed URL schemes) with documented fallbacks. Extracted from
// projectExtension to keep that function under the 15-statement linter
// budget (AC-PROJ-20).
func projectThemeDensitySchemes(ext settingsExtension, defaults SettingsInput) (ThemeMode, Density, []string) {
	theme, _ := ParseThemeMode(ext.Theme)
	if ext.Theme == "" {
		theme = defaults.Theme
	}
	density, _ := ParseDensity(ext.Density)
	if ext.Density == "" {
		density = defaults.Density
	}
	schemes := canonSchemes(ext.OpenUrlAllowedSchemes)
	if len(schemes) == 0 {
		schemes = defaults.OpenUrlAllowedSchemes
	}
	return theme, density, schemes
}

// projectMaintenanceFields resolves the maintenance/retention fields. A
// fresh-on-disk file (empty UpdatedAt) re-defaults all bookkeeping fields
// so partially-written extensions don't leak zero-valued retention.
func projectMaintenanceFields(ext settingsExtension, defaults SettingsInput) (bool, uint16, time.Weekday, uint8, uint8, uint32) {
	autoStart := ext.AutoStartWatch
	retention := ext.OpenUrlsRetentionDays
	if ext.UpdatedAt == "" {
		autoStart = defaults.AutoStartWatch
		retention = defaults.OpenUrlsRetentionDays
	}
	weekday, ok := ParseWeekday(ext.WeeklyVacuumOn)
	if !ok {
		weekday = defaults.WeeklyVacuumOn
	}
	vacHour := ext.WeeklyVacuumHourLocal
	if ext.UpdatedAt == "" || vacHour > 23 {
		vacHour = defaults.WeeklyVacuumHourLocal
	}
	walHours := ext.WalCheckpointHours
	if ext.UpdatedAt == "" || walHours == 0 {
		walHours = defaults.WalCheckpointHours
	}
	batchSize := ext.PruneBatchSize
	if ext.UpdatedAt == "" || batchSize == 0 {
		batchSize = defaults.PruneBatchSize
	}
	return autoStart, retention, weekday, vacHour, walHours, batchSize
}

// projectExtension layers DefaultSettingsInput defaults over an extension
// block read from disk. Missing values (empty Theme, nil schemes, fresh
// file with empty UpdatedAt) fall back to the documented defaults.
func projectExtension(ext settingsExtension) projectedExtension {
	defaults := DefaultSettingsInput()
	theme, density, schemes := projectThemeDensitySchemes(ext, defaults)
	autoStart, retention, weekday, vacHour, walHours, batchSize := projectMaintenanceFields(ext, defaults)
	return projectedExtension{
		theme: theme, density: density, schemes: schemes, autoStart: autoStart, retention: retention,
		weekday: weekday, vacHour: vacHour, walHours: walHours, batchSize: batchSize,
	}
}

// clampPollSeconds projects an int from the legacy schema into the uint16
// range expected by SettingsSnapshot, applying the documented default.
func clampPollSeconds(v int) uint16 {
	if v < config.MinWatchPollSeconds {
		return config.DefaultWatchPollSeconds
	}
	if v > config.MaxWatchPollSeconds {
		return config.MaxWatchPollSeconds
	}
	return uint16(v)
}

// fileExistsExec returns true if path exists, is a file, and (on POSIX) has
// any executable bit set. Mirrors the existing internal/browser logic so the
// detection is consistent.
func fileExistsExec(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

// osDefaultChromePaths returns the well-known absolute paths to probe per OS.
func osDefaultChromePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		}
	default:
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
			"/usr/bin/brave-browser",
			"/snap/bin/chromium",
		}
	}
}
