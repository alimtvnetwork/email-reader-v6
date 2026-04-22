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
}

// Run blocks until ctx is cancelled. It logs progress and tolerates
// transient IMAP errors by reconnecting on the next tick.
func Run(ctx context.Context, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	poll := time.Duration(opts.PollSeconds) * time.Second
	if poll <= 0 {
		poll = 3 * time.Second
	}

	logger.Printf("[%s] starting watcher (poll=%s, host=%s)",
		opts.Account.Alias, poll, opts.Account.ImapHost)

	// One iteration immediately, then every `poll`.
	tick := time.NewTimer(0)
	defer tick.Stop()

	// Cross-poll diagnostic state: detect server-side changes between polls.
	// If `messages` / `uidNext` / `uidValidity` never change, the mailbox is
	// genuinely receiving no mail — i.e., the problem is upstream (MX, spam,
	// wrong recipient address), not in this watcher.
	var (
		prevMessages    uint32
		prevUidNext     uint32
		prevUidValidity uint32
		pollCount       int
		startedAt       = time.Now()
		havePrev        bool
	)
	const heartbeatEvery = 20 // ~ every 20 polls (1 min at 3s poll)

	for {
		select {
		case <-ctx.Done():
			logger.Printf("[%s] watcher stopped: %v", opts.Account.Alias, ctx.Err())
			return nil
		case <-tick.C:
			pollCount++
			stats, err := pollOnce(ctx, opts, logger)
			if err != nil {
				logger.Printf("[%s] poll error:\n%s", opts.Account.Alias, errtrace.Format(err))
			} else if stats != nil {
				// Diagnostic: compare server-reported mailbox state vs previous poll.
				if havePrev {
					if stats.UidValidity != prevUidValidity {
						logger.Printf("[%s] ⚠ UIDVALIDITY CHANGED %d -> %d: mailbox was recreated/reset on server. Baseline will reset.",
							opts.Account.Alias, prevUidValidity, stats.UidValidity)
					}
					if stats.Messages != prevMessages || stats.UidNext != prevUidNext {
						logger.Printf("[%s] ✓ server state changed: messages %d->%d, uidNext %d->%d (new mail likely arrived)",
							opts.Account.Alias, prevMessages, stats.Messages, prevUidNext, stats.UidNext)
					}
				}
				prevMessages = stats.Messages
				prevUidNext = stats.UidNext
				prevUidValidity = stats.UidValidity
				havePrev = true

				// Periodic heartbeat: makes "nothing arriving" obvious vs. "watcher hung".
				if pollCount%heartbeatEvery == 0 {
					logger.Printf("[%s] ♥ heartbeat: watching for %s, %d polls completed, server still reports messages=%d uidNext=%d. If you expect new mail and don't see it: check MX records, spam folder, or send a test from webmail.",
						opts.Account.Alias, time.Since(startedAt).Round(time.Second), pollCount, stats.Messages, stats.UidNext)
				}
			}
			tick.Reset(poll)
		}
	}
}

// pollOnce performs a single connect → fetch → persist → match → open cycle.
// Returns the MailboxStats from the server (or nil on early error) so the
// caller can do cross-poll diagnostics like "did messages count change?".
func pollOnce(ctx context.Context, opts Options, logger *log.Logger) (*mailclient.MailboxStats, error) {
	start := time.Now()
	alias := opts.Account.Alias
	logger.Printf("[%s] poll start", alias)

	ws, err := opts.Store.GetWatchState(ctx, alias)
	if err != nil {
		return nil, errtrace.Wrap(err, "get watch state")
	}
	logger.Printf("[%s] watch state loaded: lastUid=%d lastSubject=%q",
		alias, ws.LastUid, ws.LastSubject)

	logger.Printf("[%s] dialing IMAP %s:%d (tls=%v) as %s",
		alias, opts.Account.ImapHost, opts.Account.ImapPort,
		opts.Account.UseTLS, opts.Account.Email)
	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return nil, errtrace.Wrap(err, "dial imap")
	}
	defer mc.Close()
	logger.Printf("[%s] connected and logged in (took %s)", alias, time.Since(start).Round(time.Millisecond))

	stats, err := mc.SelectInbox()
	if err != nil {
		return nil, errtrace.Wrap(err, "select inbox")
	}
	logger.Printf("[%s] mailbox %q stats: messages=%d recent=%d unseen=%d uidNext=%d uidValidity=%d",
		alias, stats.Name, stats.Messages, stats.Recent, stats.Unseen, stats.UidNext, stats.UidValidity)

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
		logger.Printf("[%s] baseline set to UID=%d (skipping history). New mail with UID > %d will be processed.",
			alias, baseline, baseline)
		return &stats, nil
	}

	logger.Printf("[%s] fetching messages with UID > %d (server uidNext=%d)",
		alias, ws.LastUid, stats.UidNext)
	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return &stats, errtrace.Wrap(err, "fetch since")
	}
	if len(msgs) == 0 {
		logger.Printf("[%s] no new messages (poll completed in %s)",
			alias, time.Since(start).Round(time.Millisecond))
		return &stats, nil
	}
	logger.Printf("[%s] fetched %d new message(s)", alias, len(msgs))

	for _, m := range msgs {
		if err := ctx.Err(); err != nil {
			return &stats, errtrace.Wrap(err, "context cancelled mid-batch")
		}
		path, err := mailclient.SaveRaw(alias, m)
		if err != nil {
			logger.Printf("[%s] save raw uid=%d:\n%s", alias, m.Uid, errtrace.Format(err))
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
			logger.Printf("[%s] upsert uid=%d:\n%s", alias, m.Uid, errtrace.Format(err))
			continue
		}
		if inserted {
			logger.Printf("[%s] saved uid=%d from=%q subj=%q file=%s",
				alias, m.Uid, m.From, m.Subject, path)
		} else {
			logger.Printf("[%s] duplicate uid=%d (already in DB) subj=%q", alias, m.Uid, m.Subject)
		}

		// Rules → URLs → browser
		if opts.Engine != nil && opts.Launcher != nil {
			matches := opts.Engine.Evaluate(m)
			logger.Printf("[%s] uid=%d matched %d rule URL(s)", alias, m.Uid, len(matches))
			for _, match := range matches {
				inserted, err := opts.Store.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url)
				if err != nil {
					logger.Printf("[%s] dedup url:\n%s", alias, errtrace.Format(err))
					continue
				}
				if !inserted {
					logger.Printf("[%s] skipping url %s (already opened)", alias, match.Url)
					continue
				}
				if err := opts.Launcher.Open(match.Url); err != nil {
					logger.Printf("[%s] open url %s:\n%s", alias, match.Url, errtrace.Format(err))
					continue
				}
				logger.Printf("[%s] opened %s (rule=%s)", alias, match.Url, match.RuleName)
			}
		}

		// Track highest UID seen.
		if m.Uid > ws.LastUid {
			ws.LastUid = m.Uid
			ws.LastSubject = m.Subject
			ws.LastReceivedAt = m.ReceivedAt
		}
	}

	if err := opts.Store.UpsertWatchState(ctx, ws); err != nil {
		return &stats, errtrace.Wrap(err, "update watch state")
	}
	logger.Printf("[%s] poll complete: processed=%d newLastUid=%d total=%s",
		alias, len(msgs), ws.LastUid, time.Since(start).Round(time.Millisecond))
	return &stats, nil
}
