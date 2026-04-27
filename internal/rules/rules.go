// Package rules evaluates regex-based rules against fetched emails and
// returns the set of URLs to open. All regex fields are optional —
// an empty pattern means "match anything".
package rules

import (
	"regexp"
	"sync"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
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
		bad := false
		recordErr := func(field string, e error) {
			bad = true
			if firstErr == nil {
				firstErr = errtrace.Wrapf(e, "rule %q %s", r.Name, field)
			}
		}
		if c.from, err = compileOpt(r.FromRegex); err != nil {
			recordErr("fromRegex", err)
		}
		if c.subject, err = compileOpt(r.SubjectRegex); err != nil {
			recordErr("subjectRegex", err)
		}
		if c.body, err = compileOpt(r.BodyRegex); err != nil {
			recordErr("bodyRegex", err)
		}
		if c.url, err = compileOpt(r.UrlRegex); err != nil {
			recordErr("urlRegex", err)
		}
		if bad {
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

// RuleTrace is a per-rule explanation of why a rule did or did not produce
// URLs for a given message. Surfaced to the watcher so the user can see
// exactly which condition rejected the message.
type RuleTrace struct {
	RuleName  string   // rule name from config
	Skipped   bool     // true if rule was skipped (no URL regex / disabled)
	Reason    string   // short human reason ("from regex no match", "matched: 2 url(s)", ...)
	UrlsFound []string // URLs harvested when fully matched
}

// Evaluate runs every enabled rule against the message and returns
// distinct (rule, url) matches. URLs are deduped per rule.
func (e *Engine) Evaluate(m *mailclient.Message) []Match {
	matches, _ := e.EvaluateWithTrace(m)
	return matches
}

// EvaluateWithTrace returns matches PLUS a per-rule explanation suitable
// for logging. Always allocates a trace entry per compiled rule so the
// caller can show "rule X: from regex did not match" to the user — that
// makes "✉ new mail but nothing opened" debuggable without --verbose.
func (e *Engine) EvaluateWithTrace(m *mailclient.Message) ([]Match, []RuleTrace) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var out []Match
	traces := make([]RuleTrace, 0, len(e.compiled))
	body := m.BodyText
	if body == "" {
		body = m.BodyHtml
	}
	for _, c := range e.compiled {
		t := RuleTrace{RuleName: c.rule.Name}
		switch {
		case c.from != nil && !c.from.MatchString(m.From):
			t.Reason = "fromRegex did not match From: " + truncate(m.From, 80)
		case c.subject != nil && !c.subject.MatchString(m.Subject):
			t.Reason = "subjectRegex did not match Subject: " + truncate(m.Subject, 80)
		case c.body != nil && !c.body.MatchString(body):
			t.Reason = "bodyRegex did not match body (" + lengthHint(body) + ")"
		case c.url == nil:
			t.Skipped = true
			t.Reason = "no urlRegex configured — rule cannot open anything"
		default:
			urls := c.url.FindAllString(body, -1)
			seen := map[string]struct{}{}
			for _, u := range urls {
				if _, ok := seen[u]; ok {
					continue
				}
				seen[u] = struct{}{}
				t.UrlsFound = append(t.UrlsFound, u)
				out = append(out, Match{RuleName: c.rule.Name, Url: u})
			}
			if len(t.UrlsFound) == 0 {
				t.Reason = "urlRegex matched 0 URLs in body (regex too strict?)"
			} else {
				t.Reason = "matched, harvested URL(s)"
			}
		}
		traces = append(traces, t)
	}
	return out, traces
}

// RuleCount returns how many enabled rules are loaded. Used by the watcher
// to warn when 0 rules exist (a common cause of "nothing opens").
func (e *Engine) RuleCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.compiled)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func lengthHint(s string) string {
	if len(s) == 0 {
		return "empty body"
	}
	if len(s) < 200 {
		return "short body, " + itoa(len(s)) + " bytes"
	}
	return itoa(len(s)) + " bytes"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
