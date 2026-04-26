// rules_reorder_test.go — coverage matrix for `(*RulesService).Reorder`.
//
// Coverage matrix:
//   - HappyPath_SwapsOrderPreservingFields
//   - HappyPath_IdentityReorderStillPersists  (allowed; not a no-op error)
//   - HappyPath_ReverseOrder                  (full N-element permutation)
//   - HappyPath_EmptyOnEmptyConfig            (degenerate but valid)
//   - CountMismatch_TooFew_Rejected            (ErrRuleReorderMismatch)
//   - CountMismatch_TooMany_Rejected           (ErrRuleReorderMismatch)
//   - DuplicateInInput_Rejected                (ErrRuleReorderMismatch + duplicateName ctx)
//   - MissingName_Rejected                     (ErrRuleReorderMismatch + missingName ctx)
//   - EmptyInputOnNonEmptyConfig_Rejected      (count mismatch path)
//   - LoadError_Wrapped                        (ErrConfigOpen)
//   - SaveError_Wrapped                        (ErrConfigEncode)
//
// Reuses memCfg / newSvc fixtures from rules_service_test.go.
package core

import (
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// makeReorderCfg: 3-rule config with distinguishing fields so we can
// verify that Reorder doesn't drop them when rebuilding the slice.
func makeReorderCfg() *config.Config {
	return &config.Config{Rules: []config.Rule{
		{Name: "alpha", Enabled: true, UrlRegex: "^https://a", FromRegex: "a@"},
		{Name: "beta", Enabled: false, UrlRegex: "^https://b", SubjectRegex: "[B]"},
		{Name: "gamma", Enabled: true, UrlRegex: "^https://c", BodyRegex: "g-body"},
	}}
}

func TestReorder_HappyPath_SwapsOrderPreservingFields(t *testing.T) {
	cfg := makeReorderCfg()
	load, save, path, saves := memCfg(cfg)
	svc := newSvc(t, load, save, path)

	// Move gamma to front: [gamma, alpha, beta].
	res := svc.Reorder([]string{"gamma", "alpha", "beta"})
	if res.HasError() {
		t.Fatalf("Reorder: %v", res.Error())
	}
	if *saves != 1 {
		t.Errorf("saves = %d, want 1", *saves)
	}
	got := svc.List().Value()
	wantNames := []string{"gamma", "alpha", "beta"}
	for i, n := range wantNames {
		if got[i].Name != n {
			t.Errorf("rules[%d].Name = %q, want %q", i, got[i].Name, n)
		}
	}
	// Spot-check a non-name field came along for the ride.
	if got[0].BodyRegex != "g-body" {
		t.Errorf("gamma.BodyRegex lost: got %q", got[0].BodyRegex)
	}
	if got[2].SubjectRegex != "[B]" {
		t.Errorf("beta.SubjectRegex lost: got %q", got[2].SubjectRegex)
	}
}

func TestReorder_HappyPath_IdentityStillPersists(t *testing.T) {
	cfg := makeReorderCfg()
	load, save, path, saves := memCfg(cfg)
	svc := newSvc(t, load, save, path)

	res := svc.Reorder([]string{"alpha", "beta", "gamma"})
	if res.HasError() {
		t.Fatalf("identity Reorder rejected: %v", res.Error())
	}
	if *saves != 1 {
		t.Errorf("identity Reorder saves = %d, want 1 (DnD-drop-in-place is legitimate)", *saves)
	}
}

func TestReorder_HappyPath_ReverseOrder(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	res := svc.Reorder([]string{"gamma", "beta", "alpha"})
	if res.HasError() {
		t.Fatalf("reverse Reorder: %v", res.Error())
	}
	got := svc.List().Value()
	if got[0].Name != "gamma" || got[1].Name != "beta" || got[2].Name != "alpha" {
		t.Errorf("reverse failed: %+v", []string{got[0].Name, got[1].Name, got[2].Name})
	}
}

func TestReorder_EmptyOnEmptyConfig_OK(t *testing.T) {
	svc := newSvcMem(t, &config.Config{})
	res := svc.Reorder([]string{})
	if res.HasError() {
		t.Fatalf("empty-on-empty Reorder: %v", res.Error())
	}
}

func TestReorder_CountMismatch_TooFew(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	res := svc.Reorder([]string{"alpha", "beta"}) // missing gamma
	if !res.HasError() {
		t.Fatal("expected ErrRuleReorderMismatch, got nil")
	}
	mustCoded(t, res.Error(), errtrace.ErrRuleReorderMismatch)
}

func TestReorder_CountMismatch_TooMany(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	res := svc.Reorder([]string{"alpha", "beta", "gamma", "delta"})
	if !res.HasError() {
		t.Fatal("expected ErrRuleReorderMismatch, got nil")
	}
	mustCoded(t, res.Error(), errtrace.ErrRuleReorderMismatch)
}

func TestReorder_DuplicateInInput(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	res := svc.Reorder([]string{"alpha", "alpha", "beta"}) // dup alpha, missing gamma
	if !res.HasError() {
		t.Fatal("expected ErrRuleReorderMismatch, got nil")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrRuleReorderMismatch)
	if !findCtxKeyRules(c, "duplicateName", "alpha") {
		t.Errorf("missing duplicateName=alpha in context: %+v", c.Context)
	}
}

func TestReorder_MissingName(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	// Same count, but "delta" is unknown and "gamma" is omitted.
	res := svc.Reorder([]string{"alpha", "beta", "delta"})
	if !res.HasError() {
		t.Fatal("expected ErrRuleReorderMismatch, got nil")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrRuleReorderMismatch)
	if !findCtxKeyRules(c, "missingName", "delta") {
		t.Errorf("missing missingName=delta in context: %+v", c.Context)
	}
}

func TestReorder_EmptyInputOnNonEmptyConfig_Rejected(t *testing.T) {
	svc := newSvcMem(t, makeReorderCfg())
	res := svc.Reorder(nil)
	if !res.HasError() {
		t.Fatal("expected ErrRuleReorderMismatch on nil-input vs. 3-rule config")
	}
	mustCoded(t, res.Error(), errtrace.ErrRuleReorderMismatch)
}

func TestReorder_LoadError_Wrapped(t *testing.T) {
	loadErr := errors.New("boom-load")
	load := func() (*config.Config, error) { return nil, loadErr }
	save := func(*config.Config) error { return nil }
	path := func() (string, error) { return "/fake", nil }
	svc := newSvc(t, load, save, path)

	res := svc.Reorder([]string{"alpha"})
	if !res.HasError() {
		t.Fatal("expected ErrConfigOpen, got nil")
	}
	mustCoded(t, res.Error(), errtrace.ErrConfigOpen)
	if !errors.Is(res.Error(), loadErr) {
		t.Errorf("expected wrapped loadErr, got %v", res.Error())
	}
}

func TestReorder_SaveError_Wrapped(t *testing.T) {
	saveErr := errors.New("boom-save")
	cfg := makeReorderCfg()
	load := func() (*config.Config, error) {
		cp := *cfg
		cp.Rules = append([]config.Rule(nil), cfg.Rules...)
		return &cp, nil
	}
	save := func(*config.Config) error { return saveErr }
	path := func() (string, error) { return "/fake", nil }
	svc := newSvc(t, load, save, path)

	res := svc.Reorder([]string{"alpha", "beta", "gamma"}) // valid identity
	if !res.HasError() {
		t.Fatal("expected ErrConfigEncode, got nil")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrConfigEncode)
	if !errors.Is(res.Error(), saveErr) {
		t.Errorf("expected wrapped saveErr, got %v", res.Error())
	}
	if !findCtxKeyRules(c, "firstName", "alpha") {
		t.Errorf("missing firstName=alpha in context: %+v", c.Context)
	}
}
