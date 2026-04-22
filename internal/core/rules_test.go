package core

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func TestAddRule_RequiresFields(t *testing.T) {
	withIsolatedConfig(t, func() {
		if _, err := AddRule(RuleInput{}); err == nil {
			t.Fatal("expected error for empty input")
		}
		if _, err := AddRule(RuleInput{Name: "x"}); err == nil {
			t.Fatal("expected error for missing urlRegex")
		}
		if _, err := AddRule(RuleInput{UrlRegex: "https?://.+"}); err == nil {
			t.Fatal("expected error for missing name")
		}
	})
}

func TestAddRule_RejectsInvalidRegex(t *testing.T) {
	withIsolatedConfig(t, func() {
		_, err := AddRule(RuleInput{
			Name:     "bad",
			UrlRegex: "https?://[",
		})
		if err == nil {
			t.Fatal("expected invalid regex error")
		}
		// Sanity check: nothing persisted.
		rs, _ := ListRules()
		if len(rs) != 0 {
			t.Fatalf("invalid rule should not persist, got %d rules", len(rs))
		}
	})
}

func TestAddRule_PersistsAndUpserts(t *testing.T) {
	withIsolatedConfig(t, func() {
		res, err := AddRule(RuleInput{
			Name:     "open-all",
			UrlRegex: `https?://\S+`,
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("AddRule: %v", err)
		}
		if res.Replaced {
			t.Fatal("first add should not be a replace")
		}
		// Upsert with new pattern + disabled.
		res2, err := AddRule(RuleInput{
			Name:     "open-all",
			UrlRegex: `https://example\.com/\S+`,
			Enabled:  false,
		})
		if err != nil {
			t.Fatalf("AddRule upsert: %v", err)
		}
		if !res2.Replaced {
			t.Fatal("second add should report Replaced=true")
		}
		got, err := GetRule("open-all")
		if err != nil {
			t.Fatalf("GetRule: %v", err)
		}
		if got.UrlRegex != `https://example\.com/\S+` || got.Enabled {
			t.Fatalf("upsert did not apply: %+v", got)
		}
	})
}

func TestSetRuleEnabled_TogglesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		if err := SetRuleEnabled("missing", true); err == nil {
			t.Fatal("expected error for unknown rule")
		}
		_, _ = AddRule(RuleInput{Name: "r1", UrlRegex: "x", Enabled: false})
		if err := SetRuleEnabled("r1", true); err != nil {
			t.Fatalf("enable: %v", err)
		}
		got, _ := GetRule("r1")
		if !got.Enabled {
			t.Fatal("rule should be enabled")
		}
	})
}

func TestRemoveRule_DeletesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		if err := RemoveRule("nope"); err == nil {
			t.Fatal("expected error for missing rule")
		}
		_, _ = AddRule(RuleInput{Name: "r1", UrlRegex: "x", Enabled: true})
		_, _ = AddRule(RuleInput{Name: "r2", UrlRegex: "y", Enabled: true})
		if err := RemoveRule("r1"); err != nil {
			t.Fatalf("remove: %v", err)
		}
		rs, _ := ListRules()
		if len(rs) != 1 || rs[0].Name != "r2" {
			t.Fatalf("unexpected rules after remove: %+v", rs)
		}
	})
}

func TestCountEnabledRules(t *testing.T) {
	rs := []config.Rule{
		{Name: "a", Enabled: true},
		{Name: "b", Enabled: false},
		{Name: "c", Enabled: true},
	}
	if got := CountEnabledRules(rs); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := CountEnabledRules(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}
