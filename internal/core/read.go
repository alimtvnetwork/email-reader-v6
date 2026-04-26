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
// loadEmailDetail validates the alias against config, opens the store, and
// fetches the saved email + summary. Caller owns the returned *store.Store
// and must Close() it.
func loadEmailDetail(ctx context.Context, alias string, uid uint32) (*config.Config, *store.Store, *store.Email, EmailDetail, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, EmailDetail{}, errtrace.Wrap(err, "load config")
	}
	if cfg.FindAccount(alias) == nil {
		return cfg, nil, nil, EmailDetail{}, errtrace.New(fmt.Sprintf("no account with alias %q", alias))
	}
	st, err := store.Open()
	if err != nil {
		return cfg, nil, nil, EmailDetail{}, errtrace.Wrap(err, "open store")
	}
	row, err := st.GetEmailByUid(ctx, alias, uid)
	if err != nil {
		st.Close()
		return cfg, nil, nil, EmailDetail{}, errtrace.Wrapf(err, "load email alias=%s uid=%d", alias, uid)
	}
	detail := EmailDetail{
		EmailSummary: toSummary(*row),
		To:           row.ToAddr, Cc: row.CcAddr,
		BodyText: row.BodyText, BodyHtml: row.BodyHtml,
	}
	return cfg, st, row, detail, nil
}

// ensureSeededRule installs the default URL-matching rule when no rule is
// enabled, mirroring the watcher's first-run behaviour.
func ensureSeededRule(cfg *config.Config, emit func(ReadEvent)) {
	if CountEnabledRules(cfg.Rules) != 0 {
		return
	}
	seeded := config.Rule{
		Name: "default-open-any-url", Enabled: true,
		UrlRegex: `https?://[^\s<>"'\)\]]+`,
	}
	cfg.Rules = append(cfg.Rules, seeded)
	if err := config.Save(cfg); err != nil {
		emit(ReadEvent{Kind: ReadEventSeededRule, RuleName: seeded.Name,
			Note: "warning: could not save: " + err.Error()})
		return
	}
	emit(ReadEvent{Kind: ReadEventSeededRule, RuleName: seeded.Name})
}

// buildEngineAndLauncher constructs the rules engine + browser launcher
// and emits the corresponding events. Returns (nil, nil, nil) when the
// caller should exit early (no enabled rules).
func buildEngineAndLauncher(cfg config.Config, emit func(ReadEvent)) (*rules.Engine, *browser.Launcher, error) {
	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		emit(ReadEvent{Kind: ReadEventRulesLoaded, Note: "warning: " + ruleErr.Error()})
	}
	if engine == nil || engine.RuleCount() == 0 {
		emit(ReadEvent{Kind: ReadEventNoRules})
		return nil, nil, nil
	}
	emit(ReadEvent{Kind: ReadEventRulesLoaded, RuleCount: engine.RuleCount()})
	launcher := browser.New(cfg.Browser)
	path, err := launcher.Path()
	if err != nil {
		return nil, nil, errtrace.Wrap(err, "resolve browser")
	}
	emit(ReadEvent{Kind: ReadEventBrowserResolved, BrowserPath: path, IncognitoArg: launcher.IncognitoArg()})
	return engine, launcher, nil
}

// rowToMessage adapts a stored Email row back into a mailclient.Message
// so the rules engine can re-evaluate it.
func rowToMessage(row *store.Email) *mailclient.Message {
	return &mailclient.Message{
		Uid: row.Uid, MessageId: row.MessageId,
		From: row.FromAddr, To: row.ToAddr, Cc: row.CcAddr,
		Subject:  row.Subject,
		BodyText: row.BodyText, BodyHtml: row.BodyHtml,
		ReceivedAt: row.ReceivedAt,
	}
}

// evaluateMatches runs the engine, emits trace events, and returns the
// matched URLs. When there are no matches it emits NoMatches+Done and
// returns nil so the caller exits cleanly.
func evaluateMatches(engine *rules.Engine, msg *mailclient.Message, emit func(ReadEvent)) []rules.Match {
	matches, traces := engine.EvaluateWithTrace(msg)
	for _, t := range traces {
		emit(ReadEvent{Kind: ReadEventRuleTrace, RuleName: t.RuleName, Trace: t})
	}
	if len(matches) == 0 {
		emit(ReadEvent{Kind: ReadEventNoMatches})
		emit(ReadEvent{Kind: ReadEventDone})
		return nil
	}
	return matches
}

// openOneMatch records the URL in OpenedUrls (non-fatal on conflict),
// emits Opening, then attempts the launch and emits Opened or OpenFailed.
func openOneMatch(ctx context.Context, st *store.Store, launcher *browser.Launcher, emailId int64, match rules.Match, i, total int, emit func(ReadEvent)) {
	note := ""
	if _, err := st.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url); err != nil {
		note = "already in OpenedUrls — re-launching anyway"
	}
	emit(ReadEvent{
		Kind: ReadEventOpening, RuleName: match.RuleName, Url: match.Url,
		Index: i, Total: total, Note: note,
	})
	if err := launcher.Open(match.Url); err != nil {
		emit(ReadEvent{Kind: ReadEventOpenFailed, RuleName: match.RuleName, Url: match.Url, Err: err})
		return
	}
	emit(ReadEvent{Kind: ReadEventOpened, RuleName: match.RuleName, Url: match.Url})
}

// openMatches walks the matched URLs with a 10s gap between launches and
// honours ctx cancellation.
func openMatches(ctx context.Context, st *store.Store, launcher *browser.Launcher, emailId int64, matches []rules.Match, emit func(ReadEvent)) error {
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
		openOneMatch(ctx, st, launcher, emailId, match, i, len(matches), emit)
	}
	return nil
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
	cfg, st, row, detail, err := loadEmailDetail(ctx, alias, uid)
	if err != nil {
		return err
	}
	defer st.Close()
	emit(ReadEvent{Kind: ReadEventStart, Email: &detail})
	ensureSeededRule(&cfg, emit)
	engine, launcher, err := buildEngineAndLauncher(cfg, emit)
	if err != nil {
		return err
	}
	if engine == nil {
		return nil
	}
	matches := evaluateMatches(engine, rowToMessage(row), emit)
	if matches == nil {
		return nil
	}
	if err := openMatches(ctx, st, launcher, row.Id, matches, emit); err != nil {
		return err
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
