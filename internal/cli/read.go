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
	"time"

	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
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
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if cfg.FindAccount(alias) == nil {
		return errtrace.New(fmt.Sprintf("no account with alias %q (run `email-read list`)", alias))
	}

	st, err := store.Open()
	if err != nil {
		return errtrace.Wrap(err, "open store")
	}
	defer st.Close()

	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	row, err := st.GetEmailByUid(ctx, alias, uid)
	if err != nil {
		return errtrace.Wrapf(err, "load email alias=%s uid=%d", alias, uid)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("[%s] read uid=%d  from=%s  subj=%q", alias, row.Uid, shortAddrCli(row.FromAddr), row.Subject)
	if row.FilePath != "" {
		logger.Printf("    file: %s", row.FilePath)
	}

	// Auto-seed default rule (same behavior as `watch`) so `read` works
	// out-of-the-box for users who have not hand-edited config.json.
	if countEnabledRules(cfg.Rules) == 0 {
		seeded := config.Rule{
			Name:     "default-open-any-url",
			Enabled:  true,
			UrlRegex: `https?://[^\s<>"'\)\]]+`,
		}
		cfg.Rules = append(cfg.Rules, seeded)
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save seeded rule: %v\n", err)
		} else {
			logger.Printf("    ℹ no enabled rules found — seeded default rule %q (matches any http(s) URL)", seeded.Name)
		}
	}

	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", ruleErr)
	}
	if engine == nil || engine.RuleCount() == 0 {
		logger.Printf("    ⚠ 0 enabled rules — nothing to evaluate.")
		return nil
	}
	logger.Printf("    %d enabled rule(s) loaded", engine.RuleCount())

	launcher := browser.New(cfg.Browser)
	if path, err := launcher.Path(); err != nil {
		logger.Printf("    ⚠ browser not resolved:\n%s", errtrace.Format(err))
		return errtrace.Wrap(err, "resolve browser")
	} else {
		logger.Printf("    browser ready: %s (incognito flag=%q)", path, launcher.IncognitoArg())
	}

	// Build a Message struct from the stored row so the rules engine can
	// evaluate it exactly the way the watcher would.
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
		if len(t.UrlsFound) > 0 {
			logger.Printf("    rules: ✓ %q → %d url(s)", t.RuleName, len(t.UrlsFound))
		} else {
			logger.Printf("    rules: ✗ %q → %s", t.RuleName, t.Reason)
		}
	}

	if len(matches) == 0 {
		logger.Printf("    no URLs matched — nothing to open.")
		return nil
	}

	for i, match := range matches {
		if err := ctx.Err(); err != nil {
			return errtrace.Wrap(err, "cancelled mid-batch")
		}
		// 10s gap between URLs in the SAME email so the verification page
		// from the previous URL has time to load before the next one opens.
		if i > 0 {
			logger.Printf("    ⏳ waiting 10s before opening next URL…")
			if err := sleepCtxCli(ctx, 10*time.Second); err != nil {
				return errtrace.Wrap(err, "cooldown between URLs")
			}
		}
		// Always re-record the open (helps audit), but don't dedup-skip:
		// the user explicitly asked to re-open.
		if _, err := st.RecordOpenedUrl(ctx, row.Id, match.RuleName, match.Url); err != nil {
			// Non-fatal — most likely a UNIQUE conflict from a previous open.
			logger.Printf("    (note: %s already in OpenedUrls — re-launching anyway)", match.Url)
		}
		logger.Printf("    → opening in incognito: %s (rule=%s)", match.Url, match.RuleName)
		if err := launcher.Open(match.Url); err != nil {
			logger.Printf("    ✗ browser launch failed for %s:\n%s", match.Url, errtrace.Format(err))
			continue
		}
		logger.Printf("    ✓ launched")
	}
	return nil
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

// sleepCtxCli pauses for d but returns early on Ctrl+C.
func sleepCtxCli(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
