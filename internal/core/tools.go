// tools.go is the slim first slice of the `core.Tools` service. It
// implements the OpenUrl sub-tool — the only authorised browser-launch
// path — with full validation, redaction, and dedup semantics per
// spec/21-app/02-features/06-tools/01-backend.md §2.4 + 00-overview.md §5.
//
// Future slices add ReadOnce, ExportCsv, Diagnose, and the AccountEvent
// cache invalidation. Those land behind their own files (tools_read.go,
// tools_export.go, tools_diagnose.go) so this surface stays under 200 LOC.
package core

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// OpenUrlOrigin captures the caller context for the audit row. The UI
// passes Manual; rule-driven launches pass Rule; CLI passes Cli.
type OpenUrlOrigin string

const (
	OriginManual OpenUrlOrigin = "manual"
	OriginRule   OpenUrlOrigin = "rule"
	OriginCli    OpenUrlOrigin = "cli"
)

// ToolsConfig mirrors the validated subset of config.Settings consumed
// by the OpenUrl sub-tool. Other sub-tools add their own fields.
type ToolsConfig struct {
	OpenUrlDedupSeconds   int
	OpenUrlMaxLengthBytes int
	OpenUrlAllowedSchemes []string
	AllowLocalhostUrls    bool
	AllowPrivateIpUrls    bool
}

// DefaultToolsConfig returns the spec-mandated defaults (§2.4).
func DefaultToolsConfig() ToolsConfig {
	return ToolsConfig{
		OpenUrlDedupSeconds:   60,
		OpenUrlMaxLengthBytes: 8192,
		OpenUrlAllowedSchemes: []string{"https", "http"},
	}
}

// urlLauncher is the slim browser surface Tools depends on. Implemented
// by *browser.Launcher; abstracted so tests can inject a stub.
type urlLauncher interface {
	Open(url string) error
	Path() (string, error)
}

// openedUrlRecorder is the slim store surface for OpenUrl audit + dedup.
// Tests inject an in-memory implementation.
type openedUrlRecorder interface {
	HasOpenedUrl(ctx context.Context, emailId int64, url string) (bool, error)
	RecordOpenedUrl(ctx context.Context, emailId int64, ruleName, url string) (bool, error)
}

// Tools is the unified service holding the four sub-tool implementations.
// OpenUrl + ReadOnce + ExportCsv + CachedDiagnose + RecentOpenedUrls are
// wired in this slice; the AccountEvent cache invalidation hook lands
// when `core.Accounts` exposes a Subscribe channel.
type Tools struct {
	browser       urlLauncher
	store         openedUrlRecorder
	cfg           ToolsConfig
	openMu        sync.Mutex // serialises per-key dedup checks
	keys          map[string]time.Time
	diagCacheOnce sync.Once
	diagCachePtr  *diagnoseCache
}

// NewTools constructs a Tools service. Validation: scheme list non-empty,
// length bound 256..65536, dedup 0..3600s; otherwise ER-TLS-21750.
func NewTools(b urlLauncher, st openedUrlRecorder, cfg ToolsConfig) errtrace.Result[*Tools] {
	if err := validateToolsConfig(cfg); err != nil {
		return errtrace.Err[*Tools](err)
	}
	return errtrace.Ok(&Tools{browser: b, store: st, cfg: cfg, keys: map[string]time.Time{}})
}

func validateToolsConfig(cfg ToolsConfig) error {
	if cfg.OpenUrlMaxLengthBytes < 256 || cfg.OpenUrlMaxLengthBytes > 65536 {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "OpenUrlMaxLengthBytes out of [256,65536]")
	}
	if cfg.OpenUrlDedupSeconds < 0 || cfg.OpenUrlDedupSeconds > 3600 {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "OpenUrlDedupSeconds out of [0,3600]")
	}
	if len(cfg.OpenUrlAllowedSchemes) == 0 {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "OpenUrlAllowedSchemes empty")
	}
	for _, s := range cfg.OpenUrlAllowedSchemes {
		if s != "http" && s != "https" {
			return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "OpenUrlAllowedSchemes ⊄ {http,https}")
		}
	}
	return nil
}

// OpenUrlSpec is the request shape for OpenUrl.
type OpenUrlSpec struct {
	Url      string
	Origin   OpenUrlOrigin
	Alias    string
	RuleName string
	EmailId  int64
}

// OpenUrlReport summarises the outcome.
type OpenUrlReport struct {
	Url           string // canonical (post-redaction)
	OriginalUrl   string // pre-redaction
	BrowserBinary string
	Deduped       bool
	IsIncognito   bool
}

// OpenUrl is the only authorised browser-launch path. Validates →
// redacts → dedup-checks → launches → audits per overview §5.
func (t *Tools) OpenUrl(ctx context.Context, spec OpenUrlSpec) errtrace.Result[OpenUrlReport] {
	if err := t.validateUrl(spec.Url); err != nil {
		return errtrace.Err[OpenUrlReport](err)
	}
	canonical, original := redactUrl(spec.Url)
	if dup, err := t.checkDedup(ctx, spec, canonical); err != nil {
		return errtrace.Err[OpenUrlReport](err)
	} else if dup {
		return errtrace.Ok(OpenUrlReport{Url: canonical, OriginalUrl: original, Deduped: true, IsIncognito: true})
	}
	binary, err := t.browser.Path()
	if err != nil {
		return errtrace.Err[OpenUrlReport](errtrace.WrapCode(err, errtrace.ErrToolsOpenUrlNoBrowser, "browser.Path"))
	}
	if err := t.browser.Open(canonical); err != nil {
		return errtrace.Err[OpenUrlReport](errtrace.WrapCode(err, errtrace.ErrToolsOpenUrlLaunchFailed, "browser.Open"))
	}
	t.recordAudit(ctx, spec, canonical)
	return errtrace.Ok(OpenUrlReport{Url: canonical, OriginalUrl: original, BrowserBinary: binary, IsIncognito: true})
}

// validateUrl runs the §5.1 pipeline; first failure short-circuits.
func (t *Tools) validateUrl(raw string) error {
	if raw == "" {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlEmpty, "url empty")
	}
	if len(raw) > t.cfg.OpenUrlMaxLengthBytes {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlTooLong, fmt.Sprintf("url > %d bytes", t.cfg.OpenUrlMaxLengthBytes))
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errtrace.WrapCode(err, errtrace.ErrToolsOpenUrlMalformed, "url.Parse")
	}
	if !schemeAllowed(u.Scheme, t.cfg.OpenUrlAllowedSchemes) {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlScheme, "scheme "+u.Scheme+" not allowed")
	}
	if u.Host == "" {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlMalformed, "url host empty")
	}
	return t.validateHost(u.Hostname())
}

func (t *Tools) validateHost(host string) error {
	if !t.cfg.AllowLocalhostUrls && isLoopback(host) {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlLocalhost, "loopback host blocked: "+host)
	}
	if !t.cfg.AllowPrivateIpUrls && isPrivateIp(host) {
		return errtrace.NewCoded(errtrace.ErrToolsOpenUrlPrivateIp, "private-ip host blocked: "+host)
	}
	return nil
}

func schemeAllowed(scheme string, allowed []string) bool {
	s := strings.ToLower(scheme)
	for _, a := range allowed {
		if strings.ToLower(a) == s {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func isPrivateIp(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// checkDedup consults the in-memory key index AND the persistent store.
// Returns true iff a recent open exists within the dedup window.
func (t *Tools) checkDedup(ctx context.Context, spec OpenUrlSpec, canonical string) (bool, error) {
	if t.cfg.OpenUrlDedupSeconds == 0 {
		return false, nil
	}
	key := spec.Alias + "|" + canonical
	t.openMu.Lock()
	last, ok := t.keys[key]
	t.openMu.Unlock()
	if ok && time.Since(last) < time.Duration(t.cfg.OpenUrlDedupSeconds)*time.Second {
		return true, nil
	}
	if spec.EmailId > 0 {
		hit, err := t.store.HasOpenedUrl(ctx, spec.EmailId, canonical)
		if err != nil {
			return false, errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "store.HasOpenedUrl")
		}
		if hit {
			return true, nil
		}
	}
	return false, nil
}

// recordAudit writes the persistent + in-memory audit entries. Failures
// are logged-and-swallowed: the launch already happened (§7 trade-off).
func (t *Tools) recordAudit(ctx context.Context, spec OpenUrlSpec, canonical string) {
	key := spec.Alias + "|" + canonical
	t.openMu.Lock()
	t.keys[key] = time.Now()
	t.openMu.Unlock()
	if spec.EmailId > 0 {
		_, _ = t.store.RecordOpenedUrl(ctx, spec.EmailId, spec.RuleName, canonical)
	}
}
