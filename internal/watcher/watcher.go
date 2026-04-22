// Package watcher runs the polling loop for a single account: connect to IMAP,
// fetch new messages by UID, persist them to SQLite + filesystem, evaluate
// rules, and open matching URLs in an incognito browser. Stops cleanly on
// context cancellation (Ctrl+C in the CLI).
package watcher

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
)

// Options bundles everything the watcher needs to run.
type Options struct {
	Account     config.Account
	PollSeconds int
	Engine      *rules.Engine
	Launcher    *browser.Launcher
	Store       *store.Store
	Logger      *log.Logger // optional; defaults to stdout
	Verbose     bool        // if true, log every poll step. Default: only state changes.
}

// Run blocks until ctx is cancelled. It logs progress and tolerates
// transient IMAP errors by reconnecting on the next tick.
//
// Logging modes:
//   - Quiet (default): only startup banner, baseline, new-mail events,
//     errors, and a periodic heartbeat. Idle polls are silent.
//   - Verbose (--verbose): every poll step is logged.
func Run(ctx context.Context, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	poll := time.Duration(opts.PollSeconds) * time.Second
	if poll <= 0 {
		poll = 3 * time.Second
	}

	alias := opts.Account.Alias
	logger.Printf("[%s] watching %s @ %s:%d (poll=%s)  press Ctrl+C to stop",
		alias, opts.Account.Email, opts.Account.ImapHost, opts.Account.ImapPort, poll)
	if !opts.Verbose {
		logger.Printf("[%s] quiet mode — only new mail / errors / heartbeat will be shown. Re-run with --verbose for full trace.", alias)
	}
	// Always announce rule + browser readiness so the user can immediately tell
	// why "new mail" arrives but nothing opens.
	if opts.Engine == nil {
		logger.Printf("[%s] ⚠ no rules engine attached — incoming mail will be saved but no URLs will be opened", alias)
	} else {
		n := opts.Engine.RuleCount()
		if n == 0 {
			logger.Printf("[%s] ⚠ 0 enabled rules loaded — incoming mail will be saved but no URLs will be opened. Add a rule in data/config.json (rules[].enabled=true with a urlRegex).", alias)
		} else {
			logger.Printf("[%s] %d enabled rule(s) loaded", alias, n)
		}
	}
	if opts.Launcher == nil {
		logger.Printf("[%s] ⚠ no browser launcher attached — URLs will be matched but never opened", alias)
	} else if path, err := opts.Launcher.Path(); err != nil {
		logger.Printf("[%s] ⚠ browser not resolved yet:\n%s", alias, errtrace.Format(err))
	} else {
		logger.Printf("[%s] browser ready: %s (incognito flag=%q)", alias, path, opts.Launcher.IncognitoArg())
	}

	tick := time.NewTimer(0)
	defer tick.Stop()

	// Cross-poll diagnostic state.
	var (
		prevMessages    uint32
		prevUidNext     uint32
		prevUidValidity uint32
		pollCount       int
		startedAt       = time.Now()
		havePrev        bool
		lastError       string // de-dupe spammy repeated errors in quiet mode
	)
	const heartbeatEvery = 60 // ~ every 60 polls (~3 min at 3s poll)

	for {
		select {
		case <-ctx.Done():
			logger.Printf("[%s] stopped (%s, %d polls)", alias, time.Since(startedAt).Round(time.Second), pollCount)
			return nil
		case <-tick.C:
			pollCount++
			stats, err := pollOnce(ctx, opts, logger)
			if err != nil {
				msg := err.Error()
				if opts.Verbose || msg != lastError {
					logger.Printf("[%s] ✗ poll error:\n%s", alias, errtrace.Format(err))
					lastError = msg
				}
			} else if stats != nil {
				lastError = ""
				if havePrev {
					if stats.UidValidity != prevUidValidity {
						logger.Printf("[%s] ⚠ UIDVALIDITY changed %d → %d (mailbox reset on server, baseline will reset)",
							alias, prevUidValidity, stats.UidValidity)
					}
					if opts.Verbose && (stats.Messages != prevMessages || stats.UidNext != prevUidNext) {
						logger.Printf("[%s] server state: messages %d→%d, uidNext %d→%d",
							alias, prevMessages, stats.Messages, prevUidNext, stats.UidNext)
					}
				}
				prevMessages = stats.Messages
				prevUidNext = stats.UidNext
				prevUidValidity = stats.UidValidity
				havePrev = true

				if pollCount%heartbeatEvery == 0 {
					logger.Printf("[%s] ♥ alive — %s, %d polls, mailbox messages=%d uidNext=%d (no new mail since last heartbeat)",
						alias, time.Since(startedAt).Round(time.Second), pollCount, stats.Messages, stats.UidNext)
				}
			}
			tick.Reset(poll)
		}
	}
}

// pollOnce performs a single connect → fetch → persist → match → open cycle.
// In quiet mode (opts.Verbose=false) it stays silent unless something
// noteworthy happens (new mail, errors, baseline being set).
func pollOnce(ctx context.Context, opts Options, logger *log.Logger) (*mailclient.MailboxStats, error) {
	start := time.Now()
	alias := opts.Account.Alias
	v := opts.Verbose

	if v {
		logger.Printf("[%s] poll start", alias)
	}

	ws, err := opts.Store.GetWatchState(ctx, alias)
	if err != nil {
		return nil, errtrace.Wrap(err, "get watch state")
	}
	if v {
		logger.Printf("[%s] watch state loaded: lastUid=%d lastSubject=%q",
			alias, ws.LastUid, ws.LastSubject)
		logger.Printf("[%s] dialing IMAP %s:%d (tls=%v) as %s",
			alias, opts.Account.ImapHost, opts.Account.ImapPort,
			opts.Account.UseTLS, opts.Account.Email)
	}

	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return nil, errtrace.Wrap(err, "dial imap")
	}
	defer mc.Close()
	if v {
		logger.Printf("[%s] connected and logged in (took %s)", alias, time.Since(start).Round(time.Millisecond))
	}

	stats, err := mc.SelectInbox()
	if err != nil {
		return nil, errtrace.Wrap(err, "select inbox")
	}
	if v {
		logger.Printf("[%s] mailbox %q stats: messages=%d recent=%d unseen=%d uidNext=%d uidValidity=%d",
			alias, stats.Name, stats.Messages, stats.Recent, stats.Unseen, stats.UidNext, stats.UidValidity)
	}

	// First-run baseline: don't replay history. Snapshot UIDNEXT-1 and exit.
	if ws.LastUid == 0 {
		baseline := uint32(0)
		if stats.UidNext > 0 {
			baseline = stats.UidNext - 1
		}
		ws.LastUid = baseline
		if err := opts.Store.UpsertWatchState(ctx, ws); err != nil {
			return &stats, errtrace.Wrap(err, "baseline watch state")
		}
		// Always log baseline — it's a one-time meaningful event.
		logger.Printf("[%s] baseline set to UID=%d (skipping history). Watching for UID > %d.",
			alias, baseline, baseline)
		return &stats, nil
	}

	if v {
		logger.Printf("[%s] fetching messages with UID > %d (server uidNext=%d)",
			alias, ws.LastUid, stats.UidNext)
	}
	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return &stats, errtrace.Wrap(err, "fetch since")
	}
	if len(msgs) == 0 {
		if v {
			logger.Printf("[%s] no new messages (poll completed in %s)",
				alias, time.Since(start).Round(time.Millisecond))
		}
		return &stats, nil
	}

	// New mail arrived — always announce in both modes.
	logger.Printf("[%s] ✉ %d new message(s)", alias, len(msgs))

	for _, m := range msgs {
		if err := ctx.Err(); err != nil {
			return &stats, errtrace.Wrap(err, "context cancelled mid-batch")
		}
		path, err := mailclient.SaveRaw(alias, m)
		if err != nil {
			logger.Printf("[%s] ✗ save raw uid=%d:\n%s", alias, m.Uid, errtrace.Format(err))
		}
		row := &store.Email{
			Alias:      alias,
			MessageId:  m.MessageId,
			Uid:        m.Uid,
			FromAddr:   m.From,
			ToAddr:     m.To,
			CcAddr:     m.Cc,
			Subject:    m.Subject,
			BodyText:   m.BodyText,
			BodyHtml:   m.BodyHtml,
			ReceivedAt: m.ReceivedAt,
			FilePath:   path,
		}
		emailId, inserted, err := opts.Store.UpsertEmail(ctx, row)
		if err != nil {
			logger.Printf("[%s] ✗ upsert uid=%d:\n%s", alias, m.Uid, errtrace.Format(err))
			continue
		}
		if inserted {
			logger.Printf("    uid=%d  from=%s  subj=%q", m.Uid, shortAddr(m.From), m.Subject)
			if v {
				logger.Printf("    saved → %s", path)
			}
		} else if v {
			logger.Printf("    uid=%d duplicate (already in DB) subj=%q", m.Uid, m.Subject)
		}

		// Rules → URLs → browser. Always log the per-rule outcome (even in
		// quiet mode) so the user can see why "new mail arrived but nothing
		// opened" — this was the #1 source of confusion before.
		if opts.Engine != nil {
			matches, traces := opts.Engine.Evaluate(m), []rules.RuleTrace(nil)
			matches, traces = opts.Engine.EvaluateWithTrace(m)
			if len(traces) == 0 {
				logger.Printf("    rules: 0 enabled rules — nothing to evaluate (add one in data/config.json)")
			} else {
				for _, t := range traces {
					if len(t.UrlsFound) > 0 {
						logger.Printf("    rules: ✓ %q → %d url(s)", t.RuleName, len(t.UrlsFound))
					} else {
						logger.Printf("    rules: ✗ %q → %s", t.RuleName, t.Reason)
					}
				}
			}
			if opts.Launcher == nil && len(matches) > 0 {
				logger.Printf("    ⚠ %d URL(s) matched but no browser launcher attached", len(matches))
			}
			for _, match := range matches {
				if opts.Launcher == nil {
					break
				}
				inserted, err := opts.Store.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url)
				if err != nil {
					logger.Printf("    ✗ dedup url:\n%s", errtrace.Format(err))
					continue
				}
				if !inserted {
					logger.Printf("    ↻ skip %s (already opened previously for this email)", match.Url)
					continue
				}
				logger.Printf("    → opening in incognito: %s (rule=%s)", match.Url, match.RuleName)
				if err := opts.Launcher.Open(match.Url); err != nil {
					logger.Printf("    ✗ browser launch failed for %s:\n%s", match.Url, errtrace.Format(err))
					continue
				}
				logger.Printf("    ✓ launched")
			}
		}

		if m.Uid > ws.LastUid {
			ws.LastUid = m.Uid
			ws.LastSubject = m.Subject
			ws.LastReceivedAt = m.ReceivedAt
		}
	}

	if err := opts.Store.UpsertWatchState(ctx, ws); err != nil {
		return &stats, errtrace.Wrap(err, "update watch state")
	}
	if v {
		logger.Printf("[%s] poll complete: processed=%d newLastUid=%d total=%s",
			alias, len(msgs), ws.LastUid, time.Since(start).Round(time.Millisecond))
	}
	return &stats, nil
}

// shortAddr trims the long `"Display Name" <addr@host>` form down to just
// the angle-addr if present, else returns input. Keeps idle-poll new-mail
// log lines compact.
func shortAddr(s string) string {
	if i := indexByte(s, '<'); i >= 0 {
		if j := indexByte(s[i:], '>'); j > 0 {
			return s[i+1 : i+j]
		}
	}
	return s
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
