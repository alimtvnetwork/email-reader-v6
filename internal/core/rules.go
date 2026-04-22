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

// AddRule validates input, compiles regex patterns to catch syntax errors
// before persisting, and writes the rule to config.json. If a rule with the
// same name exists it is replaced (upsert semantics).
func AddRule(in RuleInput) (*AddRuleResult, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.UrlRegex = strings.TrimSpace(in.UrlRegex)
	if in.Name == "" || in.UrlRegex == "" {
		return nil, errtrace.New("name and urlRegex are required")
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
			return nil, errtrace.Wrapf(err, "invalid %s", label)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.Wrap(err, "load config")
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
	if err := config.Save(cfg); err != nil {
		return nil, errtrace.Wrap(err, "save config")
	}
	p, _ := config.Path()
	return &AddRuleResult{Rule: r, ConfigPath: p, Replaced: replaced}, nil
}

// ListRules returns all configured rules (a copy — safe to mutate).
func ListRules() ([]config.Rule, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.Wrap(err, "load config")
	}
	out := make([]config.Rule, len(cfg.Rules))
	copy(out, cfg.Rules)
	return out, nil
}

// GetRule returns the rule with the given name or an error.
func GetRule(name string) (config.Rule, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Rule{}, errtrace.Wrap(err, "load config")
	}
	r := cfg.FindRule(name)
	if r == nil {
		return config.Rule{}, errtrace.New(fmt.Sprintf("no rule with name %q", name))
	}
	return *r, nil
}

// SetRuleEnabled flips the enabled flag of an existing rule.
func SetRuleEnabled(name string, enabled bool) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	r := cfg.FindRule(name)
	if r == nil {
		return errtrace.New(fmt.Sprintf("no rule with name %q", name))
	}
	r.Enabled = enabled
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	return nil
}

// RemoveRule deletes the rule with the given name. Returns an error if
// no such rule exists.
func RemoveRule(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].Name == name {
			cfg.Rules = append(cfg.Rules[:i], cfg.Rules[i+1:]...)
			if err := config.Save(cfg); err != nil {
				return errtrace.Wrap(err, "save config")
			}
			return nil
		}
	}
	return errtrace.New(fmt.Sprintf("no rule with name %q", name))
}

// CountEnabledRules reports how many rules in the slice are enabled.
// Used by watch/read to decide whether to seed a default open-any-url rule.
func CountEnabledRules(rs []config.Rule) int {
	n := 0
	for _, r := range rs {
		if r.Enabled {
			n++
		}
	}
	return n
}
