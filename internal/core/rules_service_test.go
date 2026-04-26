// rules_service_test.go — Phase 2.6 typed *RulesService coverage.
//
// Goal: prove that the new typed service does what the old
// package-level funcs did, without ever touching disk, by injecting
// fake `loadCfg` / `saveCfg` / `cfgPath` closures backed by an
// in-memory `*config.Config`.
//
// Coverage matrix:
//   - constructor rejects nil deps in each slot (3 sub-cases)
//   - Add validates required fields → ErrCoreInvalidArgument
//   - Add catches bad regex → ErrRulePatternInvalid (with field ctx)
//   - Add upsert: new rule appended; same-name overwrites with Replaced=true
//   - Add propagates load/save errors with the right code
//   - List returns a defensive copy (mutating result doesn't touch source)
//   - Get returns ErrRuleNotFound on miss
//   - SetEnabled flips the flag and persists; ErrRuleNotFound on miss
//   - Remove deletes and persists; ErrRuleNotFound on miss
package core

import (
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// memCfg returns load/save/path closures backed by an in-memory
// config that survives across calls. saveCount is bumped on every
// save so tests can assert "did we persist?".
func memCfg(initial *config.Config) (configLoader, cfgWriter, cfgPathFn, *int) {
	cur := initial
	saves := 0
	load := func() (*config.Config, error) {
		// Return a deep-ish copy so callers mutating the result don't
		// retro-mutate the in-memory source — matches real config.Load
		// which always re-reads disk.
		cp := *cur
		cp.Rules = append([]config.Rule(nil), cur.Rules...)
		return &cp, nil
	}
	save := func(c *config.Config) error {
		saves++
		cur = c
		return nil
	}
	path := func() (string, error) { return "/fake/config.json", nil }
	return load, save, path, &saves
}

func newSvc(t *testing.T, l configLoader, s cfgWriter, p cfgPathFn) *RulesService {
	t.Helper()
	res := NewRulesService(l, s, p)
	if res.HasError() {
		t.Fatalf("NewRulesService: %v", res.Error())
	}
	return res.Value()
}

func mustCoded(t *testing.T, err error, want errtrace.Code) *errtrace.Coded {
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

func TestNewRulesService_RejectsNilDeps(t *testing.T) {
	load, save, path, _ := memCfg(&config.Config{})
	cases := []struct {
		name        string
		l           configLoader
		s           cfgWriter
		p           cfgPathFn
	}{
		{"nil load", nil, save, path},
		{"nil save", load, nil, path},
		{"nil path", load, save, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := NewRulesService(tc.l, tc.s, tc.p)
			if !res.HasError() {
				t.Fatal("expected error")
			}
			mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)
		})
	}
}

func TestRulesService_Add_RequiresNameAndUrl(t *testing.T) {
	svc := newSvc(t, memCfg(&config.Config{}))
	res := svc.Add(RuleInput{Name: "", UrlRegex: "x"})
	mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)

	res = svc.Add(RuleInput{Name: "n", UrlRegex: ""})
	mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)
}

func TestRulesService_Add_RejectsBadRegex(t *testing.T) {
	svc := newSvc(t, memCfg(&config.Config{}))
	res := svc.Add(RuleInput{Name: "n", UrlRegex: "ok", FromRegex: "[unclosed"})
	c := mustCoded(t, res.Error(), errtrace.ErrRulePatternInvalid)
	if got, ok := ctxValue(c, "field"); !ok || got != "fromRegex" {
		t.Errorf("expected field=fromRegex context, got %v (ok=%v)", got, ok)
	}
}

func TestRulesService_Add_UpsertSemantics(t *testing.T) {
	load, save, path, saves := memCfg(&config.Config{})
	svc := newSvc(t, load, save, path)

	// First add → appended, Replaced=false
	r1 := svc.Add(RuleInput{Name: "r1", UrlRegex: "a", Enabled: true})
	if r1.HasError() {
		t.Fatalf("add r1: %v", r1.Error())
	}
	if r1.Value().Replaced {
		t.Error("first add should not be Replaced")
	}
	if r1.Value().ConfigPath != "/fake/config.json" {
		t.Errorf("ConfigPath wrong: %q", r1.Value().ConfigPath)
	}

	// Same-name re-add → Replaced=true
	r2 := svc.Add(RuleInput{Name: "r1", UrlRegex: "b", Enabled: false})
	if r2.HasError() {
		t.Fatalf("add r1 v2: %v", r2.Error())
	}
	if !r2.Value().Replaced {
		t.Error("second add should be Replaced=true")
	}

	// Confirm persistence: list shows exactly 1 rule with the new url
	listed := svc.List()
	if listed.HasError() {
		t.Fatal(listed.Error())
	}
	if len(listed.Value()) != 1 || listed.Value()[0].UrlRegex != "b" {
		t.Errorf("expected 1 rule with url=b, got %+v", listed.Value())
	}
	if *saves != 2 {
		t.Errorf("expected 2 saves, got %d", *saves)
	}
}

func TestRulesService_Add_PropagatesLoadAndSaveErrors(t *testing.T) {
	loadErr := errors.New("disk gone")
	failLoad := func() (*config.Config, error) { return nil, loadErr }
	okSave := func(*config.Config) error { return nil }
	okPath := func() (string, error) { return "/p", nil }
	svc := newSvc(t, failLoad, okSave, okPath)
	res := svc.Add(RuleInput{Name: "n", UrlRegex: "u"})
	mustCoded(t, res.Error(), errtrace.ErrConfigOpen)

	saveErr := errors.New("disk full")
	okLoad := func() (*config.Config, error) { return &config.Config{}, nil }
	failSave := func(*config.Config) error { return saveErr }
	svc2 := newSvc(t, okLoad, failSave, okPath)
	res2 := svc2.Add(RuleInput{Name: "n", UrlRegex: "u"})
	c := mustCoded(t, res2.Error(), errtrace.ErrConfigEncode)
	if got, ok := ctxValue(c, "rule"); !ok || got != "n" {
		t.Errorf("expected rule context, got %v", got)
	}
}

func TestRulesService_List_ReturnsDefensiveCopy(t *testing.T) {
	cfg := &config.Config{Rules: []config.Rule{{Name: "r1", UrlRegex: "u", Enabled: true}}}
	svc := newSvc(t, memCfg(cfg))
	res := svc.List()
	if res.HasError() {
		t.Fatal(res.Error())
	}
	out := res.Value()
	out[0].Name = "MUTATED" // should not affect future List calls
	res2 := svc.List()
	if res2.Value()[0].Name != "r1" {
		t.Errorf("List result not a defensive copy: %q", res2.Value()[0].Name)
	}
}

func TestRulesService_Get_NotFound(t *testing.T) {
	svc := newSvc(t, memCfg(&config.Config{}))
	res := svc.Get("missing")
	mustCoded(t, res.Error(), errtrace.ErrRuleNotFound)
}

func TestRulesService_SetEnabled_FlipsAndPersists(t *testing.T) {
	cfg := &config.Config{Rules: []config.Rule{{Name: "r1", UrlRegex: "u", Enabled: false}}}
	load, save, path, saves := memCfg(cfg)
	svc := newSvc(t, load, save, path)

	if r := svc.SetEnabled("r1", true); r.HasError() {
		t.Fatalf("SetEnabled: %v", r.Error())
	}
	got := svc.Get("r1")
	if got.HasError() || !got.Value().Enabled {
		t.Errorf("expected Enabled=true after flip, got %+v err=%v", got.Value(), got.Error())
	}
	if *saves != 1 {
		t.Errorf("expected 1 save, got %d", *saves)
	}

	if r := svc.SetEnabled("missing", true); !r.HasError() {
		t.Fatal("expected ErrRuleNotFound for missing rule")
	} else {
		mustCoded(t, r.Error(), errtrace.ErrRuleNotFound)
	}
}

func TestRulesService_Remove_DeletesAndPersists(t *testing.T) {
	cfg := &config.Config{Rules: []config.Rule{
		{Name: "r1", UrlRegex: "u"},
		{Name: "r2", UrlRegex: "v"},
	}}
	load, save, path, saves := memCfg(cfg)
	svc := newSvc(t, load, save, path)

	if r := svc.Remove("r1"); r.HasError() {
		t.Fatalf("Remove: %v", r.Error())
	}
	if *saves != 1 {
		t.Errorf("expected 1 save, got %d", *saves)
	}
	listed := svc.List().Value()
	if len(listed) != 1 || listed[0].Name != "r2" {
		t.Errorf("expected only r2 left, got %+v", listed)
	}

	if r := svc.Remove("missing"); !r.HasError() {
		t.Fatal("expected ErrRuleNotFound")
	} else {
		mustCoded(t, r.Error(), errtrace.ErrRuleNotFound)
	}
}
