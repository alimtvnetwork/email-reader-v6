// Package rules evaluates regex-based rules against fetched emails and
// returns the set of URLs to open. All regex fields are optional —
// an empty pattern means "match anything".
package rules

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/mailclient"
)

// Match represents a single (rule, url) pair to be acted upon.
type Match struct {
	RuleName string
	Url      string
}

// compiled holds the precompiled regexes for one rule.
type compiled struct {
	rule    config.Rule
	from    *regexp.Regexp
	subject *regexp.Regexp
	body    *regexp.Regexp
	url     *regexp.Regexp
}

// Engine evaluates a set of rules.
type Engine struct {
	mu       sync.Mutex
	compiled []compiled
}

// New builds an Engine from the given rules. Disabled rules are skipped.
// Invalid regex patterns produce a non-fatal warning via the returned error
// (other rules still load).
func New(rs []config.Rule) (*Engine, error) {
	e := &Engine{}
	var firstErr error
	for _, r := range rs {
		if !r.Enabled {
			continue
		}
		c := compiled{rule: r}
		var err error
		if c.from, err = compileOpt(r.FromRegex); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("rule %q fromRegex: %w", r.Name, err)
			continue
		}
		if c.subject, err = compileOpt(r.SubjectRegex); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("rule %q subjectRegex: %w", r.Name, err)
			continue
		}
		if c.body, err = compileOpt(r.BodyRegex); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("rule %q bodyRegex: %w", r.Name, err)
			continue
		}
		if c.url, err = compileOpt(r.UrlRegex); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("rule %q urlRegex: %w", r.Name, err)
			continue
		}
		e.compiled = append(e.compiled, c)
	}
	return e, firstErr
}

func compileOpt(p string) (*regexp.Regexp, error) {
	if p == "" {
		return nil, nil
	}
	return regexp.Compile(p)
}

// Evaluate runs every enabled rule against the message and returns
// distinct (rule, url) matches. URLs are deduped per rule.
func (e *Engine) Evaluate(m *mailclient.Message) []Match {
	e.mu.Lock()
	defer e.mu.Unlock()

	var out []Match
	body := m.BodyText
	if body == "" {
		body = m.BodyHtml
	}
	for _, c := range e.compiled {
		if c.from != nil && !c.from.MatchString(m.From) {
			continue
		}
		if c.subject != nil && !c.subject.MatchString(m.Subject) {
			continue
		}
		if c.body != nil && !c.body.MatchString(body) {
			continue
		}
		if c.url == nil {
			continue // no URL pattern → nothing to open
		}
		urls := c.url.FindAllString(body, -1)
		seen := map[string]struct{}{}
		for _, u := range urls {
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			out = append(out, Match{RuleName: c.rule.Name, Url: u})
		}
	}
	return out
}
