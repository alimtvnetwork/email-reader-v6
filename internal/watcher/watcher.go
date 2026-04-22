// Package watcher runs the polling loop for a single account: connect to IMAP,
// fetch new messages by UID, persist them to SQLite + filesystem, evaluate
// rules, and open matching URLs in an incognito browser. Stops cleanly on
// context cancellation (Ctrl+C in the CLI).
package watcher

import (
	"context"
	"io"
	"log"
	"strings"
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
	Bus         *Bus        // optional; structured event stream for the UI. CLI leaves nil.
}

// ts returns the compact HH:MM:SS prefix we put on every line. The CLI
// configures the logger with no flags so we have full control of formatting.
func ts() string { return time.Now().Format("15:04:05") }

// truncURL keeps URL log lines readable when the query string is huge
// (Lovable verify links can be 200+ chars).
func truncURL(u string) string {
	const max = 90
	if len(u) <= max {
		return u
	}
	return u[:max-1] + "…"
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

	// ── Startup banner ────────────────────────────────────────────────
	logger.Printf("┌─ email-read · watching [%s] ────────────────────", alias)
	logger.Printf("│  account : %s", opts.Account.Email)
	logger.Printf("│  server  : %s:%d (tls=%v)", opts.Account.ImapHost, opts.Account.ImapPort, opts.Account.UseTLS)
	logger.Printf("│  poll    : %s   (Ctrl+C to stop)", poll)

	if opts.Engine == nil {
		logger.Printf("│  rules   : ⚠ engine not attached — saved only, no opens")
	} else {
		n := opts.Engine.RuleCount()
		if n == 0 {
			logger.Printf("│  rules   : ⚠ 0 enabled — add one in data/config.json")
		} else {
			logger.Printf("│  rules   : %d enabled", n)
		}
	}

	if opts.Launcher == nil {
		logger.Printf("│  browser : ⚠ no launcher — URLs matched but never opened")
	} else if path, err := opts.Launcher.Path(); err != nil {
		logger.Printf("│  browser : ⚠ not resolved (see error below)")
		logger.Printf("%s", errtrace.Format(err))
	} else {
		logger.Printf("│  browser : %s  (incognito %s)", shortPath(path), opts.Launcher.IncognitoArg())
	}
	mode := "quiet"
	if opts.Verbose {
		mode = "verbose"
	}
	logger.Printf("│  mode    : %s", mode)
	logger.Printf("└──────────────────────────────────────────────────")

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
			logger.Printf("")
			logger.Printf("%s  ■ stopped — ran %s, %d polls",
				ts(), time.Since(startedAt).Round(time.Second), pollCount)
			return nil
		case <-tick.C:
			pollCount++
			stats, err := pollOnce(ctx, opts, logger)
			if err != nil {
				msg := err.Error()
				if opts.Verbose || msg != lastError {
					logger.Printf("")
					logger.Printf("%s  ✗ [%s] poll error", ts(), alias)
					for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
						logger.Printf("        %s", line)
					}
					lastError = msg
				}
			} else if stats != nil {
				lastError = ""
				if havePrev {
					if stats.UidValidity != prevUidValidity {
						logger.Printf("")
						logger.Printf("%s  ⚠ [%s] UIDVALIDITY changed %d → %d (mailbox reset on server, baseline will reset)",
							ts(), alias, prevUidValidity, stats.UidValidity)
					}
					if opts.Verbose && (stats.Messages != prevMessages || stats.UidNext != prevUidNext) {
						logger.Printf("%s  · [%s] server state: messages %d→%d, uidNext %d→%d",
							ts(), alias, prevMessages, stats.Messages, prevUidNext, stats.UidNext)
					}
				}
				prevMessages = stats.Messages
				prevUidNext = stats.UidNext
				prevUidValidity = stats.UidValidity
				havePrev = true

				if pollCount%heartbeatEvery == 0 {
					logger.Printf("")
					logger.Printf("%s  ♥ [%s] alive — %s, %d polls · mailbox messages=%d uidNext=%d (no new mail)",
						ts(), alias, time.Since(startedAt).Round(time.Second), pollCount, stats.Messages, stats.UidNext)
				}
			}
			tick.Reset(poll)
		}
	}
}

// shortPath strips a long /Applications/.../Google Chrome path to just the
// app name so the startup banner stays narrow.
func shortPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

// pollOnce performs a single connect → fetch → persist → match → open cycle.
// In quiet mode (opts.Verbose=false) it stays silent unless something
// noteworthy happens (new mail, errors, baseline being set).
func pollOnce(ctx context.Context, opts Options, logger *log.Logger) (*mailclient.MailboxStats, error) {
	start := time.Now()
	alias := opts.Account.Alias
	v := opts.Verbose

	if v {
		logger.Printf("%s  · [%s] poll start", ts(), alias)
	}

	ws, err := opts.Store.GetWatchState(ctx, alias)
	if err != nil {
		return nil, errtrace.Wrap(err, "get watch state")
	}
	if v {
		logger.Printf("%s  · [%s] watch state: lastUid=%d lastSubject=%q",
			ts(), alias, ws.LastUid, ws.LastSubject)
		logger.Printf("%s  · [%s] dial %s:%d (tls=%v) as %s",
			ts(), alias, opts.Account.ImapHost, opts.Account.ImapPort,
			opts.Account.UseTLS, opts.Account.Email)
	}

	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return nil, errtrace.Wrap(err, "dial imap")
	}
	defer mc.Close()
	if v {
		logger.Printf("%s  · [%s] connected (%s)", ts(), alias, time.Since(start).Round(time.Millisecond))
	}

	stats, err := mc.SelectInbox()
	if err != nil {
		return nil, errtrace.Wrap(err, "select inbox")
	}
	if v {
		logger.Printf("%s  · [%s] mailbox %q messages=%d unseen=%d uidNext=%d uidValidity=%d",
			ts(), alias, stats.Name, stats.Messages, stats.Unseen, stats.UidNext, stats.UidValidity)
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
		logger.Printf("")
		logger.Printf("%s  ⚑ [%s] baseline set: UID=%d (history skipped, watching for UID > %d)",
			ts(), alias, baseline, baseline)
		return &stats, nil
	}

	if v {
		logger.Printf("%s  · [%s] fetch UID > %d (server uidNext=%d)",
			ts(), alias, ws.LastUid, stats.UidNext)
	}
	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return &stats, errtrace.Wrap(err, "fetch since")
	}
	if len(msgs) == 0 {
		if v {
			logger.Printf("%s  · [%s] no new messages (%s)",
				ts(), alias, time.Since(start).Round(time.Millisecond))
		}
		return &stats, nil
	}

	// 10-second cooldown after we open URL(s) for a message.
	const postOpenCooldown = 10 * time.Second
	openedAny := false

	for i, m := range msgs {
		if err := ctx.Err(); err != nil {
			return &stats, errtrace.Wrap(err, "context cancelled mid-batch")
		}
		// Apply the 10s gap BETWEEN URL-bearing messages in the same batch.
		if openedAny && i > 0 {
			logger.Printf("        ⏳ waiting 10s before next message in batch…")
			if err := sleepCtx(ctx, postOpenCooldown); err != nil {
				return &stats, errtrace.Wrap(err, "cooldown between messages")
			}
			openedAny = false
		}

		// ── New-message block ───────────────────────────────────
		logger.Printf("")
		logger.Printf("%s  ✉ [%s] new mail · uid=%d", ts(), alias, m.Uid)
		logger.Printf("        from    : %s", shortAddr(m.From))
		logger.Printf("        subject : %s", m.Subject)

		path, saveErr := mailclient.SaveRaw(alias, m)
		if saveErr != nil {
			logger.Printf("        ✗ save failed:")
			for _, line := range strings.Split(strings.TrimRight(errtrace.Format(saveErr), "\n"), "\n") {
				logger.Printf("            %s", line)
			}
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
			logger.Printf("        ✗ db upsert failed:")
			for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
				logger.Printf("            %s", line)
			}
			continue
		}
		if inserted && path != "" {
			logger.Printf("        saved   : %s", path)
		} else if !inserted {
			logger.Printf("        (already in database — duplicate)")
			if !v {
				continue
			}
		}

		// ── Rules section ───────────────────────────────────────
		urlsLaunched := 0
		if opts.Engine != nil {
			matches, traces := opts.Engine.EvaluateWithTrace(m)
			if len(traces) == 0 {
				logger.Printf("        rules   : 0 enabled — nothing to evaluate")
			} else {
				for _, t := range traces {
					if len(t.UrlsFound) > 0 {
						logger.Printf("        rules   : ✓ %s → %d url(s)", t.RuleName, len(t.UrlsFound))
					} else {
						logger.Printf("        rules   : ✗ %s → %s", t.RuleName, t.Reason)
					}
				}
			}
			if opts.Launcher == nil && len(matches) > 0 {
				logger.Printf("        ⚠ %d URL(s) matched but no browser launcher attached", len(matches))
			}
			for _, match := range matches {
				if opts.Launcher == nil {
					break
				}
				inserted, err := opts.Store.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url)
				if err != nil {
					logger.Printf("        ✗ dedup url failed:")
					for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
						logger.Printf("            %s", line)
					}
					continue
				}
				if !inserted {
					logger.Printf("        ↻ skip (already opened): %s", truncURL(match.Url))
					continue
				}
				logger.Printf("        → open  : %s", truncURL(match.Url))
				if err := opts.Launcher.Open(match.Url); err != nil {
					logger.Printf("        ✗ launch failed:")
					for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
						logger.Printf("            %s", line)
					}
					continue
				}
				logger.Printf("                   ✓ launched in incognito")
				urlsLaunched++
			}
		}

		if urlsLaunched > 0 {
			openedAny = true
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

	// 10s gap before the NEXT poll cycle, but only if we actually opened
	// a URL during this batch. Otherwise we'd just slow down idle polling.
	if openedAny {
		logger.Printf("        ⏳ waiting 10s before next poll cycle…")
		if err := sleepCtx(ctx, postOpenCooldown); err != nil {
			return &stats, errtrace.Wrap(err, "cooldown before next poll")
		}
	}

	if v {
		logger.Printf("%s  · [%s] poll done: processed=%d newLastUid=%d (%s)",
			ts(), alias, len(msgs), ws.LastUid, time.Since(start).Round(time.Millisecond))
	}
	return &stats, nil
}

// sleepCtx pauses for d but returns early if ctx is cancelled. The
// returned error is non-nil only when ctx is cancelled, so callers can wrap
// it with errtrace and bubble up cleanly on Ctrl+C.
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
