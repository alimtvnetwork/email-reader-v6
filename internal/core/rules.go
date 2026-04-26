// rules.go — typed *RulesService + thin package-level wrappers.
//
// **Phase 2.6 refactor.** The old shape exposed `AddRule`, `ListRules`,
// `GetRule`, `SetRuleEnabled`, `RemoveRule` as package-level funcs
// that each called `config.Load()` + `config.Save()` (process-globals
// reaching for `~/.config/email-read/config.json`). That made rule
// CRUD impossible to test without writing real files and made it
// impossible to share an in-memory config across services in a single
// UI tick.
//
// The new shape mirrors `core.DashboardService` / `core.EmailsService`
// (see `dashboard.go`, `emails.go`):
//
//   - `RulesService` struct holds two injected deps:
//       * `loadCfg configLoader` — read side (already declared in
//         dashboard.go and reused here verbatim)
//       * `saveCfg cfgWriter`    — write side, declared below
//     Read and write are split so tests can fake-write to memory
//     and so a future read-only consumer can construct a service
//     that explicitly cannot mutate config.
//   - `NewRulesService` is the explicit constructor; nil in either
//     slot → ErrCoreInvalidArgument.
//   - `Add`/`List`/`Get`/`SetEnabled`/`Remove` are the typed methods
//     that replace the old package funcs. Same error envelope, same
//     semantics, only the dependency source changed.
//   - The package-level `AddRule` / `ListRules` / `GetRule` /
//     `SetRuleEnabled` / `RemoveRule` stay as deprecated thin
//     wrappers that build a default-injected service per call.
//     Wrappers go away in P2.8.
//   - `CountEnabledRules` stays as a pure helper (no deps to inject).
package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// RuleInput captures everything needed to create or update a rule.
// Only Name and UrlRegex are required. Empty regex fields mean "match anything".
type RuleInput struct {
	Name         string
	UrlRegex     string
	FromRegex    string
	SubjectRegex string
	BodyRegex    string
	Enabled      bool
}

// AddRuleResult reports the saved rule plus the config path written.
type AddRuleResult struct {
	Rule       config.Rule
	ConfigPath string
	Replaced   bool // true if an existing rule with the same name was overwritten
}

// cfgWriter persists a config snapshot. Functional shape (matching
// `configLoader` in dashboard.go) so tests can write to memory in a
// one-line closure.
type cfgWriter func(*config.Config) error

// cfgPathFn returns the on-disk path of the active config (used by
// AddRuleResult so the UI can echo "wrote to ..."). Functional shape;
// tests typically return a fixed string.
type cfgPathFn func() (string, error)

// RulesService performs CRUD on configured rules via injected
// load/save/path deps. Stateless — concurrent method calls each
// load + save independently (config.Save is itself the locking
// boundary).
type RulesService struct {
	loadCfg configLoader
	saveCfg cfgWriter
	cfgPath cfgPathFn
}

// NewRulesService constructs a RulesService. All three deps are
// required; nil in any slot returns ErrCoreInvalidArgument (no
// defensive default-injection — bootstrap is the right place).
func NewRulesService(loadCfg configLoader, saveCfg cfgWriter, cfgPath cfgPathFn) errtrace.Result[*RulesService] {
	if loadCfg == nil {
		return errtrace.Err[*RulesService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewRulesService: loadCfg is nil"))
	}
	if saveCfg == nil {
		return errtrace.Err[*RulesService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewRulesService: saveCfg is nil"))
	}
	if cfgPath == nil {
		return errtrace.Err[*RulesService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewRulesService: cfgPath is nil"))
	}
	return errtrace.Ok(&RulesService{loadCfg: loadCfg, saveCfg: saveCfg, cfgPath: cfgPath})
}

// NewDefaultRulesService is the production-bootstrap convenience
// constructor: builds a `*RulesService` wired to the real
// `config.Load` / `config.Save` / `config.Path`. Used by `internal/ui`
// (Phase 2.7 wiring) so the UI package never needs to know the
// unexported `configLoader` / `cfgWriter` / `cfgPathFn` shapes.
//
// Cannot fail in practice (all three deps are non-nil), but returns
// a Result envelope to keep the constructor signature parallel with
// `NewRulesService` and `NewDefaultEmailsService`.
func NewDefaultRulesService() errtrace.Result[*RulesService] {
	return NewRulesService(config.Load, config.Save, config.Path)
}

// Add validates input, compiles regex patterns to catch syntax errors
// before persisting, and writes the rule. Upsert semantics: a rule
// with the same name is replaced.
func (s *RulesService) Add(in RuleInput) errtrace.Result[*AddRuleResult] {
	in.Name = strings.TrimSpace(in.Name)
	in.UrlRegex = strings.TrimSpace(in.UrlRegex)
	if in.Name == "" || in.UrlRegex == "" {
		return errtrace.Err[*AddRuleResult](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "name and urlRegex are required").
			WithContext("name", in.Name).
			WithContext("urlRegex", in.UrlRegex))
	}
	// Validate every regex up-front so we never persist a broken rule.
	for label, pat := range map[string]string{
		"urlRegex":     in.UrlRegex,
		"fromRegex":    in.FromRegex,
		"subjectRegex": in.SubjectRegex,
		"bodyRegex":    in.BodyRegex,
	} {
		if pat == "" {
			continue
		}
		if _, err := regexp.Compile(pat); err != nil {
			return errtrace.Err[*AddRuleResult](errtrace.WrapCode(err,
				errtrace.ErrRulePatternInvalid, "compile regex").
				WithContext("field", label).
				WithContext("pattern", pat))
		}
	}

	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[*AddRuleResult](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config"))
	}
	r := config.Rule{
		Name:         in.Name,
		Enabled:      in.Enabled,
		FromRegex:    in.FromRegex,
		SubjectRegex: in.SubjectRegex,
		BodyRegex:    in.BodyRegex,
		UrlRegex:     in.UrlRegex,
	}
	replaced := false
	if existing := cfg.FindRule(in.Name); existing != nil {
		*existing = r
		replaced = true
	} else {
		cfg.Rules = append(cfg.Rules, r)
	}
	if err := s.saveCfg(cfg); err != nil {
		return errtrace.Err[*AddRuleResult](errtrace.WrapCode(err,
			errtrace.ErrConfigEncode, "save config").
			WithContext("rule", in.Name))
	}
	p, _ := s.cfgPath()
	return errtrace.Ok(&AddRuleResult{Rule: r, ConfigPath: p, Replaced: replaced})
}

// List returns all configured rules (a copy — safe to mutate).
func (s *RulesService) List() errtrace.Result[[]config.Rule] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[[]config.Rule](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config"))
	}
	out := make([]config.Rule, len(cfg.Rules))
	copy(out, cfg.Rules)
	return errtrace.Ok(out)
}

// Get returns the rule with the given name or ErrRuleNotFound.
func (s *RulesService) Get(name string) errtrace.Result[config.Rule] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[config.Rule](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config"))
	}
	r := cfg.FindRule(name)
	if r == nil {
		return errtrace.Err[config.Rule](errtrace.NewCoded(
			errtrace.ErrRuleNotFound, fmt.Sprintf("no rule with name %q", name)).
			WithContext("name", name))
	}
	return errtrace.Ok(*r)
}

// SetEnabled flips the enabled flag of an existing rule.
func (s *RulesService) SetEnabled(name string, enabled bool) errtrace.Result[struct{}] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[struct{}](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config"))
	}
	r := cfg.FindRule(name)
	if r == nil {
		return errtrace.Err[struct{}](errtrace.NewCoded(
			errtrace.ErrRuleNotFound, fmt.Sprintf("no rule with name %q", name)).
			WithContext("name", name))
	}
	r.Enabled = enabled
	if err := s.saveCfg(cfg); err != nil {
		return errtrace.Err[struct{}](errtrace.WrapCode(err,
			errtrace.ErrConfigEncode, "save config").
			WithContext("name", name))
	}
	return errtrace.Ok(struct{}{})
}

// Remove deletes the rule with the given name. Returns
// ErrRuleNotFound if no such rule exists.
func (s *RulesService) Remove(name string) errtrace.Result[struct{}] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[struct{}](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config"))
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].Name == name {
			cfg.Rules = append(cfg.Rules[:i], cfg.Rules[i+1:]...)
			if err := s.saveCfg(cfg); err != nil {
				return errtrace.Err[struct{}](errtrace.WrapCode(err,
					errtrace.ErrConfigEncode, "save config").
					WithContext("name", name))
			}
			return errtrace.Ok(struct{}{})
		}
	}
	return errtrace.Err[struct{}](errtrace.NewCoded(
		errtrace.ErrRuleNotFound, fmt.Sprintf("no rule with name %q", name)).
		WithContext("name", name))
}

// CountEnabledRules reports how many rules in the slice are enabled.
// Used by watch/read to decide whether to seed a default open-any-url
// rule. Pure helper — no service needed.
func CountEnabledRules(rs []config.Rule) int {
	n := 0
	for _, r := range rs {
		if r.Enabled {
			n++
		}
	}
	return n
}
