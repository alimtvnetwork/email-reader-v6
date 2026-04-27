// Package watcher runs the polling loop for a single account: connect to IMAP,
// fetch new messages by UID, persist them to SQLite + filesystem, evaluate
// rules, and open matching URLs in an incognito browser. Stops cleanly on
// context cancellation (Ctrl+C in the CLI).
package watcher

import (
	"context"
	"io"
	"log"
	"math/rand"
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
	// PollSecondsCh, when non-nil, lets a Settings consumer push live
	// PollSeconds updates. Values are clamped to 1..60 and applied on the
	// NEXT loop iteration (in-flight polls are not interrupted). Per
	// spec/21-app/02-features/07-settings/01-backend.md §8 (CF-W1).
	PollSecondsCh <-chan int
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
// logStartupBanner prints the multi-line startup card describing the account,
// server, poll interval, rules, browser launcher, and verbosity mode.
func logStartupBanner(logger *log.Logger, opts Options, poll time.Duration) {
	alias := opts.Account.Alias
	logger.Printf("┌─ email-read · watching [%s] ────────────────────", alias)
	logger.Printf("│  account : %s", opts.Account.Email)
	logger.Printf("│  server  : %s:%d (tls=%v)", opts.Account.ImapHost, opts.Account.ImapPort, opts.Account.UseTLS)
	logger.Printf("│  poll    : %s   (Ctrl+C to stop)", poll)
	logBannerRules(logger, opts)
	logBannerBrowser(logger, opts)
	mode := "quiet"
	if opts.Verbose {
		mode = "verbose"
	}
	logger.Printf("│  mode    : %s", mode)
	logger.Printf("└──────────────────────────────────────────────────")
}

func logBannerRules(logger *log.Logger, opts Options) {
	if opts.Engine == nil {
		logger.Printf("│  rules   : ⚠ engine not attached — saved only, no opens")
		return
	}
	n := opts.Engine.RuleCount()
	if n == 0 {
		logger.Printf("│  rules   : ⚠ 0 enabled — add one in data/config.json")
		return
	}
	logger.Printf("│  rules   : %d enabled", n)
}

func logBannerBrowser(logger *log.Logger, opts Options) {
	if opts.Launcher == nil {
		logger.Printf("│  browser : ⚠ no launcher — URLs matched but never opened")
		return
	}
	path, err := opts.Launcher.Path()
	if err != nil {
		logger.Printf("│  browser : ⚠ not resolved (see error below)")
		logger.Printf("%s", errtrace.Format(err))
		return
	}
	// AC-SX-05: never log the IncognitoArg value at any level.
	logger.Printf("│  browser : %s  (incognito %s)", shortPath(path), redactIncog(opts.Launcher.IncognitoArg()))
}

// pollState carries cross-poll diagnostic state for a watcher loop.
type pollState struct {
	prevMessages, prevUidNext, prevUidValidity uint32
	pollCount                                  int
	startedAt                                  time.Time
	havePrev                                   bool
	lastError                                  string
	// consecutiveErrors counts EventPollError in a row. Reset to 0 on any
	// successful poll. Drives exponential backoff with jitter (CF-W-BACKOFF).
	consecutiveErrors int
}

const heartbeatEvery = 60 // ~ every 60 polls (~3 min at 3s poll)

// logPollError emits a poll-error block, de-duping repeated identical errors
// when not in verbose mode.
func logPollError(logger *log.Logger, opts Options, st *pollState, err error) {
	msg := err.Error()
	if opts.Verbose || msg != st.lastError {
		logger.Printf("")
		logger.Printf("%s  ✗ [%s] poll error", ts(), opts.Account.Alias)
		for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
			logger.Printf("        %s", line)
		}
		st.lastError = msg
	}
	st.consecutiveErrors++
	opts.Bus.Publish(Event{Kind: EventPollError, Alias: opts.Account.Alias, Err: err})
}

// handlePollOK processes a successful poll: emits UIDVALIDITY/state-change
// notes, publishes the OK event, and emits a periodic heartbeat.
func handlePollOK(logger *log.Logger, opts Options, st *pollState, stats *mailclient.MailboxStats) {
	alias := opts.Account.Alias
	st.lastError = ""
	st.consecutiveErrors = 0
	if st.havePrev {
		if stats.UidValidity != st.prevUidValidity {
			logger.Printf("")
			logger.Printf("%s  ⚠ [%s] UIDVALIDITY changed %d → %d (mailbox reset on server, baseline will reset)",
				ts(), alias, st.prevUidValidity, stats.UidValidity)
			opts.Bus.Publish(Event{Kind: EventUidValReset, Alias: alias, Stats: stats})
		}
		if opts.Verbose && (stats.Messages != st.prevMessages || stats.UidNext != st.prevUidNext) {
			logger.Printf("%s  · [%s] server state: messages %d→%d, uidNext %d→%d",
				ts(), alias, st.prevMessages, stats.Messages, st.prevUidNext, stats.UidNext)
		}
	}
	st.prevMessages, st.prevUidNext, st.prevUidValidity = stats.Messages, stats.UidNext, stats.UidValidity
	st.havePrev = true
	opts.Bus.Publish(Event{Kind: EventPollOK, Alias: alias, Stats: stats})
	if st.pollCount%heartbeatEvery == 0 {
		logger.Printf("")
		logger.Printf("%s  ♥ [%s] alive — %s, %d polls · mailbox messages=%d uidNext=%d (no new mail)",
			ts(), alias, time.Since(st.startedAt).Round(time.Second), st.pollCount, stats.Messages, stats.UidNext)
		opts.Bus.Publish(Event{Kind: EventHeartbeat, Alias: alias, Stats: stats})
	}
}

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
	logStartupBanner(logger, opts, poll)
	opts.Bus.Publish(Event{Kind: EventStarted, Alias: alias})
	tick := time.NewTimer(0)
	defer tick.Stop()
	st := &pollState{startedAt: time.Now()}
	return runLoop(ctx, opts, logger, tick, st, &poll)
}

// runLoop owns the main select loop. Extracted so Run stays ≤15 statements.
// `poll` is held by pointer so live-reload (PollSecondsCh) can mutate it
// between iterations without interrupting an in-flight poll.
func runLoop(ctx context.Context, opts Options, logger *log.Logger, tick *time.Timer, st *pollState, poll *time.Duration) error {
	alias := opts.Account.Alias
	for {
		select {
		case <-ctx.Done():
			logger.Printf("")
			logger.Printf("%s  ■ stopped — ran %s, %d polls",
				ts(), time.Since(st.startedAt).Round(time.Second), st.pollCount)
			opts.Bus.Publish(Event{Kind: EventStopped, Alias: alias})
			return nil
		case secs, ok := <-opts.PollSecondsCh:
			if ok {
				applyPollReload(logger, alias, poll, secs)
			}
		case <-tick.C:
			st.pollCount++
			stats, err := pollOnce(ctx, opts, logger)
			if err != nil {
				logPollError(logger, opts, st, err)
				if berr := opts.Store.BumpConsecutiveFailures(ctx, alias); berr != nil {
					// Best-effort: log but keep polling. The
					// counter will resync on the next outcome.
					logger.Printf("%s  ! [%s] bump consecutive failures: %v", ts(), alias, berr)
				}
			} else if stats != nil {
				handlePollOK(logger, opts, st, stats)
				if rerr := opts.Store.ResetConsecutiveFailures(ctx, alias); rerr != nil {
					logger.Printf("%s  ! [%s] reset consecutive failures: %v", ts(), alias, rerr)
				}
			}
			tick.Reset(nextDelay(*poll, st, logger, alias))
		}
	}
}

// nextDelay picks the wait until the next poll attempt. Happy path returns
// the configured cadence; on a streak of errors it returns
// NextPollDelay(base, streak, jitter) and logs the chosen back-off so an
// operator can tell why polls slowed down. Splitting this off keeps
// runLoop ≤15 statements and the math testable without a fake clock.
func nextDelay(base time.Duration, st *pollState, logger *log.Logger, alias string) time.Duration {
	if st.consecutiveErrors == 0 {
		return base
	}
	d := NextPollDelay(base, st.consecutiveErrors, rand.Float64())
	logger.Printf("%s  ⏳ [%s] backing off after %d consecutive error(s): next poll in %s",
		ts(), alias, st.consecutiveErrors, d.Round(100*time.Millisecond))
	return d
}

// applyPollReload clamps and applies a new PollSeconds value, logging the
// change. The new cadence takes effect on the next tick (in-flight polls
// are NOT interrupted) per spec CF-W1.
func applyPollReload(logger *log.Logger, alias string, poll *time.Duration, secs int) {
	if secs < 1 {
		secs = 1
	}
	if secs > 60 {
		secs = 60
	}
	next := time.Duration(secs) * time.Second
	if next == *poll {
		return
	}
	logger.Printf("%s  ⚙ [%s] poll cadence reloaded: %s → %s (applies next tick)",
		ts(), alias, *poll, next)
	*poll = next
}

// shortPath strips a long /Applications/.../Google Chrome path to just the
// app name so the startup banner stays narrow.
func shortPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

// redactIncog returns a constant marker indicating whether an incognito
// argument is configured, without revealing the value. Spec AC-SX-05:
// IncognitoArg never appears in any log line, at any level.
func redactIncog(arg string) string {
	if arg == "" {
		return "<none>"
	}
	return "<set>"
}

// pollOnce performs a single connect → fetch → persist → match → open cycle.
// In quiet mode (opts.Verbose=false) it stays silent unless something
// noteworthy happens (new mail, errors, baseline being set).
// postOpenCooldown is the gap we enforce after opening URL(s) for a
// message — both BETWEEN URL-bearing messages in a batch and BEFORE the
// next poll cycle.
const postOpenCooldown = 10 * time.Second

// logErrLines wraps a multi-line errtrace.Format dump with a leader.
func logErrLines(logger *log.Logger, leader string, err error) {
	logger.Printf("        %s", leader)
	for _, line := range strings.Split(strings.TrimRight(errtrace.Format(err), "\n"), "\n") {
		logger.Printf("            %s", line)
	}
}

// connectAndSelect dials IMAP and selects the inbox. The caller owns the
// returned *mailclient.Client and must Close() it.
func connectAndSelect(opts Options, logger *log.Logger, start time.Time) (*mailclient.Client, mailclient.MailboxStats, error) {
	alias := opts.Account.Alias
	v := opts.Verbose
	if v {
		logger.Printf("%s  · [%s] dial %s:%d (tls=%v) as %s",
			ts(), alias, opts.Account.ImapHost, opts.Account.ImapPort, opts.Account.UseTLS, opts.Account.Email)
	}
	mc, err := mailclient.Dial(opts.Account)
	if err != nil {
		return nil, mailclient.MailboxStats{}, errtrace.WrapCode(err, errtrace.ErrMailDial, "watcher.connectAndSelect: dial imap")
	}
	if v {
		logger.Printf("%s  · [%s] connected (%s)", ts(), alias, time.Since(start).Round(time.Millisecond))
	}
	stats, err := mc.SelectInbox()
	if err != nil {
		mc.Close()
		return nil, mailclient.MailboxStats{}, errtrace.WrapCode(err, errtrace.ErrMailSelectMailbox, "watcher.connectAndSelect: select inbox")
	}
	if v {
		logger.Printf("%s  · [%s] mailbox %q messages=%d unseen=%d uidNext=%d uidValidity=%d",
			ts(), alias, stats.Name, stats.Messages, stats.Unseen, stats.UidNext, stats.UidValidity)
	}
	return mc, stats, nil
}

// handleBaseline performs the first-run baseline snapshot when LastUid==0.
// Returns true when baseline was applied (caller should exit pollOnce).
func handleBaseline(ctx context.Context, opts Options, logger *log.Logger, ws *store.WatchState, stats *mailclient.MailboxStats) (bool, error) {
	if ws.LastUid != 0 {
		return false, nil
	}
	baseline := uint32(0)
	if stats.UidNext > 0 {
		baseline = stats.UidNext - 1
	}
	ws.LastUid = baseline
	if err := opts.Store.UpsertWatchState(ctx, *ws); err != nil {
		return true, errtrace.WrapCode(err, errtrace.ErrDbWatchState, "watcher.handleBaseline: upsert baseline watch state")
	}
	logger.Printf("")
	logger.Printf("%s  ⚑ [%s] baseline set: UID=%d (history skipped, watching for UID > %d)",
		ts(), opts.Account.Alias, baseline, baseline)
	opts.Bus.Publish(Event{Kind: EventBaseline, Alias: opts.Account.Alias, Stats: stats})
	return true, nil
}

// persistMessage writes the raw .eml + DB row. Returns emailId, inserted,
// and the saved file path. A save failure is logged but non-fatal; a DB
// upsert failure is fatal for this message.
func persistMessage(ctx context.Context, opts Options, logger *log.Logger, m *mailclient.Message) (int64, bool, string, error) {
	path, saveErr := mailclient.SaveRaw(opts.Account.Alias, m)
	if saveErr != nil {
		logErrLines(logger, "✗ save failed:", saveErr)
	}
	row := &store.Email{
		Alias: opts.Account.Alias, MessageId: m.MessageId, Uid: m.Uid,
		FromAddr: m.From, ToAddr: m.To, CcAddr: m.Cc, Subject: m.Subject,
		BodyText: m.BodyText, BodyHtml: m.BodyHtml,
		ReceivedAt: m.ReceivedAt, FilePath: path,
	}
	emailId, inserted, err := opts.Store.UpsertEmail(ctx, row)
	return emailId, inserted, path, err
}

// launchMatches opens each rule-matched URL through the launcher with
// per-URL dedup. Returns the count of URLs successfully launched.
func launchMatches(ctx context.Context, opts Options, logger *log.Logger, emailId int64, matches []rules.Match) int {
	alias := opts.Account.Alias
	launched := 0
	if opts.Launcher == nil && len(matches) > 0 {
		logger.Printf("        ⚠ %d URL(s) matched but no browser launcher attached", len(matches))
		return 0
	}
	for _, match := range matches {
		if opts.Launcher == nil {
			break
		}
		inserted, err := opts.Store.RecordOpenedUrl(ctx, emailId, match.RuleName, match.Url)
		if err != nil {
			logErrLines(logger, "✗ dedup url failed:", err)
			continue
		}
		if !inserted {
			logger.Printf("        ↻ skip (already opened): %s", truncURL(match.Url))
			continue
		}
		logger.Printf("        → open  : %s", truncURL(match.Url))
		opts.Bus.Publish(Event{Kind: EventRuleMatch, Alias: alias, RuleName: match.RuleName, Url: match.Url})
		if err := opts.Launcher.Open(match.Url); err != nil {
			logErrLines(logger, "✗ launch failed:", err)
			opts.Bus.Publish(Event{Kind: EventUrlOpened, Alias: alias, RuleName: match.RuleName, Url: match.Url, OpenOK: false, Err: err})
			continue
		}
		logger.Printf("                   ✓ launched in incognito")
		opts.Bus.Publish(Event{Kind: EventUrlOpened, Alias: alias, RuleName: match.RuleName, Url: match.Url, OpenOK: true})
		launched++
	}
	return launched
}

// evaluateRules logs trace lines and launches matched URLs. Returns the
// count launched (caller uses it to decide post-message cooldown).
func evaluateRules(ctx context.Context, opts Options, logger *log.Logger, emailId int64, m *mailclient.Message) int {
	if opts.Engine == nil {
		return 0
	}
	matches, traces := opts.Engine.EvaluateWithTrace(m)
	if len(traces) == 0 {
		logger.Printf("        rules   : 0 enabled — nothing to evaluate")
	}
	for _, t := range traces {
		if len(t.UrlsFound) > 0 {
			logger.Printf("        rules   : ✓ %s → %d url(s)", t.RuleName, len(t.UrlsFound))
		} else {
			logger.Printf("        rules   : ✗ %s → %s", t.RuleName, t.Reason)
		}
	}
	return launchMatches(ctx, opts, logger, emailId, matches)
}

// processMessage handles one fetched message: persist, evaluate rules,
// launch URLs, advance ws cursor. Returns whether any URL was launched
// (used to decide the post-message cooldown).
func processMessage(ctx context.Context, opts Options, logger *log.Logger, ws *store.WatchState, m *mailclient.Message) (openedAny bool, err error) {
	alias := opts.Account.Alias
	logger.Printf("")
	logger.Printf("%s  ✉ [%s] new mail · uid=%d", ts(), alias, m.Uid)
	logger.Printf("        from    : %s", shortAddr(m.From))
	logger.Printf("        subject : %s", m.Subject)
	opts.Bus.Publish(Event{Kind: EventNewMail, Alias: alias, Message: m})

	emailId, inserted, path, perr := persistMessage(ctx, opts, logger, m)
	if perr != nil {
		logErrLines(logger, "✗ db upsert failed:", perr)
		return false, nil
	}
	if inserted && path != "" {
		logger.Printf("        saved   : %s", path)
	} else if !inserted {
		logger.Printf("        (already in database — duplicate)")
		if !opts.Verbose {
			advanceCursor(ws, m)
			return false, nil
		}
	}
	urlsLaunched := evaluateRules(ctx, opts, logger, emailId, m)
	advanceCursor(ws, m)
	return urlsLaunched > 0, nil
}

func advanceCursor(ws *store.WatchState, m *mailclient.Message) {
	if m.Uid > ws.LastUid {
		ws.LastUid = m.Uid
		ws.LastSubject = m.Subject
		ws.LastReceivedAt = m.ReceivedAt
	}
}

// finalizeBatch persists the advanced watch state and applies the
// post-cycle cooldown when any URL was launched in this batch.
func finalizeBatch(ctx context.Context, opts Options, logger *log.Logger, ws store.WatchState, openedAny bool) error {
	if err := opts.Store.UpsertWatchState(ctx, ws); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrDbWatchState, "watcher.finalizeBatch: update watch state")
	}
	if !openedAny {
		return nil
	}
	logger.Printf("        ⏳ waiting 10s before next poll cycle…")
	if err := sleepCtx(ctx, postOpenCooldown); err != nil {
		return errtrace.WrapCode(err, errtrace.ErrCoreContextCancelled, "watcher.finalizeBatch: cooldown before next poll")
	}
	return nil
}

// fetchAndCheckEmpty fetches messages > ws.LastUid and logs the empty-batch
// case. Returns (msgs, true) when the caller should exit early with no work.
func fetchAndCheckEmpty(opts Options, logger *log.Logger, mc *mailclient.Client, ws store.WatchState, stats mailclient.MailboxStats, start time.Time) ([]*mailclient.Message, bool, error) {
	alias, v := opts.Account.Alias, opts.Verbose
	if v {
		logger.Printf("%s  · [%s] fetch UID > %d (server uidNext=%d)", ts(), alias, ws.LastUid, stats.UidNext)
	}
	msgs, err := mc.FetchSince(ws.LastUid)
	if err != nil {
		return nil, false, errtrace.WrapCode(err, errtrace.ErrMailFetchUid, "watcher.fetchAndCheckEmpty: FETCH since")
	}
	if len(msgs) == 0 {
		if v {
			logger.Printf("%s  · [%s] no new messages (%s)", ts(), alias, time.Since(start).Round(time.Millisecond))
		}
		return nil, true, nil
	}
	return msgs, false, nil
}

// loadWatchState fetches the persisted watch cursor and emits the verbose
// poll-start + watch-state lines so pollOnce stays focused on flow control.
func loadWatchState(ctx context.Context, opts Options, logger *log.Logger) (store.WatchState, error) {
	alias := opts.Account.Alias
	if opts.Verbose {
		logger.Printf("%s  · [%s] poll start", ts(), alias)
	}
	ws, err := opts.Store.GetWatchState(ctx, alias)
	if err != nil {
		return ws, errtrace.WrapCode(err, errtrace.ErrDbWatchState, "watcher.loadWatchState: get watch state")
	}
	if opts.Verbose {
		logger.Printf("%s  · [%s] watch state: lastUid=%d lastSubject=%q", ts(), alias, ws.LastUid, ws.LastSubject)
	}
	return ws, nil
}

// logPollDone emits the verbose end-of-poll summary line.
func logPollDone(opts Options, logger *log.Logger, msgsCount int, lastUid uint32, start time.Time) {
	if !opts.Verbose {
		return
	}
	logger.Printf("%s  · [%s] poll done: processed=%d newLastUid=%d (%s)",
		ts(), opts.Account.Alias, msgsCount, lastUid, time.Since(start).Round(time.Millisecond))
}

func pollOnce(ctx context.Context, opts Options, logger *log.Logger) (*mailclient.MailboxStats, error) {
	start := time.Now()
	ws, err := loadWatchState(ctx, opts, logger)
	if err != nil {
		return nil, err
	}
	mc, stats, err := connectAndSelect(opts, logger, start)
	if err != nil {
		return nil, err
	}
	defer mc.Close()
	if done, berr := handleBaseline(ctx, opts, logger, &ws, &stats); done {
		return &stats, berr
	}
	msgs, empty, err := fetchAndCheckEmpty(opts, logger, mc, ws, stats, start)
	if err != nil || empty {
		return &stats, err
	}
	openedAny, err := processBatch(ctx, opts, logger, &ws, msgs)
	if err != nil {
		return &stats, err
	}
	if err := finalizeBatch(ctx, opts, logger, ws, openedAny); err != nil {
		return &stats, err
	}
	logPollDone(opts, logger, len(msgs), ws.LastUid, start)
	return &stats, nil
}

// processBatch iterates the fetched messages, applying the inter-message
// cooldown when the previous message launched a URL. Returns true when any
// message in the batch launched a URL (caller uses for the cycle cooldown).
func processBatch(ctx context.Context, opts Options, logger *log.Logger, ws *store.WatchState, msgs []*mailclient.Message) (bool, error) {
	openedAny, batchOpened := false, false
	for i, m := range msgs {
		if err := ctx.Err(); err != nil {
			return batchOpened, errtrace.WrapCode(err, errtrace.ErrCoreContextCancelled, "watcher.processBatch: context cancelled mid-batch")
		}
		if openedAny && i > 0 {
			logger.Printf("        ⏳ waiting 10s before next message in batch…")
			if err := sleepCtx(ctx, postOpenCooldown); err != nil {
				return batchOpened, errtrace.WrapCode(err, errtrace.ErrCoreContextCancelled, "watcher.processBatch: cooldown between messages")
			}
			openedAny = false
		}
		opened, err := processMessage(ctx, opts, logger, ws, m)
		if err != nil {
			return batchOpened, err
		}
		if opened {
			openedAny = true
			batchOpened = true
		}
	}
	return batchOpened, nil
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
