package core

import (
	"context"
	"fmt"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
)

// ReadEventKind classifies an event emitted while replaying a saved email.
// The CLI translates these into log lines; the UI renders them as cards
// or appends them to a log panel.
type ReadEventKind int

const (
	ReadEventStart           ReadEventKind = iota // email loaded; payload = EmailDetail summary
	ReadEventSeededRule                           // a default rule was auto-installed
	ReadEventRulesLoaded                          // rules engine ready; Count populated
	ReadEventBrowserResolved                      // browser path detected
	ReadEventNoRules                              // 0 enabled rules — abort
	ReadEventRuleTrace                            // one per evaluated rule
	ReadEventNoMatches                            // engine returned 0 matches
	ReadEventCooldown                             // 10s gap before next URL
	ReadEventOpening                              // about to launch URL
	ReadEventOpened                               // URL successfully launched
	ReadEventOpenFailed                           // browser launch failed
	ReadEventDone                                 // pipeline finished
)

// ReadEvent is one step in the read pipeline. Only the fields relevant to
// Kind are populated; everything else is the zero value.
type ReadEvent struct {
	Kind         ReadEventKind
	Email        *EmailDetail   // ReadEventStart
	RuleName     string         // SeededRule, RuleTrace, Opening, Opened, OpenFailed
	RuleCount    int            // RulesLoaded
	BrowserPath  string         // BrowserResolved
	IncognitoArg string         // BrowserResolved
	Trace        rules.RuleTrace // RuleTrace
	Url          string         // Opening, Opened, OpenFailed
	Index        int            // Opening (0-based position in match list)
	Total        int            // Opening (total match count)
	CooldownFor  time.Duration  // Cooldown
	Err          error          // OpenFailed
	Note         string         // free-form supplementary text
}

// ReadEmail re-runs the rule engine against a previously stored email and
// opens any matched URLs in incognito. Progress is reported via the emit
// callback so callers (CLI and UI) decide how to render each event.
//
// The function honours ctx for cancellation between URL launches; the 10s
// cooldown between URLs returns early when ctx is cancelled. Returns an
// error only for fatal setup failures (missing account, browser unresolved,
// store I/O); per-URL launch failures are reported via OpenFailed events
// and do not abort the loop.
func ReadEmail(ctx context.Context, alias string, uid uint32, emit func(ReadEvent)) error {
	if emit == nil {
		emit = func(ReadEvent) {}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if cfg.FindAccount(alias) == nil {
		return errtrace.New(fmt.Sprintf("no account with alias %q", alias))
	}

	st, err := store.Open()
	if err != nil {
		return errtrace.Wrap(err, "open store")
	}
	defer st.Close()

	row, err := st.GetEmailByUid(ctx, alias, uid)
	if err != nil {
		return errtrace.Wrapf(err, "load email alias=%s uid=%d", alias, uid)
	}
	detail := EmailDetail{
		EmailSummary: toSummary(*row),
		To:           row.ToAddr,
		Cc:           row.CcAddr,
		BodyText:     row.BodyText,
		BodyHtml:     row.BodyHtml,
	}
	emit(ReadEvent{Kind: ReadEventStart, Email: &detail})

	// Auto-seed a default rule when no enabled rules exist so the command
	// works out-of-the-box. Same behaviour as the watcher.
	if CountEnabledRules(cfg.Rules) == 0 {
		seeded := config.Rule{
			Name:     "default-open-any-url",
			Enabled:  true,
			UrlRegex: `https?://[^\s<>"'\)\]]+`,
		}
		cfg.Rules = append(cfg.Rules, seeded)
		if err := config.Save(cfg); err != nil {
			emit(ReadEvent{Kind: ReadEventSeededRule, RuleName: seeded.Name,
				Note: "warning: could not save: " + err.Error()})
		} else {
			emit(ReadEvent{Kind: ReadEventSeededRule, RuleName: seeded.Name})
		}
	}

	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		emit(ReadEvent{Kind: ReadEventRulesLoaded, Note: "warning: " + ruleErr.Error()})
	}
	if engine == nil || engine.RuleCount() == 0 {
		emit(ReadEvent{Kind: ReadEventNoRules})
		return nil
	}
	emit(ReadEvent{Kind: ReadEventRulesLoaded, RuleCount: engine.RuleCount()})

	launcher := browser.New(cfg.Browser)
	path, err := launcher.Path()
	if err != nil {
		return errtrace.Wrap(err, "resolve browser")
	}
	emit(ReadEvent{
		Kind:         ReadEventBrowserResolved,
		BrowserPath:  path,
		IncognitoArg: launcher.IncognitoArg(),
	})

	msg := &mailclient.Message{
		Uid:        row.Uid,
		MessageId:  row.MessageId,
		From:       row.FromAddr,
		To:         row.ToAddr,
		Cc:         row.CcAddr,
		Subject:    row.Subject,
		BodyText:   row.BodyText,
		BodyHtml:   row.BodyHtml,
		ReceivedAt: row.ReceivedAt,
	}
	matches, traces := engine.EvaluateWithTrace(msg)
	for _, t := range traces {
		emit(ReadEvent{Kind: ReadEventRuleTrace, RuleName: t.RuleName, Trace: t})
	}
	if len(matches) == 0 {
		emit(ReadEvent{Kind: ReadEventNoMatches})
		emit(ReadEvent{Kind: ReadEventDone})
		return nil
	}

	for i, match := range matches {
		if err := ctx.Err(); err != nil {
			return errtrace.Wrap(err, "cancelled mid-batch")
		}
		if i > 0 {
			emit(ReadEvent{Kind: ReadEventCooldown, CooldownFor: 10 * time.Second})
			if err := sleepCtx(ctx, 10*time.Second); err != nil {
				return errtrace.Wrap(err, "cooldown between URLs")
			}
		}
		if _, err := st.RecordOpenedUrl(ctx, row.Id, match.RuleName, match.Url); err != nil {
			// Non-fatal — most likely a UNIQUE conflict (already opened once).
			emit(ReadEvent{
				Kind: ReadEventOpening, RuleName: match.RuleName, Url: match.Url,
				Index: i, Total: len(matches),
				Note: "already in OpenedUrls — re-launching anyway",
			})
		} else {
			emit(ReadEvent{
				Kind: ReadEventOpening, RuleName: match.RuleName, Url: match.Url,
				Index: i, Total: len(matches),
			})
		}
		if err := launcher.Open(match.Url); err != nil {
			emit(ReadEvent{Kind: ReadEventOpenFailed, RuleName: match.RuleName,
				Url: match.Url, Err: err})
			continue
		}
		emit(ReadEvent{Kind: ReadEventOpened, RuleName: match.RuleName, Url: match.Url})
	}
	emit(ReadEvent{Kind: ReadEventDone})
	return nil
}

// sleepCtx pauses for d or returns early when ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
