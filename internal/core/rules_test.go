package core

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func TestAddRule_RequiresFields(t *testing.T) {
	withIsolatedConfig(t, func() {
		if r := AddRule(RuleInput{}); !r.HasError() {
			t.Fatal("expected error for empty input")
		}
		if r := AddRule(RuleInput{Name: "x"}); !r.HasError() {
			t.Fatal("expected error for missing urlRegex")
		}
		if r := AddRule(RuleInput{UrlRegex: "https?://.+"}); !r.HasError() {
			t.Fatal("expected error for missing name")
		}
	})
}

func TestAddRule_RejectsInvalidRegex(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := AddRule(RuleInput{
			Name:     "bad",
			UrlRegex: "https?://[",
		})
		if !r.HasError() {
			t.Fatal("expected invalid regex error")
		}
		// Sanity check: nothing persisted.
		rs := ListRules()
		if rs.HasError() {
			t.Fatalf("ListRules: %v", rs.Error())
		}
		if len(rs.Value()) != 0 {
			t.Fatalf("invalid rule should not persist, got %d rules", len(rs.Value()))
		}
	})
}

func TestAddRule_PersistsAndUpserts(t *testing.T) {
	withIsolatedConfig(t, func() {
		res := AddRule(RuleInput{
			Name:     "open-all",
			UrlRegex: `https?://\S+`,
			Enabled:  true,
		})
		if res.HasError() {
			t.Fatalf("AddRule: %v", res.Error())
		}
		if res.Value().Replaced {
			t.Fatal("first add should not be a replace")
		}
		// Upsert with new pattern + disabled.
		res2 := AddRule(RuleInput{
			Name:     "open-all",
			UrlRegex: `https://example\.com/\S+`,
			Enabled:  false,
		})
		if res2.HasError() {
			t.Fatalf("AddRule upsert: %v", res2.Error())
		}
		if !res2.Value().Replaced {
			t.Fatal("second add should report Replaced=true")
		}
		got := GetRule("open-all")
		if got.HasError() {
			t.Fatalf("GetRule: %v", got.Error())
		}
		if got.Value().UrlRegex != `https://example\.com/\S+` || got.Value().Enabled {
			t.Fatalf("upsert did not apply: %+v", got.Value())
		}
	})
}

func TestSetRuleEnabled_TogglesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		if r := SetRuleEnabled("missing", true); !r.HasError() {
			t.Fatal("expected error for unknown rule")
		}
		_ = AddRule(RuleInput{Name: "r1", UrlRegex: "x", Enabled: false})
		if r := SetRuleEnabled("r1", true); r.HasError() {
			t.Fatalf("enable: %v", r.Error())
		}
		got := GetRule("r1")
		if got.HasError() || !got.Value().Enabled {
			t.Fatal("rule should be enabled")
		}
	})
}

func TestRemoveRule_DeletesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		if r := RemoveRule("nope"); !r.HasError() {
			t.Fatal("expected error for missing rule")
		}
		_ = AddRule(RuleInput{Name: "r1", UrlRegex: "x", Enabled: true})
		_ = AddRule(RuleInput{Name: "r2", UrlRegex: "y", Enabled: true})
		if r := RemoveRule("r1"); r.HasError() {
			t.Fatalf("remove: %v", r.Error())
		}
		rs := ListRules()
		if rs.HasError() {
			t.Fatalf("list: %v", rs.Error())
		}
		v := rs.Value()
		if len(v) != 1 || v[0].Name != "r2" {
			t.Fatalf("unexpected rules after remove: %+v", v)
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
