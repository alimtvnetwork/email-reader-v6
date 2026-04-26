// doctor.go is the structured backend for the `email-read doctor`
// inspection. It returns one DoctorReport per account so the CLI and the
// Tools UI can both render it (no more "fmt.Printf" inside the CLI body).
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md (doctor sub-tool).
package core

import (
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// DoctorReport captures the diagnostic for a single account: the
// stored-bytes view, the sanitized view, and rune-dumps for both.
// Hidden = true when sanitized != raw, i.e. there ARE invisible chars.
type DoctorReport struct {
	Alias        string
	Email        string
	StoredBytes  int          // length of raw decoded bytes (pre-sanitization)
	RuneCount    int          // length of sanitized rune slice
	Hidden       bool         // raw != sanitized → invisible chars present
	DecodeError  string       // non-empty when DecodePassword failed
	Sanitized    []DoctorRune // rune dump of the sanitized password
	Raw          []DoctorRune // rune dump of the raw bytes (only when Hidden)
}

// DoctorRune is one entry in the rune-dump table: the visible glyph plus
// its Unicode codepoint and zero-based index.
type DoctorRune struct {
	Index int
	Code  rune
	Glyph string // string(Code) — what the user sees
}

// Doctor inspects every configured account (or only `target` when set)
// and returns a structured report. Pure aside from config.Load — no IO,
// no IMAP. Errors that DON'T stop the iteration (e.g. one bad password)
// surface as DoctorReport.DecodeError; only catastrophic failures (no
// config, no accounts, target not found) come back as a Result error.
func Doctor(target string) errtrace.Result[[]DoctorReport] {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Err[[]DoctorReport](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "doctor: load config"))
	}
	if len(cfg.Accounts) == 0 {
		return errtrace.Err[[]DoctorReport](errtrace.NewCoded(
			errtrace.ErrConfigAccountMissing, "doctor: no accounts configured"))
	}
	out := collectReports(cfg.Accounts, target)
	if target != "" && len(out) == 0 {
		return errtrace.Err[[]DoctorReport](errtrace.NewCoded(
			errtrace.ErrConfigAccountMissing, "doctor: alias not found: "+target))
	}
	return errtrace.Ok(out)
}

// collectReports iterates accounts, applying the optional alias filter
// and building one DoctorReport per match. Extracted to keep Doctor at
// ≤15 statements per coding standards.
func collectReports(accts []config.Account, target string) []DoctorReport {
	out := make([]DoctorReport, 0, len(accts))
	for _, a := range accts {
		if target != "" && a.Alias != target {
			continue
		}
		out = append(out, buildDoctorReport(a))
	}
	return out
}

// buildDoctorReport runs the per-account decode + rune-dump.
func buildDoctorReport(a config.Account) DoctorReport {
	rep := DoctorReport{Alias: a.Alias, Email: a.Email}
	pw, perr := config.DecodePassword(a.PasswordB64)
	if perr != nil {
		rep.DecodeError = perr.Error()
		return rep
	}
	rawBytes, _ := config.DecodeRawPassword(a.PasswordB64)
	rawStr := string(rawBytes)
	rep.StoredBytes = len(rawStr)
	rep.RuneCount = len([]rune(pw))
	rep.Hidden = rawStr != pw
	rep.Sanitized = runeDump(pw)
	if rep.Hidden {
		rep.Raw = runeDump(rawStr)
	}
	return rep
}

// runeDump converts a string to a slice of DoctorRune entries (one per
// codepoint, with the original index — counts grapheme clusters as
// separate runes which is the desired diagnostic view).
func runeDump(s string) []DoctorRune {
	out := make([]DoctorRune, 0, len([]rune(s)))
	for i, r := range []rune(s) {
		out = append(out, DoctorRune{Index: i, Code: r, Glyph: string(r)})
	}
	return out
}
