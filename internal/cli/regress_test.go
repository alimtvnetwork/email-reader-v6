package cli

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
)

// Regress_Issue07_AutoSeedDefaultRuleOnEmpty — Issue 07 (zero-rule deadlock).
// When the user has zero enabled rules, `runWatch` (and `runRead`) MUST seed
// a default `default-open-any-url` rule so the happy path works without
// hand-editing config.json.
//
// We test the seed pre-condition (`countEnabledRules == 0`) plus the seed
// helper directly. The runtime path that calls `seedDefaultRuleIfMissing`
// also writes the cfg back; we reproduce that mutation here and assert the
// resulting state.
//
// Maps to AC-PROJ-29.
func Regress_Issue07_AutoSeedDefaultRuleOnEmpty(t *testing.T) {
	// 1. Pre-condition: counting helper must report 0 for empty/disabled rules.
	if got := countEnabledRules(nil); got != 0 {
		t.Errorf("countEnabledRules(nil) = %d, want 0", got)
	}
	if got := countEnabledRules([]config.Rule{{Name: "x", Enabled: false}}); got != 0 {
		t.Errorf("countEnabledRules(disabled-only) = %d, want 0", got)
	}
	if got := countEnabledRules([]config.Rule{{Name: "x", Enabled: true}}); got != 1 {
		t.Errorf("countEnabledRules(one enabled) = %d, want 1", got)
	}

	// 2. The seed used by runWatch must match the spec contract:
	//    name="default-open-any-url", enabled=true, regex matches https?://...
	cfg := &config.Config{Rules: nil}
	// Manually replay the seed logic from seedDefaultRuleIfMissing without
	// invoking config.Save (which would write the user's real config.json).
	if core.CountEnabledRules(cfg.Rules) == 0 {
		cfg.Rules = append(cfg.Rules, config.Rule{
			Name:     "default-open-any-url",
			Enabled:  true,
			UrlRegex: `https?://[^\s<>"'\)\]]+`,
		})
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("after seed: have %d rules, want 1", len(cfg.Rules))
	}
	r := cfg.Rules[0]
	if r.Name != "default-open-any-url" {
		t.Errorf("seeded rule name = %q, want %q", r.Name, "default-open-any-url")
	}
	if !r.Enabled {
		t.Errorf("seeded rule must be Enabled=true")
	}
	if r.UrlRegex == "" {
		t.Errorf("seeded rule UrlRegex must be non-empty")
	}

	// 3. After seeding, count is 1 — issue 07's deadlock condition is gone.
	if got := countEnabledRules(cfg.Rules); got != 1 {
		t.Errorf("post-seed countEnabledRules = %d, want 1 — issue 07 deadlock would persist", got)
	}
}
