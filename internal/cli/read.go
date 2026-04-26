// Re-process a previously-saved email: run the rules engine against its
// stored body and open any matched URLs in the configured incognito browser.
//
// This is the manual counterpart to `watch`: useful when the user wants to
// re-trigger the verification flow for an email that was already received,
// or when they're debugging a rule and want to retry without waiting for
// new mail. Same per-rule trace + 10s post-open cooldown as the watcher.
package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// newReadCmd registers `email-read read <alias> <uid>` — re-runs the rule
// engine on a previously-saved email and opens matched URLs in incognito.
func newReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read <alias> <uid>",
		Short: "Re-open URLs from a saved email (alias + IMAP UID) in incognito.",
		Long: `Load a previously-saved email by (alias, uid), evaluate the configured
rules against its stored body, and open any matched URLs in the same
incognito browser the watcher uses.

This bypasses dedup: even if the watcher already opened these URLs once,
'read' will re-launch them. Useful when the verification page expired or
the browser was closed before you clicked.

Example:
  email-read read admin 12`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			uid64, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return errtrace.Wrapf(err, "parse uid %q", args[1])
			}
			return runRead(cmd.Context(), alias, uint32(uid64))
		},
	}
}

func runRead(parent context.Context, alias string, uid uint32) error {
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stdout, "", 0)
	emit := func(ev core.ReadEvent) { renderReadEvent(logger, alias, ev) }
	r := core.ReadEmail(ctx, alias, uid, emit)
	if r.HasError() {
		return r.PropagateError()
	}
	return nil
}

// renderReadEvent prints one event in the existing CLI log style. Kept here
// (not in core) so the UI can render the same events differently.
func renderReadEvent(logger *log.Logger, alias string, ev core.ReadEvent) {
	switch ev.Kind {
	case core.ReadEventStart:
		e := ev.Email
		logger.Printf("[%s] read uid=%d  from=%s  subj=%q",
			alias, e.Uid, shortAddrCli(e.From), e.Subject)
		if e.FilePath != "" {
			logger.Printf("    file: %s", e.FilePath)
		}
	case core.ReadEventSeededRule:
		if ev.Note != "" {
			fmt.Fprintln(os.Stderr, ev.Note)
		} else {
			logger.Printf("    ℹ no enabled rules found — seeded default rule %q (matches any http(s) URL)", ev.RuleName)
		}
	case core.ReadEventRulesLoaded:
		if ev.Note != "" {
			fmt.Fprintln(os.Stderr, ev.Note)
		}
		if ev.RuleCount > 0 {
			logger.Printf("    %d enabled rule(s) loaded", ev.RuleCount)
		}
	case core.ReadEventBrowserResolved:
		logger.Printf("    browser ready: %s (incognito flag=%q)", ev.BrowserPath, ev.IncognitoArg)
	case core.ReadEventNoRules:
		logger.Printf("    ⚠ 0 enabled rules — nothing to evaluate.")
	case core.ReadEventRuleTrace:
		t := ev.Trace
		if len(t.UrlsFound) > 0 {
			logger.Printf("    rules: ✓ %q → %d url(s)", t.RuleName, len(t.UrlsFound))
		} else {
			logger.Printf("    rules: ✗ %q → %s", t.RuleName, t.Reason)
		}
	case core.ReadEventNoMatches:
		logger.Printf("    no URLs matched — nothing to open.")
	case core.ReadEventCooldown:
		logger.Printf("    ⏳ waiting %s before opening next URL…", ev.CooldownFor)
	case core.ReadEventOpening:
		if ev.Note != "" {
			logger.Printf("    (note: %s already in OpenedUrls — re-launching anyway)", ev.Url)
		}
		logger.Printf("    → opening in incognito: %s (rule=%s)", ev.Url, ev.RuleName)
	case core.ReadEventOpened:
		logger.Printf("    ✓ launched")
	case core.ReadEventOpenFailed:
		logger.Printf("    ✗ browser launch failed for %s:\n%s", ev.Url, errtrace.Format(ev.Err))
	case core.ReadEventDone:
		// no-op — the trailing log lines already convey completion.
	}
}

// shortAddrCli mirrors the watcher's helper. Duplicated here to avoid a
// circular import (cli already depends on watcher's siblings, not watcher).
func shortAddrCli(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			for j := i + 1; j < len(s); j++ {
				if s[j] == '>' {
					return s[i+1 : j]
				}
			}
		}
	}
	return s
}
