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
	ws, err := opts.Store.GetWatchState(ctx, opts.Account.Alias)
	if err != nil {
		return fmt.Errorf("get watch state: %w", err)
	}

	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return err
	}
	defer mc.Close()

	uidNext, err := mc.SelectInbox()
	if err != nil {
		return err
	}

	// First-run baseline: don't replay history. Snapshot UIDNEXT-1 and exit.
	if ws.LastUid == 0 {
		baseline := uint32(0)
		if uidNext > 0 {
			baseline = uidNext - 1
		}
		ws.LastUid = baseline
		if err := opts.Store.UpsertWatchState(ctx, ws); err != nil {
			return fmt.Errorf("baseline watch state: %w", err)
		}
		logger.Printf("[%s] baseline set to UID=%d (skipping history)",
			opts.Account.Alias, baseline)
		return nil
	}

	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	logger.Printf("[%s] fetched %d new message(s)", opts.Account.Alias, len(msgs))

	for _, m := range msgs {
		if err := ctx.Err(); err != nil {
			return err
		}
		path, err := mailclient.SaveRaw(opts.Account.Alias, m)
		if err != nil {
			logger.Printf("[%s] save raw uid=%d: %v", opts.Account.Alias, m.Uid, err)
		}
		row := &store.Email{
			Alias:      opts.Account.Alias,
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
			logger.Printf("[%s] upsert uid=%d: %v", opts.Account.Alias, m.Uid, err)
			continue
		}
		if inserted {
			logger.Printf("[%s] saved uid=%d subj=%q", opts.Account.Alias, m.Uid, m.Subject)
		}

		// Rules → URLs → browser
		if opts.Engine != nil && opts.Launcher != nil {
			for _, match := range opts.Engine.Evaluate(m) {
				inserted, err := opts.Store.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url)
				if err != nil {
					logger.Printf("[%s] dedup url: %v", opts.Account.Alias, err)
					continue
				}
				if !inserted {
					continue // already opened
				}
				if err := opts.Launcher.Open(match.Url); err != nil {
					logger.Printf("[%s] open url %s: %v", opts.Account.Alias, match.Url, err)
					continue
				}
				logger.Printf("[%s] opened %s (rule=%s)", opts.Account.Alias, match.Url, match.RuleName)
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
	return nil
}
