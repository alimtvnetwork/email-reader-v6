// Package watcher runs the polling loop for a single account: connect to IMAP,
// fetch new messages by UID, persist them to SQLite + filesystem, evaluate
// rules, and open matching URLs in an incognito browser. Stops cleanly on
// context cancellation (Ctrl+C in the CLI).
package watcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
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

	for {
		select {
		case <-ctx.Done():
			logger.Printf("[%s] watcher stopped: %v", opts.Account.Alias, ctx.Err())
			return nil
		case <-tick.C:
			if err := pollOnce(ctx, opts, logger); err != nil {
				logger.Printf("[%s] poll error: %v", opts.Account.Alias, err)
			}
			tick.Reset(poll)
		}
	}
}

// pollOnce performs a single connect → fetch → persist → match → open cycle.
func pollOnce(ctx context.Context, opts Options, logger *log.Logger) error {
	start := time.Now()
	alias := opts.Account.Alias
	logger.Printf("[%s] poll start", alias)

	ws, err := opts.Store.GetWatchState(ctx, alias)
	if err != nil {
		return fmt.Errorf("get watch state: %w", err)
	}
	logger.Printf("[%s] watch state loaded: lastUid=%d lastSubject=%q",
		alias, ws.LastUid, ws.LastSubject)

	logger.Printf("[%s] dialing IMAP %s:%d (tls=%v) as %s",
		alias, opts.Account.ImapHost, opts.Account.ImapPort,
		opts.Account.UseTLS, opts.Account.Email)
	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return err
	}
	defer mc.Close()
	logger.Printf("[%s] connected and logged in (took %s)", alias, time.Since(start).Round(time.Millisecond))

	stats, err := mc.SelectInbox()
	if err != nil {
		return err
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
			return fmt.Errorf("baseline watch state: %w", err)
		}
		logger.Printf("[%s] baseline set to UID=%d (skipping history). New mail with UID > %d will be processed.",
			alias, baseline, baseline)
		return nil
	}

	logger.Printf("[%s] fetching messages with UID > %d (server uidNext=%d)",
		alias, ws.LastUid, stats.UidNext)
	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		logger.Printf("[%s] no new messages (poll completed in %s)",
			alias, time.Since(start).Round(time.Millisecond))
		return nil
	}
	logger.Printf("[%s] fetched %d new message(s)", alias, len(msgs))

	for _, m := range msgs {
		if err := ctx.Err(); err != nil {
			return err
		}
		path, err := mailclient.SaveRaw(alias, m)
		if err != nil {
			logger.Printf("[%s] save raw uid=%d: %v", alias, m.Uid, err)
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
			logger.Printf("[%s] upsert uid=%d: %v", alias, m.Uid, err)
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
					logger.Printf("[%s] dedup url: %v", alias, err)
					continue
				}
				if !inserted {
					logger.Printf("[%s] skipping url %s (already opened)", alias, match.Url)
					continue
				}
				if err := opts.Launcher.Open(match.Url); err != nil {
					logger.Printf("[%s] open url %s: %v", alias, match.Url, err)
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
		return fmt.Errorf("update watch state: %w", err)
	}
	logger.Printf("[%s] poll complete: processed=%d newLastUid=%d total=%s",
		alias, len(msgs), ws.LastUid, time.Since(start).Round(time.Millisecond))
	return nil
}
