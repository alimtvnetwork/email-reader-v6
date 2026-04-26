// rules_test.go — Phase 2.8: routed through *RulesService instead of
// the deprecated package-level wrappers (deleted in P2.8b). Each test
// constructs a default-injected service via NewDefaultRulesService and
// drives the same scenarios that the old wrappers covered.
package core

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// newRulesSvc builds a default-injected *RulesService backed by the
// real config package, used inside withIsolatedConfig to point at a
// per-test temp dir.
func newRulesSvc(t *testing.T) *RulesService {
	t.Helper()
	res := NewDefaultRulesService()
	if res.HasError() {
		t.Fatalf("NewDefaultRulesService: %v", res.Error())
	}
	return res.Value()
}

func TestAddRule_RequiresFields(t *testing.T) {
	withIsolatedConfig(t, func() {
		svc := newRulesSvc(t)
		if r := svc.Add(RuleInput{}); !r.HasError() {
			t.Fatal("expected error for empty input")
		}
		if r := svc.Add(RuleInput{Name: "x"}); !r.HasError() {
			t.Fatal("expected error for missing urlRegex")
		}
		if r := svc.Add(RuleInput{UrlRegex: "https?://.+"}); !r.HasError() {
			t.Fatal("expected error for missing name")
		}
	})
}

func TestAddRule_RejectsInvalidRegex(t *testing.T) {
	withIsolatedConfig(t, func() {
		svc := newRulesSvc(t)
		r := svc.Add(RuleInput{
			Name:     "bad",
			UrlRegex: "https?://[",
		})
		if !r.HasError() {
			t.Fatal("expected invalid regex error")
		}
		// Sanity check: nothing persisted.
		rs := svc.List()
		if rs.HasError() {
			t.Fatalf("List: %v", rs.Error())
		}
		if len(rs.Value()) != 0 {
			t.Fatalf("invalid rule should not persist, got %d rules", len(rs.Value()))
		}
	})
}

func TestAddRule_PersistsAndUpserts(t *testing.T) {
	withIsolatedConfig(t, func() {
		svc := newRulesSvc(t)
		res := svc.Add(RuleInput{
			Name:     "open-all",
			UrlRegex: `https?://\S+`,
			Enabled:  true,
		})
		if res.HasError() {
			t.Fatalf("Add: %v", res.Error())
		}
		if res.Value().Replaced {
			t.Fatal("first add should not be a replace")
		}
		// Upsert with new pattern + disabled.
		res2 := svc.Add(RuleInput{
			Name:     "open-all",
			UrlRegex: `https://example\.com/\S+`,
			Enabled:  false,
		})
		if res2.HasError() {
			t.Fatalf("Add upsert: %v", res2.Error())
		}
		if !res2.Value().Replaced {
			t.Fatal("second add should report Replaced=true")
		}
		got := svc.Get("open-all")
		if got.HasError() {
			t.Fatalf("Get: %v", got.Error())
		}
		if got.Value().UrlRegex != `https://example\.com/\S+` || got.Value().Enabled {
			t.Fatalf("upsert did not apply: %+v", got.Value())
		}
	})
}

func TestSetRuleEnabled_TogglesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		svc := newRulesSvc(t)
		if r := svc.SetEnabled("missing", true); !r.HasError() {
			t.Fatal("expected error for unknown rule")
		}
		_ = svc.Add(RuleInput{Name: "r1", UrlRegex: "x", Enabled: false})
		if r := svc.SetEnabled("r1", true); r.HasError() {
			t.Fatalf("enable: %v", r.Error())
		}
		got := svc.Get("r1")
		if got.HasError() || !got.Value().Enabled {
			t.Fatal("rule should be enabled")
		}
	})
}

func TestRemoveRule_DeletesAndErrors(t *testing.T) {
	withIsolatedConfig(t, func() {
		svc := newRulesSvc(t)
		if r := svc.Remove("nope"); !r.HasError() {
			t.Fatal("expected error for missing rule")
		}
		_ = svc.Add(RuleInput{Name: "r1", UrlRegex: "x", Enabled: true})
		_ = svc.Add(RuleInput{Name: "r2", UrlRegex: "y", Enabled: true})
		if r := svc.Remove("r1"); r.HasError() {
			t.Fatalf("remove: %v", r.Error())
		}
		rs := svc.List()
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
