// cf_acceptance_openurl_test.go locks the cross-feature spec contracts
// for the OpenUrl validation chokepoint:
//
//	CF-T2 — `OpenUrlAllowedSchemes` is the single source of truth — every
//	        caller (Tools UI, Rules engine, Watch loop) gets the SAME
//	        rejection code regardless of Origin.
//	CF-T3 — `AllowLocalhostUrls = false` causes OpenUrl to reject
//	        `http://localhost`, `http://127.0.0.1`, `http://[::1]` with
//	        ER-TLS-21764 (`ErrToolsOpenUrlLocalhost`).
//	CF-R1 — Rule "Open URL" actions go through `core.Tools.OpenUrl`,
//	        which honours the scheme allowlist (single chokepoint).
//
// Spec: spec/21-app/02-features/07-settings/99-consistency-report.md
// §CF-Tools and §CF-Rules tables.
package core

import (
	"context"
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
)

// TestCF_T2_OpenUrl_SchemeRejection_AllCallers parametrises the same
// disallowed URLs over every Origin. Each (caller, url) pair must
// surface the same coded error and never call the launcher — proving
// `OpenUrlAllowedSchemes` is the single source of truth.
func TestCF_T2_OpenUrl_SchemeRejection_AllCallers(t *testing.T) {
	disallowed := []struct {
		name string
		url  string
		code errtrace.Code
	}{
		{"javascript", "javascript:alert(1)", errtrace.ErrToolsOpenUrlScheme},
		{"file", "file:///etc/passwd", errtrace.ErrToolsOpenUrlScheme},
		{"data", "data:text/html,<script>", errtrace.ErrToolsOpenUrlScheme},
		{"chrome-internal", "chrome://settings", errtrace.ErrToolsOpenUrlScheme},
	}
	callers := []OpenUrlOrigin{OriginManual, OriginRule, OriginCli}
	for _, c := range callers {
		for _, tc := range disallowed {
			name := string(c) + "/" + tc.name
			t.Run(name, func(t *testing.T) {
				assertOpenUrlRejected(t, OpenUrlSpec{Url: tc.url, Origin: c}, tc.code)
			})
		}
	}
}

// TestCF_T3_OpenUrl_LocalhostBlocked_Default proves the spec default
// `AllowLocalhostUrls = false` blocks loopback hosts with
// ER-TLS-21764 across every loopback shape.
func TestCF_T3_OpenUrl_LocalhostBlocked_Default(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"hostname", "http://localhost/admin"},
		{"hostname-with-port", "http://localhost:8080/"},
		{"ipv4-loopback", "http://127.0.0.1/"},
		{"ipv6-loopback", "http://[::1]/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertOpenUrlRejected(t,
				OpenUrlSpec{Url: tc.url, Origin: OriginManual},
				errtrace.ErrToolsOpenUrlLocalhost,
			)
		})
	}
}

// TestCF_T3_OpenUrl_LocalhostAllowed_WhenOptedIn proves the inverse:
// flipping `AllowLocalhostUrls = true` lets loopback URLs pass
// validation. Together with the previous test this locks the boolean
// is the single switch — not a hard-coded behaviour.
func TestCF_T3_OpenUrl_LocalhostAllowed_WhenOptedIn(t *testing.T) {
	b := &fakeBrowser{}
	s := newFakeStore()
	cfg := DefaultToolsConfig()
	cfg.AllowLocalhostUrls = true
	r := NewTools(b, s, cfg)
	if r.HasError() {
		t.Fatalf("NewTools: %v", r.Error())
	}
	tools := r.Value()
	res := tools.OpenUrl(context.Background(), OpenUrlSpec{
		Url: "http://localhost:8080/healthz", Origin: OriginManual,
	})
	if res.HasError() {
		t.Fatalf("expected localhost to pass when allowed; got %v", res.Error())
	}
	if len(b.opened) != 1 {
		t.Fatalf("expected 1 launch, got %d", len(b.opened))
	}
}

// TestCF_R1_Rules_OpenUrl_HonorsSchemeAllowlist proves the Rule path
// (Origin=rule) is identical to the Tools path: a `javascript:` URL
// dispatched as a rule action is rejected with the same code as a
// Tools-UI launch. The "Rule" caller is simulated by setting
// `Origin = OriginRule` — the actual rules.Engine wraps OpenUrl with
// no per-caller bypass, so this proves the chokepoint contract.
func TestCF_R1_Rules_OpenUrl_HonorsSchemeAllowlist(t *testing.T) {
	assertOpenUrlRejected(t,
		OpenUrlSpec{Url: "javascript:alert('rule-action')", Origin: OriginRule},
		errtrace.ErrToolsOpenUrlScheme,
	)
}

// assertOpenUrlRejected is the shared assertion: a fresh Tools with
// default config must reject `spec` with `wantCode` and never call the
// browser. Using fresh state per-call keeps each parametrised case
// independent (no dedup interference).
func assertOpenUrlRejected(t *testing.T, spec OpenUrlSpec, wantCode errtrace.Code) {
	t.Helper()
	b := &fakeBrowser{}
	s := newFakeStore()
	tools := mustTools(t, b, s)
	res := tools.OpenUrl(context.Background(), spec)
	if !res.HasError() {
		t.Fatalf("expected error for %q (origin=%s)", spec.Url, spec.Origin)
	}
	var coded *errtrace.Coded
	if !errors.As(res.Error(), &coded) || coded.Code != wantCode {
		t.Fatalf("expected code %s, got %v", wantCode, res.Error())
	}
	if len(b.opened) != 0 {
		t.Fatalf("browser must NOT launch on validation failure (origin=%s, url=%q)",
			spec.Origin, spec.Url)
	}
}
