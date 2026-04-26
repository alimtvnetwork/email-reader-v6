// dashboard_test.go locks the P2.2 contract for `*DashboardService`:
//
//   - Constructor rejects nil dependencies with ErrCoreInvalidArgument
//     (catches lazy "let nil-deref panic at first use" bugs).
//   - Summary faithfully aggregates injected counts (no surprise
//     extra DB calls, alias scope honoured, etc.).
//   - Error envelope on cfg/store failures matches the pre-refactor
//     contract: ErrConfigOpen for cfg load, ErrDbQueryEmail for
//     emails count, with scope/alias context preserved.
//   (Phase 2.8b: the legacy package-level `LoadDashboardStats` and method `LoadStats`
//    wrapper has been deleted; all callers now go through
//    *DashboardService directly.)
//
// All fakes are one-line closures — no test mocks library, no
// reflect tricks. This is the principal payoff of the
// `func`-typed-dependency style chosen in `dashboard.go`.
package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

func TestNewDashboardService_RejectsNilDeps(t *testing.T) {
	cases := []struct {
		name        string
		loadCfg     configLoader
		countEmails emailsCounter
		wantSubstr  string
	}{
		{"nil_loadCfg", nil, fakeCounter(nil), "loadCfg is nil"},
		{"nil_countEmails", fakeLoader(&config.Config{}), nil, "countEmails is nil"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := NewDashboardService(tc.loadCfg, tc.countEmails)
			if !res.HasError() {
				t.Fatalf("expected error, got %+v", res.Value())
			}
			if !hasCode(res.Error(), errtrace.ErrCoreInvalidArgument) {
				t.Errorf("error code = %v, want ErrCoreInvalidArgument", codeOf(res.Error()))
			}
			if got := res.Error().Error(); !containsStr(got, tc.wantSubstr) {
				t.Errorf("error message %q does not contain %q", got, tc.wantSubstr)
			}
		})
	}
}

func TestDashboardService_Summary_AggregatesCounts(t *testing.T) {
	cfg := &config.Config{
		Accounts: []config.Account{{Alias: "a"}, {Alias: "b"}},
		Rules: []config.Rule{
			{Name: "r1", Enabled: true},
			{Name: "r2", Enabled: true},
			{Name: "r3", Enabled: false},
		},
	}
	// Track per-call args so we can assert the alias-scoped count is
	// only requested when alias != "" (the contract).
	calls := []string{}
	counter := func(ctx context.Context, alias string) errtrace.Result[int] {
		calls = append(calls, alias)
		switch alias {
		case "":
			return errtrace.Ok(42)
		case "a":
			return errtrace.Ok(7)
		default:
			return errtrace.Ok(0)
		}
	}
	svc := mustService(t, fakeLoader(cfg), counter)

	t.Run("global_only_when_alias_empty", func(t *testing.T) {
		calls = nil
		res := svc.Summary(context.Background(), "")
		if res.HasError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		got := res.Value()
		if got.Accounts != 2 || got.RulesTotal != 3 || got.RulesEnabled != 2 ||
			got.EmailsTotal != 42 || got.EmailsForAlias != 0 || got.Alias != "" {
			t.Errorf("stats = %+v, want {Accounts:2 RulesTotal:3 RulesEnabled:2 EmailsTotal:42 EmailsForAlias:0 Alias:\"\"}", got)
		}
		if len(calls) != 1 || calls[0] != "" {
			t.Errorf("counter calls = %v, want exactly one call with empty alias", calls)
		}
	})

	t.Run("scoped_when_alias_set", func(t *testing.T) {
		calls = nil
		res := svc.Summary(context.Background(), "a")
		if res.HasError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		got := res.Value()
		if got.EmailsTotal != 42 || got.EmailsForAlias != 7 || got.Alias != "a" {
			t.Errorf("stats = %+v, want EmailsTotal:42 EmailsForAlias:7 Alias:a", got)
		}
		if len(calls) != 2 || calls[0] != "" || calls[1] != "a" {
			t.Errorf("counter calls = %v, want [\"\", \"a\"]", calls)
		}
	})
}

func TestDashboardService_Summary_PropagatesCfgError(t *testing.T) {
	wantInner := errors.New("cfg.json: permission denied")
	loader := func() (*config.Config, error) { return nil, wantInner }
	svc := mustService(t, loader, fakeCounter(nil))

	res := svc.Summary(context.Background(), "")
	if !res.HasError() {
		t.Fatal("expected cfg load failure to surface")
	}
	if !hasCode(res.Error(), errtrace.ErrConfigOpen) {
		t.Errorf("error code = %v, want ErrConfigOpen", codeOf(res.Error()))
	}
	if !errors.Is(res.Error(), wantInner) {
		t.Errorf("Unwrap chain does not contain inner error %v", wantInner)
	}
}

func TestDashboardService_Summary_PropagatesCountError(t *testing.T) {
	cfg := &config.Config{}
	wantInner := errors.New("disk full")
	counter := func(ctx context.Context, alias string) errtrace.Result[int] {
		return errtrace.Err[int](wantInner)
	}
	svc := mustService(t, fakeLoader(cfg), counter)

	res := svc.Summary(context.Background(), "")
	if !res.HasError() {
		t.Fatal("expected count failure to surface")
	}
	if !hasCode(res.Error(), errtrace.ErrDbQueryEmail) {
		t.Errorf("error code = %v, want ErrDbQueryEmail", codeOf(res.Error()))
	}
	if !errors.Is(res.Error(), wantInner) {
		t.Errorf("Unwrap chain does not contain inner error %v", wantInner)
	}
}

// --- test helpers ---

func mustService(t *testing.T, l configLoader, c emailsCounter) *DashboardService {
	t.Helper()
	res := NewDashboardService(l, c)
	if res.HasError() {
		t.Fatalf("NewDashboardService: %v", res.Error())
	}
	return res.Value()
}

func fakeLoader(cfg *config.Config) configLoader {
	return func() (*config.Config, error) { return cfg, nil }
}

func fakeCounter(perAlias map[string]int) emailsCounter {
	return func(ctx context.Context, alias string) errtrace.Result[int] {
		return errtrace.Ok(perAlias[alias])
	}
}

// codeOf returns the first errtrace.Code found by walking the
// Unwrap chain. Distinct from the existing `hasCode` helper in
// watch_factory_test.go (which only returns bool); we need the
// actual Code value for error-message formatting.
func codeOf(err error) errtrace.Code {
	for e := err; e != nil; e = errors.Unwrap(e) {
		var c *errtrace.Coded
		if errors.As(e, &c) {
			return c.Code
		}
	}
	return ""
}

// containsStr is the local alias for strings.Contains kept for
// readability of the test body. The package-level `contains` helper
// in settings_test.go has a different signature (rune-based scan)
// so we use a uniquely-named wrapper to avoid the collision.
func containsStr(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
