// services_rules_test.go — coverage for the P5.5 thin wrappers.
//
// Coverage matrix:
//   - Rename: nil receiver → ErrRulesServiceUnwired
//   - Rename: nil Rules    → ErrRulesServiceUnwired (with oldName/newName ctx)
//   - Rename: wired        → delegates and returns the service's result
//   - Reorder: nil Rules   → ErrRulesServiceUnwired (with nameCount ctx)
//   - Reorder: wired       → delegates and returns the service's result
//   - lenAsString: 0/1/multi-digit branches all hit
//
// We construct a `*core.RulesService` from in-memory closures to
// avoid disk IO; uses the same shape as core/rules_service_test.go's
// `memCfg` helper but reimplemented locally so the UI test file
// doesn't reach into core's test files.

//go:build !nofyne

package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// memRulesSvc returns a *core.RulesService backed by an in-memory
// config slice — minimal copy of core's memCfg helper so this test
// stays self-contained.
func memRulesSvc(t *testing.T, rules []config.Rule) *core.RulesService {
	t.Helper()
	cur := &config.Config{Rules: append([]config.Rule(nil), rules...)}
	load := func() (*config.Config, error) {
		cp := *cur
		cp.Rules = append([]config.Rule(nil), cur.Rules...)
		return &cp, nil
	}
	save := func(c *config.Config) error { cur = c; return nil }
	path := func() (string, error) { return "/fake/config.json", nil }
	res := core.NewRulesService(load, save, path)
	if res.HasError() {
		t.Fatalf("NewRulesService: %v", res.Error())
	}
	return res.Value()
}

func mustCodedUI(t *testing.T, err error, want errtrace.Code) *errtrace.Coded {
	t.Helper()
	var c *errtrace.Coded
	if !errors.As(err, &c) {
		t.Fatalf("expected *errtrace.Coded, got %T: %v", err, err)
	}
	if c.Code != want {
		t.Fatalf("expected code %v, got %v", want, c.Code)
	}
	return c
}

func ctxValue(c *errtrace.Coded, key string) (string, bool) {
	for _, f := range c.Context {
		if f.Key == key {
			if s, ok := f.Value.(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

func TestServicesRename_NilReceiver_Unwired(t *testing.T) {
	var s *Services
	res := s.Rename("a", "b")
	if !res.HasError() {
		t.Fatal("expected ErrRulesServiceUnwired, got nil")
	}
	mustCodedUI(t, res.Error(), errtrace.ErrRulesServiceUnwired)
}

func TestServicesRename_NilRules_Unwired_WithCtx(t *testing.T) {
	s := &Services{} // Rules == nil
	res := s.Rename("alpha", "beta")
	if !res.HasError() {
		t.Fatal("expected ErrRulesServiceUnwired, got nil")
	}
	c := mustCodedUI(t, res.Error(), errtrace.ErrRulesServiceUnwired)
	if v, _ := ctxValue(c, "oldName"); v != "alpha" {
		t.Errorf("oldName ctx = %q, want %q", v, "alpha")
	}
	if v, _ := ctxValue(c, "newName"); v != "beta" {
		t.Errorf("newName ctx = %q, want %q", v, "beta")
	}
}

func TestServicesRename_Wired_Delegates(t *testing.T) {
	svc := memRulesSvc(t, []config.Rule{
		{Name: "alpha", UrlRegex: "^https://a"},
		{Name: "beta", UrlRegex: "^https://b"},
	})
	s := &Services{Rules: svc}

	res := s.Rename("alpha", "renamed")
	if res.HasError() {
		t.Fatalf("Rename: %v", res.Error())
	}
	listed := svc.List().Value()
	if listed[0].Name != "renamed" {
		t.Errorf("after Rename, rules[0].Name = %q, want %q", listed[0].Name, "renamed")
	}

	// Surface a service-side failure (collision) to prove we don't
	// swallow it on the way through.
	collide := s.Rename("renamed", "beta")
	if !collide.HasError() {
		t.Fatal("expected duplicate error from service")
	}
	mustCodedUI(t, collide.Error(), errtrace.ErrRuleDuplicate)
}

func TestServicesReorder_NilRules_Unwired_WithCountCtx(t *testing.T) {
	s := &Services{}
	res := s.Reorder([]string{"a", "b", "c"})
	if !res.HasError() {
		t.Fatal("expected ErrRulesServiceUnwired, got nil")
	}
	c := mustCodedUI(t, res.Error(), errtrace.ErrRulesServiceUnwired)
	if v, _ := ctxValue(c, "nameCount"); v != "3" {
		t.Errorf("nameCount ctx = %q, want %q", v, "3")
	}
}

func TestServicesReorder_Wired_Delegates(t *testing.T) {
	svc := memRulesSvc(t, []config.Rule{
		{Name: "alpha", UrlRegex: "^https://a"},
		{Name: "beta", UrlRegex: "^https://b"},
		{Name: "gamma", UrlRegex: "^https://c"},
	})
	s := &Services{Rules: svc}

	res := s.Reorder([]string{"gamma", "alpha", "beta"})
	if res.HasError() {
		t.Fatalf("Reorder: %v", res.Error())
	}
	listed := svc.List().Value()
	got := []string{listed[0].Name, listed[1].Name, listed[2].Name}
	want := []string{"gamma", "alpha", "beta"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}

	// Service-side validation failure surfaces unchanged.
	bad := s.Reorder([]string{"gamma", "alpha", "missing"})
	if !bad.HasError() {
		t.Fatal("expected reorder mismatch error")
	}
	mustCodedUI(t, bad.Error(), errtrace.ErrRuleReorderMismatch)
}

func TestLenAsString_AllBranches(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "0"},
		{[]string{}, "0"},
		{[]string{"x"}, "1"},
		{make([]string, 9), "9"},
		{make([]string, 10), "10"},
		{make([]string, 123), "123"},
	}
	for _, tc := range cases {
		if got := lenAsString(tc.in); got != tc.want {
			t.Errorf("lenAsString(len=%d) = %q, want %q", len(tc.in), got, tc.want)
		}
	}
}
