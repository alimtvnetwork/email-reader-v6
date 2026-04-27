// edge_cases_test.go — Slice #186 / `.lovable/suggestions.md` rules-edge-cases item.
//
// Closes the 4 edge cases the legacy suggestion enumerated:
//
//  1. Invalid regex → no panic; bad rule skipped; valid rules still load.
//  2. URLs with `&` query parameters → captured whole, not split on `&`.
//  3. Multi-URL email → all distinct URLs returned (open-all semantics, NOT
//     first-only).
//  4. Case-sensitivity via inline `(?i)` flag → `(?i)` matches mixed-case;
//     absence of `(?i)` is case-sensitive (regression guard).
//
// Companion to the existing 3 tests in `rules_test.go` (matches+dedupe,
// no-from-match, empty-pattern-matches-any).

package rules

import (
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/mailclient"
)

// (1) Invalid regex must not crash New(); the bad rule is skipped, and any
// other valid rule in the same call still loads. This protects against a
// single typo'd `urlRegex` taking the whole engine down at process start.
func TestEdgeCase_InvalidRegex_NoPanic_ValidRuleStillLoads(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New() panicked on invalid regex: %v", r)
		}
	}()

	rs := []config.Rule{
		{Name: "bad-from", Enabled: true, FromRegex: "[unterminated", UrlRegex: `https?://\S+`},
		{Name: "bad-url", Enabled: true, UrlRegex: "(unbalanced"},
		{Name: "bad-subject", Enabled: true, SubjectRegex: "*invalid", UrlRegex: `https?://\S+`},
		{Name: "bad-body", Enabled: true, BodyRegex: "(?P<", UrlRegex: `https?://\S+`},
		{Name: "good", Enabled: true, UrlRegex: `https?://\S+`},
	}

	e, err := New(rs)
	if err == nil {
		t.Fatal("expected non-nil error from invalid regex compilation")
	}
	if e == nil {
		t.Fatal("New() returned nil engine despite at least one valid rule")
	}
	// Per `rules.go` line 60-62, an invalid regex causes that one rule to be
	// skipped (firstErr captured, `continue` jumps past the append). So the
	// only fully-loaded rule is "good".
	if got := e.RuleCount(); got != 1 {
		t.Fatalf("expected 1 loaded rule (the only one with no bad regex), got %d", got)
	}

	// Sanity: the "good" rule still evaluates without crashing.
	m := &mailclient.Message{BodyText: "click https://x.com/y"}
	matches := e.Evaluate(m)
	if len(matches) != 1 || matches[0].RuleName != "good" {
		t.Fatalf("good rule did not survive invalid-regex sibling rules: %+v", matches)
	}
}

// (2) URLs with `&` query parameters must be captured whole. The naïve
// `[^&]+` mistake would truncate at the first `&`; the standard `\S+`
// pattern in the existing tests should handle this — this is a regression
// guard against future "tighten the URL regex" edits that drop ampersands.
func TestEdgeCase_UrlWithAmpersandQueryParams_NotSplit(t *testing.T) {
	e, err := New([]config.Rule{{
		Name: "magic", Enabled: true,
		UrlRegex: `https?://\S+`,
	}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	full := "https://example.com/auth?token=ABC&user=42&ts=1700000000&sig=deadbeef"
	m := &mailclient.Message{BodyText: "Sign in: " + full + " — thanks."}

	got := e.Evaluate(m)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 URL match, got %d: %+v", len(got), got)
	}
	if got[0].Url != full {
		t.Fatalf("URL was truncated/split on `&`:\n  want %q\n  got  %q", full, got[0].Url)
	}
	// Defensive: every query param survived.
	for _, want := range []string{"token=ABC", "user=42", "ts=1700000000", "sig=deadbeef"} {
		if !strings.Contains(got[0].Url, want) {
			t.Errorf("captured URL missing expected query fragment %q: %s", want, got[0].Url)
		}
	}
}

// (3) Multi-URL email: all distinct URLs in the body MUST be returned (each
// becomes its own Match), not just the first. This pins the "open-all"
// semantic — a future "first-only" optimisation would silently drop URLs
// 2..N and break magic-link flows that include both the auth URL and a
// fallback URL.
func TestEdgeCase_MultiUrlEmail_AllDistinctUrlsReturned(t *testing.T) {
	e, err := New([]config.Rule{{
		Name: "openall", Enabled: true,
		UrlRegex: `https?://\S+`,
	}})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m := &mailclient.Message{BodyText: "Primary: https://a.example.com/1\n" +
		"Backup:  https://b.example.com/2\n" +
		"Status:  https://c.example.com/3"}

	got := e.Evaluate(m)
	if len(got) != 3 {
		t.Fatalf("expected 3 distinct URL matches (open-all), got %d: %+v", len(got), got)
	}

	wantUrls := map[string]bool{
		"https://a.example.com/1": false,
		"https://b.example.com/2": false,
		"https://c.example.com/3": false,
	}
	for _, g := range got {
		if g.RuleName != "openall" {
			t.Errorf("unexpected rule: %s", g.RuleName)
		}
		if _, ok := wantUrls[g.Url]; !ok {
			t.Errorf("unexpected URL match: %s", g.Url)
		}
		wantUrls[g.Url] = true
	}
	for url, seen := range wantUrls {
		if !seen {
			t.Errorf("expected URL %s was not returned (first-only regression?)", url)
		}
	}
}

// (4) Case-sensitivity is controlled via Go's inline `(?i)` regex flag (no
// separate config field). Pin both halves so a future config-schema rewrite
// can't silently alter behaviour:
//   - `(?i)Sign` MUST match `SIGN IN` (case-insensitive when flagged).
//   - `Sign`     MUST NOT match `SIGN IN` (case-sensitive when unflagged).
func TestEdgeCase_CaseSensitivity_InlineFlagOnly(t *testing.T) {
	insensitive, err := New([]config.Rule{{
		Name: "ci", Enabled: true,
		SubjectRegex: "(?i)Sign in",
		UrlRegex:     `https?://\S+`,
	}})
	if err != nil {
		t.Fatalf("new (insensitive): %v", err)
	}
	sensitive, err := New([]config.Rule{{
		Name: "cs", Enabled: true,
		SubjectRegex: "Sign in",
		UrlRegex:     `https?://\S+`,
	}})
	if err != nil {
		t.Fatalf("new (sensitive): %v", err)
	}

	mixedCase := &mailclient.Message{
		From: "x@y.z", Subject: "SIGN IN to continue",
		BodyText: "https://example.com/a",
	}

	if got := insensitive.Evaluate(mixedCase); len(got) != 1 {
		t.Errorf("(?i) should match SIGN IN against \"Sign in\"; got %d matches: %+v", len(got), got)
	}
	if got := sensitive.Evaluate(mixedCase); len(got) != 0 {
		t.Errorf("unflagged regex should NOT match SIGN IN against \"Sign in\"; got %d matches: %+v", len(got), got)
	}

	// Symmetric: lowercase subject MUST match the unflagged "Sign in" only
	// when the case actually agrees. With "sign in" lowercase, neither
	// pattern as written matches (sensitive = no, insensitive = yes —
	// covered above). Pin the literal-match case to round out coverage.
	exactCase := &mailclient.Message{
		From: "x@y.z", Subject: "Sign in please",
		BodyText: "https://example.com/a",
	}
	if got := sensitive.Evaluate(exactCase); len(got) != 1 {
		t.Errorf("case-sensitive pattern should match identical case; got %d: %+v", len(got), got)
	}
}
