// tools_log_scan_test.go — AC-DB-55 `Test_LogScan_NoOriginalUrlLeak`.
//
// Spec: spec/23-app-database/97-acceptance-criteria.md §F.
//
// Goal: prove that the pre-redaction `OriginalUrl` value never appears
// in any log record emitted at or above INFO level by the OpenUrl path
// (or by any helper it calls). Passwords, OTPs, and userinfo embedded
// in URLs are sensitive — they're allowed in DEBUG (developer
// telemetry) but must never reach INFO/WARN/ERROR sinks where they may
// land in shipped log files.
//
// Strategy:
//  1. Install a buffering `slog.Handler` as the process default for
//     the duration of the test (restored on cleanup).
//  2. Also redirect the legacy `log` package writer into the same
//     buffer so a stray `log.Printf` would still be caught.
//  3. Drive `Tools.OpenUrl` with a URL whose original form contains
//     both userinfo (`user:pw`) and a sensitive query value (`otp=123456`).
//     Both must be stripped/masked by `redactUrl` before any log call.
//  4. Walk every captured `slog.Record` whose `Level >= LevelInfo`,
//     plus every legacy log line, and assert none contain the
//     sensitive substrings.
//
// The test deliberately exercises *both* the launch-success path and
// the dedup-hit path (which has its own `recordAuditExt` branch), so
// future log additions on either branch would be caught.
package core

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/lovable/email-read/internal/store"
)

func Test_LogScan_NoOriginalUrlLeak(t *testing.T) {
	// Cannot run in parallel — mutates process-global slog default and
	// log writer.

	const (
		sensitiveUserinfo = "user:pw"
		sensitiveOtp      = "123456"
		rawUrl            = "https://" + sensitiveUserinfo + "@example.com/login?otp=" + sensitiveOtp + "&q=ok"
	)

	// Sanity: confirm the test inputs really do trigger the redactor.
	canonical, original := redactUrl(rawUrl)
	if canonical == original {
		t.Fatalf("test setup bug: redactor produced no change for %q", rawUrl)
	}
	if !strings.Contains(original, sensitiveUserinfo) || !strings.Contains(original, sensitiveOtp) {
		t.Fatalf("test setup bug: original lost sensitive markers: %q", original)
	}
	if strings.Contains(canonical, sensitiveUserinfo) || strings.Contains(canonical, sensitiveOtp) {
		t.Fatalf("redactUrl failed to scrub: canonical=%q", canonical)
	}

	scan := installCapturingLogger(t)

	br := &fakeBrowser{}
	st := newFakeStore()
	tools := NewTools(br, st, DefaultToolsConfig()).Value()

	ctx := context.Background()
	spec := OpenUrlSpec{
		Url:     rawUrl,
		Origin:  OriginManual,
		Alias:   "atto",
		EmailId: 0, // skip persistent insert; keep the assertion focused on logs
	}

	// First call — should launch and (in any future log addition) emit
	// a "launched" log entry.
	r1 := tools.OpenUrl(ctx, spec)
	if r1.Error() != nil {
		t.Fatalf("OpenUrl#1: %v", r1.Error())
	}
	if r1.Value().Deduped {
		t.Fatal("OpenUrl#1 unexpectedly deduped")
	}

	// Second call — should hit the in-memory dedup branch and (in any
	// future log addition) emit a "deduped" log entry.
	r2 := tools.OpenUrl(ctx, spec)
	if r2.Error() != nil {
		t.Fatalf("OpenUrl#2: %v", r2.Error())
	}
	if !r2.Value().Deduped {
		t.Fatal("OpenUrl#2 should have been deduped")
	}

	// Cross-check: the in-memory result objects do legitimately carry
	// the original URL (it's part of the API surface), but those are
	// returned to the caller, not logged.
	if !strings.Contains(r1.Value().OriginalUrl, sensitiveUserinfo) {
		t.Fatalf("API contract changed: OriginalUrl no longer carries userinfo: %q", r1.Value().OriginalUrl)
	}

	// Now the actual AC: scan logs.
	if leaks := scan.scanForSubstrings(sensitiveUserinfo, sensitiveOtp); len(leaks) > 0 {
		t.Fatalf("AC-DB-55: pre-redaction OriginalUrl leaked at INFO+ in %d log record(s):\n%s",
			len(leaks), strings.Join(leaks, "\n"))
	}

	// Bonus assertion: silence the AccountEvent-style audit by feeding a
	// store that returns RecordOpenedUrlExt from a successful EmailId
	// path too; then re-scan. Keeps the test tight against any future
	// log emissions on the persistent-write branch.
	stPersist := newFakeStore()
	toolsP := NewTools(br, stPersist, DefaultToolsConfig()).Value()
	specP := spec
	specP.EmailId = 99
	specP.Url = rawUrl // redact path again
	rp := toolsP.OpenUrl(ctx, specP)
	if rp.Error() != nil {
		t.Fatalf("OpenUrl persist branch: %v", rp.Error())
	}
	if leaks := scan.scanForSubstrings(sensitiveUserinfo, sensitiveOtp); len(leaks) > 0 {
		t.Fatalf("AC-DB-55 (persist branch): leaked in %d record(s):\n%s",
			len(leaks), strings.Join(leaks, "\n"))
	}
}

// logScan captures both `slog` records (≥ INFO) and legacy `log`
// package writes during the test, with a mutex guarding both sinks.
type logScan struct {
	mu        sync.Mutex
	slogLines []string // formatted slog records at LevelInfo+
	legacyBuf *bytes.Buffer
}

// scanForSubstrings returns any captured log line that contains any of
// the forbidden substrings.
func (s *logScan) scanForSubstrings(needles ...string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var hits []string
	all := append([]string{}, s.slogLines...)
	for _, line := range strings.Split(s.legacyBuf.String(), "\n") {
		if line != "" {
			all = append(all, "log: "+line)
		}
	}
	for _, line := range all {
		for _, n := range needles {
			if strings.Contains(line, n) {
				hits = append(hits, line)
				break
			}
		}
	}
	return hits
}

// installCapturingLogger swaps the process-default slog logger and the
// `log` package writer, restoring both on test cleanup.
func installCapturingLogger(t *testing.T) *logScan {
	t.Helper()
	scan := &logScan{legacyBuf: &bytes.Buffer{}}

	prevSlog := slog.Default()
	prevLogWriter := log.Writer()
	prevLogFlags := log.Flags()

	handler := &captureHandler{scan: scan, level: slog.LevelInfo}
	slog.SetDefault(slog.New(handler))
	log.SetOutput(io.MultiWriter(scan.legacyBuf))
	log.SetFlags(0)

	t.Cleanup(func() {
		slog.SetDefault(prevSlog)
		log.SetOutput(prevLogWriter)
		log.SetFlags(prevLogFlags)
	})
	return scan
}

// captureHandler is a minimal slog.Handler that buffers records at
// `level` and above. We rebuild a printable line from the message plus
// every attribute (key=value), which covers all spellings — `slog.String`,
// `slog.Any`, structured groups, etc.
type captureHandler struct {
	scan  *logScan
	level slog.Level
	attrs []slog.Attr
	group string
}

func (h *captureHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String())
	b.WriteByte(' ')
	b.WriteString(r.Message)
	for _, a := range h.attrs {
		b.WriteByte(' ')
		writeAttr(&b, h.group, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		writeAttr(&b, h.group, a)
		return true
	})
	h.scan.mu.Lock()
	h.scan.slogLines = append(h.scan.slogLines, b.String())
	h.scan.mu.Unlock()
	return nil
}

func writeAttr(b *strings.Builder, group string, a slog.Attr) {
	if group != "" {
		b.WriteString(group)
		b.WriteByte('.')
	}
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(a.Value.String())
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := append([]slog.Attr{}, h.attrs...)
	merged = append(merged, attrs...)
	return &captureHandler{scan: h.scan, level: h.level, attrs: merged, group: h.group}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &captureHandler{scan: h.scan, level: h.level, attrs: h.attrs, group: g}
}

// Compile-time confirmation we satisfy slog.Handler.
var _ slog.Handler = (*captureHandler)(nil)

// Touch store.OpenedUrlInsert so a future refactor that drops the field
// in this test file doesn't silently lose its compile-time link.
var _ = store.OpenedUrlInsert{}
