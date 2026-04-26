// rules_rename_test.go — coverage matrix for `(*RulesService).Rename`.
//
// Coverage matrix (10 cases):
//   - HappyPath_RenamesPreservingFieldsAndPosition
//   - HappyPath_TrimsBothNames
//   - EmptyOldName_Rejected            (ErrCoreInvalidArgument)
//   - WhitespaceOldName_Rejected       (TrimSpace contract)
//   - EmptyNewName_Rejected            (ErrCoreInvalidArgument)
//   - SameName_Rejected                (ErrRuleRenameNoop — caller bug)
//   - SameName_AfterTrim_Rejected      (whitespace can't sneak past)
//   - OldNameMissing_NotFound          (ErrRuleNotFound + ctx)
//   - NewNameTaken_Duplicate           (ErrRuleDuplicate + both names ctx)
//   - LoadError_Wrapped                (ErrConfigOpen + ctx)
//   - SaveError_Wrapped                (ErrConfigEncode + ctx)
//   - SuccessfulRename_PersistsExactlyOnce
//
// All cases use the in-memory `memCfg` / `newSvcMem` fixtures from
// `rules_service_test.go` — no disk IO.
package core

import (
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// makeRenameCfg builds a 3-rule config so position-preservation
// assertions have something meaningful to check (rule[1] in the
// middle is the rename target across most cases).
func makeRenameCfg() *config.Config {
	return &config.Config{Rules: []config.Rule{
		{Name: "first", Enabled: true, UrlRegex: "^https://a"},
		{Name: "middle", Enabled: false, UrlRegex: "^https://b",
			FromRegex: "alice@", SubjectRegex: "[urgent]", BodyRegex: "verify"},
		{Name: "last", Enabled: true, UrlRegex: "^https://c"},
	}}
}

func TestRename_HappyPath_PreservesAllOtherFieldsAndPosition(t *testing.T) {
	cfg := makeRenameCfg()
	load, save, path, saves := memCfg(cfg)
	svc := newSvc(t, load, save, path)

	res := svc.Rename("middle", "renamed")
	if res.HasError() {
		t.Fatalf("Rename: %v", res.Error())
	}
	if *saves != 1 {
		t.Errorf("saves = %d, want 1 (single-shot persist)", *saves)
	}
	// Reload via the service's own List to see what was persisted.
	listRes := svc.List()
	if listRes.HasError() {
		t.Fatalf("List after rename: %v", listRes.Error())
	}
	rules := listRes.Value()
	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3 (rename must not add/remove)", len(rules))
	}
	// Position preserved: index 1 is still our renamed rule.
	got := rules[1]
	if got.Name != "renamed" {
		t.Errorf("rules[1].Name = %q, want %q", got.Name, "renamed")
	}
	// Every OTHER field copied verbatim.
	want := config.Rule{
		Name: "renamed", Enabled: false, UrlRegex: "^https://b",
		FromRegex: "alice@", SubjectRegex: "[urgent]", BodyRegex: "verify",
	}
	if got != want {
		t.Errorf("rules[1] = %+v, want %+v", got, want)
	}
	// Neighbors untouched.
	if rules[0].Name != "first" || rules[2].Name != "last" {
		t.Errorf("neighbors mutated: %+v / %+v", rules[0], rules[2])
	}
}

func TestRename_TrimsBothNames(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	if res := svc.Rename("  middle ", "  renamed\t"); res.HasError() {
		t.Fatalf("Rename with whitespace-padded names: %v", res.Error())
	}
	if r := svc.Get("renamed"); r.HasError() {
		t.Errorf("Get(\"renamed\") after trim-rename: %v", r.Error())
	}
}

func TestRename_EmptyOldName_Rejected(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("", "newone")
	if !res.HasError() {
		t.Fatal("empty oldName should be rejected")
	}
	mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)
}

func TestRename_WhitespaceOldName_Rejected(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("   \t  ", "newone")
	if !res.HasError() {
		t.Fatal("whitespace-only oldName should be rejected after TrimSpace")
	}
	mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)
}

func TestRename_EmptyNewName_Rejected(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("middle", "")
	if !res.HasError() {
		t.Fatal("empty newName should be rejected")
	}
	mustCoded(t, res.Error(), errtrace.ErrCoreInvalidArgument)
}

func TestRename_SameName_Rejected(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("middle", "middle")
	if !res.HasError() {
		t.Fatal("oldName == newName should be rejected as no-op caller bug")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrRuleRenameNoop)
	if !findCtxKeyRules(c, "name", "middle") {
		t.Errorf("missing name context: %+v", c.Context)
	}
}

func TestRename_SameNameAfterTrim_Rejected(t *testing.T) {
	// "middle " trims to "middle" — must hit the same no-op branch
	// even though the raw inputs differ.
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("middle", "  middle\t")
	if !res.HasError() {
		t.Fatal("post-trim oldName == newName should be rejected")
	}
	mustCoded(t, res.Error(), errtrace.ErrRuleRenameNoop)
}

func TestRename_OldNameMissing_NotFound(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("ghost", "newone")
	if !res.HasError() {
		t.Fatal("rename of non-existent rule should fail")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrRuleNotFound)
	if !findCtxKeyRules(c, "oldName", "ghost") {
		t.Errorf("missing oldName context: %+v", c.Context)
	}
}

func TestRename_NewNameTaken_Duplicate(t *testing.T) {
	svc := newSvcMem(t, makeRenameCfg())
	res := svc.Rename("middle", "last") // "last" already exists
	if !res.HasError() {
		t.Fatal("rename to existing name should fail with duplicate")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrRuleDuplicate)
	if !findCtxKeyRules(c, "newName", "last") {
		t.Errorf("missing newName context: %+v", c.Context)
	}
	// And critically: the rename must NOT have partially applied —
	// "middle" must still exist, "last" must still be the original.
	if r := svc.Get("middle"); r.HasError() {
		t.Errorf("middle should still exist after rejected rename: %v", r.Error())
	}
}

func TestRename_LoadError_Wrapped(t *testing.T) {
	loadErr := errors.New("disk on fire")
	load := func() (*config.Config, error) { return nil, loadErr }
	save := func(*config.Config) error { t.Fatal("save called despite load failure"); return nil }
	path := func() (string, error) { return "/fake", nil }
	svc := newSvc(t, load, save, path)

	res := svc.Rename("middle", "newone")
	if !res.HasError() {
		t.Fatal("load failure should propagate")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrConfigOpen)
	if !errors.Is(res.Error(), loadErr) {
		t.Errorf("error chain should wrap loadErr; got %v", res.Error())
	}
	if !findCtxKeyRules(c, "oldName", "middle") || !findCtxKeyRules(c, "newName", "newone") {
		t.Errorf("missing rename context on load error: %+v", c.Context)
	}
}

func TestRename_SaveError_Wrapped(t *testing.T) {
	saveErr := errors.New("disk full")
	cfg := makeRenameCfg()
	load := func() (*config.Config, error) {
		cp := *cfg
		cp.Rules = append([]config.Rule(nil), cfg.Rules...)
		return &cp, nil
	}
	save := func(*config.Config) error { return saveErr }
	path := func() (string, error) { return "/fake", nil }
	svc := newSvc(t, load, save, path)

	res := svc.Rename("middle", "newone")
	if !res.HasError() {
		t.Fatal("save failure should propagate")
	}
	c := mustCoded(t, res.Error(), errtrace.ErrConfigEncode)
	if !errors.Is(res.Error(), saveErr) {
		t.Errorf("error chain should wrap saveErr; got %v", res.Error())
	}
	if !findCtxKeyRules(c, "oldName", "middle") || !findCtxKeyRules(c, "newName", "newone") {
		t.Errorf("missing rename context on save error: %+v", c.Context)
	}
}

// findCtxKeyRules is a local helper mirroring the one in
// emails_refresh_test.go — kept package-local rather than exported
// to avoid leaking test-only helpers into the public surface. The
// duplicate is trivial (5 lines) and avoids a rename_test.go ↔
// refresh_test.go cross-file fixture dependency.
func findCtxKeyRules(c *errtrace.Coded, key, want string) bool {
	for _, kv := range c.Context {
		if kv.Key == key && kv.Value == want {
			return true
		}
	}
	return false
}
