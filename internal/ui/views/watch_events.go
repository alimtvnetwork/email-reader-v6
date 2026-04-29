// watch_events.go — framework-agnostic helpers that translate
// `watcher.Event` values into the strings + counters the Watch view
// renders. Kept out of watch.go so we can unit-test the formatting
// logic without dragging in the Fyne build tag.
//
// Why a separate file: the Cards tab, Raw log tab, and footer
// counters all consume the same event stream but render different
// projections. Centralising the projections here keeps the Fyne file
// focused on widget plumbing and gives us a single place to evolve
// the on-screen wording.
package views

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/ui/errlog"
	"github.com/lovable/email-read/internal/watcher"
)

// ReportWatchEventError forwards error-bearing watcher events into the
// process-wide error log (Diagnostics → Error Log + persisted
// data/error-log.jsonl + `email-read errors tail`). Without this hook
// the Watch view's Raw log tab is the *only* place poll errors appear,
// and the user (rightly) expected the Error Log to mirror them.
//
// Currently forwards EventPollError and the failure branch of
// EventUrlOpened. Other event kinds carry no error and are skipped.
// Component tag is namespaced as "watcher.<alias>" so multi-account
// installs can distinguish sources in the error log without losing the
// `watcher` prefix the rest of the codebase uses.
//
// Safe to call from any goroutine; nil-safe (errlog.ReportError is a
// no-op when ev.Err is nil).
func ReportWatchEventError(ev watcher.Event) {
	switch ev.Kind {
	case watcher.EventPollError:
		errlog.ReportError("watcher."+ev.Alias, ev.Err)
	case watcher.EventUrlOpened:
		if !ev.OpenOK {
			errlog.ReportError("watcher.openurl."+ev.Alias, ev.Err)
		}
	}
}

// WatchCounters is the rolling tally shown in the footer. Incremented
// by accumulateCounters; reset by ResetCounters when the user
// restarts a runner. Plain ints — atomic safety is provided by the
// single-goroutine consumer that owns the struct.
type WatchCounters struct {
	Polls    int
	NewMail  int
	Matches  int
	Errors   int
	Opens    int
	OpenFail int
}

// FormatCounters renders the footer string. Stable order so the UI
// width does not jiggle as numbers grow.
func (c WatchCounters) FormatCounters() string {
	return fmt.Sprintf("polls=%d · newMail=%d · matches=%d · opens=%d · errors=%d",
		c.Polls, c.NewMail, c.Matches, c.Opens, c.Errors)
}

// AccumulateCounters folds one event into the counters. Centralises
// the kind→counter mapping so the UI consumer stays a one-liner.
// Returns the updated counters by value to keep the call site
// goroutine-safe (the consumer owns the canonical copy).
func AccumulateCounters(c WatchCounters, ev watcher.Event) WatchCounters {
	switch ev.Kind {
	case watcher.EventPollOK, watcher.EventBaseline, watcher.EventHeartbeat:
		c.Polls++
	case watcher.EventNewMail:
		c.NewMail++
	case watcher.EventRuleMatch:
		c.Matches++
	case watcher.EventPollError:
		c.Errors++
	case watcher.EventUrlOpened:
		if ev.OpenOK {
			c.Opens++
		} else {
			c.OpenFail++
		}
	}
	return c
}

// FormatRawLogLine renders one watcher.Event as a single CLI-style
// log line — the format the Raw log tab prepends to its scroll
// buffer. Mirrors the watcher's own logger lines closely so users who
// know the CLI feel at home.
func FormatRawLogLine(ev watcher.Event) string {
	ts := ev.At.Format("15:04:05")
	switch ev.Kind {
	case watcher.EventStarted:
		return fmt.Sprintf("%s  ▶ [%s] watch started", ts, ev.Alias)
	case watcher.EventStopped:
		return fmt.Sprintf("%s  ■ [%s] watch stopped", ts, ev.Alias)
	case watcher.EventBaseline:
		return fmt.Sprintf("%s  · [%s] baseline %s", ts, ev.Alias, formatStats(ev))
	case watcher.EventPollOK:
		return fmt.Sprintf("%s  · [%s] poll ok %s", ts, ev.Alias, formatStats(ev))
	case watcher.EventPollError:
		return fmt.Sprintf("%s  ✗ [%s] poll error: %s", ts, ev.Alias, errMsg(ev.Err))
	case watcher.EventNewMail:
		return fmt.Sprintf("%s  ✉ [%s] new mail: %s", ts, ev.Alias, formatMessageHeader(ev))
	case watcher.EventRuleMatch:
		return fmt.Sprintf("%s  ◆ [%s] rule %q matched %s", ts, ev.Alias, ev.RuleName, truncURL(ev.Url))
	case watcher.EventUrlOpened:
		return formatUrlOpenedLine(ts, ev)
	case watcher.EventHeartbeat:
		return fmt.Sprintf("%s  ♥ [%s] heartbeat %s", ts, ev.Alias, formatStats(ev))
	case watcher.EventUidValReset:
		return fmt.Sprintf("%s  ⚠ [%s] UIDVALIDITY reset %s", ts, ev.Alias, formatStats(ev))
	}
	return fmt.Sprintf("%s  ? [%s] unknown event %q", ts, ev.Alias, ev.Kind)
}

// formatUrlOpenedLine handles the open success / failure split so
// FormatRawLogLine stays under the 15-statement cap.
func formatUrlOpenedLine(ts string, ev watcher.Event) string {
	if ev.OpenOK {
		return fmt.Sprintf("%s  → [%s] opened %s", ts, ev.Alias, truncURL(ev.Url))
	}
	return fmt.Sprintf("%s  ✗ [%s] open failed %s: %s", ts, ev.Alias, truncURL(ev.Url), errMsg(ev.Err))
}

// WatchCard is the projection that powers the Cards tab. Only events
// the user actually cares about become cards (new mail, rule match,
// open success/failure, poll errors). Heartbeats and ok-polls are
// noise — they live in the Raw log tab instead.
type WatchCard struct {
	At    time.Time
	Title string
	Body  string
	Tone  CardTone
}

// CardTone classifies the card visually. The Fyne layer maps each
// tone to an icon + colour; here we just enumerate.
type CardTone int

const (
	CardToneInfo CardTone = iota
	CardToneSuccess
	CardToneWarn
	CardToneError
)

// EventToCard converts an event into a card, or returns ok=false for
// events that do not deserve a dedicated card (heartbeat, baseline,
// ok-poll, started/stopped — the header already tracks lifecycle).
func EventToCard(ev watcher.Event) (WatchCard, bool) {
	switch ev.Kind {
	case watcher.EventNewMail:
		return cardFromNewMail(ev), true
	case watcher.EventRuleMatch:
		return WatchCard{At: ev.At, Tone: CardToneInfo,
			Title: "Rule matched: " + ev.RuleName,
			Body:  truncURL(ev.Url)}, true
	case watcher.EventUrlOpened:
		return cardFromUrlOpened(ev), true
	case watcher.EventPollError:
		return WatchCard{At: ev.At, Tone: CardToneError,
			Title: "Poll error · " + ev.Alias,
			Body:  errMsg(ev.Err)}, true
	case watcher.EventUidValReset:
		return WatchCard{At: ev.At, Tone: CardToneWarn,
			Title: "UIDVALIDITY reset · " + ev.Alias,
			Body:  "Server reset the UID space; baseline re-established."}, true
	}
	return WatchCard{}, false
}

// cardFromNewMail builds the new-mail card. Split out so EventToCard
// stays compact and so we can unit-test the header projection
// independently.
func cardFromNewMail(ev watcher.Event) WatchCard {
	return WatchCard{
		At:    ev.At,
		Tone:  CardToneSuccess,
		Title: "New mail · " + ev.Alias,
		Body:  formatMessageHeader(ev),
	}
}

// cardFromUrlOpened builds the URL-open card; tone splits on OpenOK.
func cardFromUrlOpened(ev watcher.Event) WatchCard {
	if ev.OpenOK {
		return WatchCard{At: ev.At, Tone: CardToneSuccess,
			Title: "Opened URL", Body: truncURL(ev.Url)}
	}
	return WatchCard{At: ev.At, Tone: CardToneError,
		Title: "Open failed", Body: truncURL(ev.Url) + " — " + errMsg(ev.Err)}
}

// formatMessageHeader renders the From + Subject line for a new-mail
// event. Defensive against nil Message (the publisher contract says
// it is set, but a future bug here would otherwise nil-panic the UI).
func formatMessageHeader(ev watcher.Event) string {
	if ev.Message == nil {
		return "(message details unavailable)"
	}
	from := strings.TrimSpace(ev.Message.From)
	if from == "" {
		from = "(unknown sender)"
	}
	subj := strings.TrimSpace(ev.Message.Subject)
	if subj == "" {
		subj = "(no subject)"
	}
	return from + " — " + subj
}

// formatStats renders the MailboxStats triple shown next to ok-polls
// / heartbeats / baseline lines. Empty string when stats absent so
// the parent line stays clean.
func formatStats(ev watcher.Event) string {
	if ev.Stats == nil {
		return ""
	}
	return fmt.Sprintf("(messages=%d uidnext=%d unseen=%d)",
		ev.Stats.Messages, ev.Stats.UidNext, ev.Stats.Unseen)
}

// errMsg defensively renders an error, returning a placeholder when
// the publisher passed nil despite the kind contract.
func errMsg(err error) string {
	if err == nil {
		return "(no error message)"
	}
	if isMailReachabilityError(err) {
		return err.Error() + " — TCP never reached IMAP login; test 993/143 with nc and ask hosting to open IMAP/Dovecot/firewall if they time out"
	}
	return err.Error()
}

func isMailReachabilityError(err error) bool {
	var coded *errtrace.Coded
	return errors.As(err, &coded) && (coded.Code == errtrace.ErrMailTimeout || coded.Code == errtrace.ErrMailDial)
}

// truncURL keeps URL renderings readable. Mirrors watcher.truncURL —
// duplicated rather than exported to avoid widening the watcher
// package's surface for a UI-only concern.
func truncURL(u string) string {
	const max = 90
	if len(u) <= max {
		return u
	}
	return u[:max-1] + "…"
}

// AppendBounded prepends `s` to `buf` and trims to `cap` entries. The
// Raw log tab and Cards tab both use this so the slice never grows
// without bound (a long-running watcher could otherwise OOM the UI).
// Newest-first ordering matches user expectation: latest event at the
// top, scroll down for history.
func AppendBounded[T any](buf []T, item T, cap int) []T {
	if cap <= 0 {
		return buf
	}
	out := append([]T{item}, buf...)
	if len(out) > cap {
		out = out[:cap]
	}
	return out
}
