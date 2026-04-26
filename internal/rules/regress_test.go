package rules

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/mailclient"
)

// Regress_Issue05_PerMailRuleTraceAlwaysEmitted — Issue 05 (silent rule
// failure). EvaluateWithTrace MUST return one RuleTrace per enabled rule for
// every message, even when no URLs match. This is what lets the watcher
// always log "rule X: from regex did not match" so a quiet watcher never
// hides why a URL did NOT open.
//
// Maps to AC-PROJ-27.
func Regress_Issue05_PerMailRuleTraceAlwaysEmitted(t *testing.T) {
	rs := []config.Rule{
		{Name: "strict-from", Enabled: true, FromRegex: "boss@", UrlRegex: `https?://\S+`},
		{Name: "strict-subj", Enabled: true, SubjectRegex: "^URGENT:", UrlRegex: `https?://\S+`},
		{Name: "always", Enabled: true, UrlRegex: `https?://\S+`},
		{Name: "disabled", Enabled: false, UrlRegex: `https?://\S+`}, // skipped, expected
	}
	eng, err := New(rs)
	if err != nil {
		t.Fatalf("rules.New: %v", err)
	}

	// Message that will NOT match the first two rules — but the trace must
	// still contain entries for them so the watcher can explain WHY.
	m := &mailclient.Message{
		From:     "alice@example.com",
		Subject:  "hello",
		BodyText: "see https://example.com/link",
	}
	matches, traces := eng.EvaluateWithTrace(m)

	// One trace per ENABLED rule (3), regardless of how many produced URLs.
	if len(traces) != 3 {
		t.Fatalf("got %d traces, want 3 (one per enabled rule, even on no-match) — issue 05 regresses",
			len(traces))
	}

	// Trace MUST cover the strict rules so the user sees the rejection reason.
	seen := map[string]bool{}
	for _, tr := range traces {
		if tr.Reason == "" {
			t.Errorf("rule %q produced an empty Reason — silent-failure regression", tr.RuleName)
		}
		seen[tr.RuleName] = true
	}
	for _, want := range []string{"strict-from", "strict-subj", "always"} {
		if !seen[want] {
			t.Errorf("trace missing for enabled rule %q", want)
		}
	}

	// And the matching rule must have actually produced a Match.
	if len(matches) == 0 {
		t.Errorf("expected ≥1 match from the always-rule, got 0")
	}
}
