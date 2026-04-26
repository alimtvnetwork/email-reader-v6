// settings_validate.go enforces the validation rules from
// spec/21-app/02-features/07-settings/01-backend.md §6. Each rule maps to a
// dedicated error code (ER-SET-21771..21777) so UIs and tests can switch on
// the code rather than parsing free-form strings.
package core

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// disallowedSchemes are blocked outright (XSS / data-exfil vectors).
var disallowedSchemes = map[string]struct{}{
	"file":       {},
	"javascript": {},
	"data":       {},
	"vbscript":   {},
}

var (
	schemeRegex    = regexp.MustCompile(`^[a-z][a-z0-9+\-.]*$`)
	incognitoRegex = regexp.MustCompile(`^--?[a-zA-Z][a-zA-Z0-9\-]*$`)
)

// normalizeInput trims whitespace, lower-cases schemes, dedupes + sorts them,
// and trims the BrowserOverride strings. It also substitutes defaults for the
// zero-valued maintenance knobs so callers (and existing tests) that construct
// a partial SettingsInput don't trip the §5 range validators on fields they
// never touched. It does NOT validate.
func normalizeInput(in SettingsInput) SettingsInput {
	in.BrowserOverride.ChromePath = strings.TrimSpace(in.BrowserOverride.ChromePath)
	in.BrowserOverride.IncognitoArg = strings.TrimSpace(in.BrowserOverride.IncognitoArg)
	in.OpenUrlAllowedSchemes = canonSchemes(in.OpenUrlAllowedSchemes)
	defaults := DefaultSettingsInput()
	if in.WalCheckpointHours == 0 {
		in.WalCheckpointHours = defaults.WalCheckpointHours
	}
	if in.PruneBatchSize == 0 {
		in.PruneBatchSize = defaults.PruneBatchSize
	}
	// WeeklyVacuumOn/HourLocal: zero is valid (Sunday/midnight); leave as-is.
	return in
}

// canonSchemes lower-cases, trims, dedupes, and sorts.
func canonSchemes(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// validateInput runs each §6 rule in code order, returning the first error.
func validateInput(in SettingsInput) error {
	if err := validatePollSeconds(in.PollSeconds); err != nil {
		return err
	}
	if err := validateTheme(in.Theme); err != nil {
		return err
	}
	if err := validateDensity(in.Density); err != nil {
		return err
	}
	if err := validateSchemes(in.OpenUrlAllowedSchemes); err != nil {
		return err
	}
	if err := validateChromePath(in.BrowserOverride.ChromePath); err != nil {
		return err
	}
	if err := validateIncognitoArg(in.BrowserOverride.IncognitoArg); err != nil {
		return err
	}
	if err := validateRetentionDays(in.OpenUrlsRetentionDays); err != nil {
		return err
	}
	if err := validateMaintenanceKnobs(in); err != nil {
		return err
	}
	return validateLocalhostComposite(in)
}

// validateRetentionDays caps the OpenedUrls audit-row lifetime at 10 years.
// 0 disables retention entirely (per spec/23-app-database/04-retention §1).
func validateRetentionDays(v uint16) error {
	if v > 3650 {
		return errtrace.NewCoded(errtrace.ErrSettingsRetentionDays,
			"retention days out of range").
			WithContext("value", v).
			WithContext("min", 0).
			WithContext("max", 3650)
	}
	return nil
}

// validateMaintenanceKnobs enforces the spec/23-app-database/04 §5 ranges.
// All four rows share ER-SET-21778 per the spec table.
func validateMaintenanceKnobs(in SettingsInput) error {
	if in.WeeklyVacuumOn < time.Sunday || in.WeeklyVacuumOn > time.Saturday {
		return errtrace.NewCoded(errtrace.ErrSettingsPersist,
			"weekly vacuum weekday out of range").
			WithContext("value", int(in.WeeklyVacuumOn))
	}
	if in.WeeklyVacuumHourLocal > 23 {
		return errtrace.NewCoded(errtrace.ErrSettingsPersist,
			"weekly vacuum hour out of range").
			WithContext("value", in.WeeklyVacuumHourLocal).
			WithContext("min", 0).WithContext("max", 23)
	}
	if in.WalCheckpointHours < 1 || in.WalCheckpointHours > 168 {
		return errtrace.NewCoded(errtrace.ErrSettingsPersist,
			"wal checkpoint hours out of range").
			WithContext("value", in.WalCheckpointHours).
			WithContext("min", 1).WithContext("max", 168)
	}
	if in.PruneBatchSize < 100 || in.PruneBatchSize > 50000 {
		return errtrace.NewCoded(errtrace.ErrSettingsPersist,
			"prune batch size out of range").
			WithContext("value", in.PruneBatchSize).
			WithContext("min", 100).WithContext("max", 50000)
	}
	return nil
}

func validatePollSeconds(v uint16) error {
	if v < 1 || v > 60 {
		return errtrace.NewCoded(errtrace.ErrSettingsPollSeconds,
			"poll seconds out of range").
			WithContext("value", v).
			WithContext("min", 1).
			WithContext("max", 60)
	}
	return nil
}

func validateTheme(t ThemeMode) error {
	switch t {
	case ThemeDark, ThemeLight, ThemeSystem:
		return nil
	}
	return errtrace.NewCoded(errtrace.ErrSettingsTheme, "unknown theme mode").
		WithContext("value", uint8(t))
}

func validateDensity(d Density) error {
	switch d {
	case DensityComfortable, DensityCompact:
		return nil
	}
	return errtrace.NewCoded(errtrace.ErrSettingsDensity, "unknown density mode").
		WithContext("value", uint8(d))
}

func validateSchemes(schemes []string) error {
	if len(schemes) == 0 {
		return errtrace.NewCoded(errtrace.ErrSettingsUrlScheme,
			"at least one allowed scheme required")
	}
	for _, s := range schemes {
		if _, bad := disallowedSchemes[s]; bad {
			return errtrace.NewCoded(errtrace.ErrSettingsUrlScheme,
				"disallowed url scheme").WithContext("scheme", s)
		}
		if !schemeRegex.MatchString(s) {
			return errtrace.NewCoded(errtrace.ErrSettingsUrlScheme,
				"malformed url scheme").WithContext("scheme", s)
		}
	}
	return nil
}

func validateChromePath(p string) error {
	if p == "" {
		return nil
	}
	if !isAbsolutePath(p) {
		return errtrace.NewCoded(errtrace.ErrSettingsChromePath,
			"chrome path must be absolute").WithContext("path", p)
	}
	info, err := os.Stat(p)
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrSettingsChromePath,
			"chrome path stat failed").WithContext("path", p)
	}
	if info.IsDir() {
		return errtrace.NewCoded(errtrace.ErrSettingsChromePath,
			"chrome path is a directory").WithContext("path", p)
	}
	return nil
}

func validateIncognitoArg(arg string) error {
	if arg == "" {
		return nil
	}
	if !incognitoRegex.MatchString(arg) {
		return errtrace.NewCoded(errtrace.ErrSettingsIncognitoArg,
			"incognito arg malformed").WithContext("arg", arg)
	}
	return nil
}

// validateLocalhostComposite enforces the §6 ER-SET-21777 rule:
// AllowLocalhostUrls=true requires "http" in OpenUrlAllowedSchemes.
func validateLocalhostComposite(in SettingsInput) error {
	if !in.AllowLocalhostUrls {
		return nil
	}
	for _, s := range in.OpenUrlAllowedSchemes {
		if s == "http" {
			return nil
		}
	}
	return errtrace.NewCoded(errtrace.ErrSettingsCompositeRule,
		"localhost requires http scheme")
}

// isAbsolutePath is a tiny wrapper so we can stub it in tests without pulling
// in path/filepath here (and to keep the OS-specific behaviour explicit).
func isAbsolutePath(p string) bool {
	if p == "" {
		return false
	}
	// Unix absolute path or Windows drive-letter path.
	if p[0] == '/' || p[0] == '\\' {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		return true
	}
	return false
}
