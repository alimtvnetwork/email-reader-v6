// services_tools_test.go — Slice #116c (Phase 6.3) wiring guard.
//
// `BuildServices` is the canonical app-boot entry point. After
// Slice #116c it must return a `*Services` whose `Tools` field is a
// non-nil `ToolsFactory` so the four Tools sub-tabs (OpenUrl, Read,
// Export, Recent opens) can build their `*core.Tools` instances
// without reaching for `config.Load()` themselves. A regression that
// silently dropped the assignment (e.g. a future refactor that
// renamed the field or split `BuildServices`) would not show up in
// the AST guard — the guard only sees the absence of the inline
// `config.Load()` calls in the views, not the absence of the
// upstream wiring. This test closes that loop.
//
// We intentionally do *not* invoke the factory here: under the test
// environment `config.Load()` may legitimately fail (no on-disk
// config), and the contract is "the field is wired", not "the
// factory always returns a usable Tools instance". Sub-tabs already
// surface a degraded-path status when the factory errors.

//go:build !nofyne

package ui

import "testing"

func TestServicesToolsFactoryWired(t *testing.T) {
	s := BuildServices()
	if s == nil {
		t.Fatal("BuildServices returned nil")
	}
	if s.Tools == nil {
		t.Fatal("BuildServices: Tools factory not wired (Slice #116c regression)")
	}
}
