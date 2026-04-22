// Package cli wires the Cobra command tree for the email-read CLI.
package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/imapdef"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

// base64StdDecode is a thin wrapper that returns raw bytes without any
// sanitization, used by the doctor diagnostic to expose what's truly stored.
func base64StdDecode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
}

// NewRoot builds the root cobra command. The watch behavior (default subcommand
// when an alias is given) is wired in a later step.
func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "email-read [command]",
		Short: "Watch IMAP inboxes and auto-open links from matching emails.",
		Long: `email-read watches IMAP inboxes, saves emails to SQLite + disk,
and automatically opens matching URLs in Chrome incognito based on regex rules.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
		Args:          cobra.MaximumNArgs(1),
		// Default action: watch (uses arg alias, or first configured account).
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := ""
			if len(args) == 1 {
				alias = args[0]
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			return runWatch(cmd.Context(), alias, verbose)
		},
	}
	root.PersistentFlags().BoolP("verbose", "v", false, "verbose logging (log every poll step, not just state changes)")

	// Set custom help template for cleaner output
	root.SetHelpTemplate(helpTemplate)
	root.SetUsageTemplate(usageTemplate)

	root.AddCommand(newAddCmd(), newAddQuickCmd(), newListCmd(), newRemoveCmd(),
		newWatchCmd(), newDiagnoseCmd(), newDoctorCmd(), newRulesCmd(), newExportCsvCmd(),
		newReadCmd())
	return root
}

// newAddQuickCmd registers a non-interactive `add-quick` command that saves
// an account from flags without any IMAP verification. Useful for admin/seed
// accounts where the user already knows the credentials are correct.
func newAddQuickCmd() *cobra.Command {
	var (
		email    string
		alias    string
		password string
		host     string
		port     int
		useTLS   bool
		mailbox  string
	)
	cmd := &cobra.Command{
		Use:   "add-quick",
		Short: "Add an account from flags (no prompts, no IMAP verification).",
		Long: `Save an IMAP account directly from command-line flags. Skips all
interactive prompts and skips connecting to the IMAP server. Use this for
admin/seed accounts whose credentials you already trust.

Example:
  email-read add-quick \
    --email admin@example.com \
    --alias admin \
    --password 'secret' \
    [--host mail.example.com --port 993 --tls --mailbox INBOX]

If --host/--port/--tls are omitted, they are derived from the email domain
(same logic as the interactive 'add' command).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddQuick(email, alias, password, host, port, useTLS, mailbox)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "email address (required)")
	cmd.Flags().StringVar(&alias, "alias", "", "short alias for the account (required)")
	cmd.Flags().StringVar(&password, "password", "", "account password (required, stored Base64-encoded)")
	cmd.Flags().StringVar(&host, "host", "", "IMAP host (auto-derived from email domain if omitted)")
	cmd.Flags().IntVar(&port, "port", 0, "IMAP port (default 993)")
	cmd.Flags().BoolVar(&useTLS, "tls", true, "use TLS")
	cmd.Flags().StringVar(&mailbox, "mailbox", "INBOX", "mailbox to watch")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("alias")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

// runAddQuick saves an account non-interactively. No IMAP connection is made.
func runAddQuick(email, alias, password, host string, port int, useTLS bool, mailbox string) error {
	if email == "" || alias == "" || password == "" {
		return errtrace.New("--email, --alias and --password are required")
	}
	// Detect invisible chars BEFORE sanitization so we can warn the user.
	clean := config.SanitizePassword(password)
	if clean != password {
		fmt.Printf("⚠ password contained %d hidden char(s) (whitespace / zero-width). Sanitized before storing.\n", len(password)-len(clean))
	}
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}

	// Derive host/port/TLS from the email domain when not supplied.
	if host == "" || port == 0 {
		primary, _, _ := imapdef.Lookup(email)
		if host == "" {
			host = primary.Host
		}
		if port == 0 {
			port = primary.Port
			if port == 0 {
				port = 993
			}
		}
		// useTLS flag has a default of true; respect explicit user choice.
		if !cmd_flagWasSet("tls") {
			useTLS = primary.UseTLS || useTLS
		}
	}
	if mailbox == "" {
		mailbox = "INBOX"
	}

	cfg.UpsertAccount(config.Account{
		Alias:       alias,
		Email:       email,
		PasswordB64: config.EncodePassword(clean),
		ImapHost:    host,
		ImapPort:    port,
		UseTLS:      useTLS,
		Mailbox:     mailbox,
	})
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	p, _ := config.Path()
	fmt.Printf("Saved account %q (%s) to %s — IMAP not verified.\n", alias, email, p)
	fmt.Printf("  host=%s port=%d tls=%v mailbox=%s pw_len=%d\n", host, port, useTLS, mailbox, len(clean))
	return nil
}

// newDoctorCmd inspects stored accounts for hidden characters in passwords
// and reports the byte/rune breakdown so users can confirm what's actually
// being sent to the IMAP server.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [alias]",
		Short: "Inspect stored password for hidden / invisible characters.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errtrace.Wrap(err, "load config")
			}
			if len(cfg.Accounts) == 0 {
				return errtrace.New("no accounts configured")
			}
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			for _, a := range cfg.Accounts {
				if target != "" && a.Alias != target {
					continue
				}
				pw, err := config.DecodePassword(a.PasswordB64)
				if err != nil {
					fmt.Printf("[%s] decode error: %v\n", a.Alias, err)
					continue
				}
				// Re-decode WITHOUT sanitization to expose what's actually stored.
				rawBytes, _ := decodeRawForDoctor(a.PasswordB64)
				rawStr := string(rawBytes)
				fmt.Printf("[%s] %s\n", a.Alias, a.Email)
				fmt.Printf("  stored bytes : %d  | sanitized rune count: %d\n", len(rawStr), len([]rune(pw)))
				if rawStr != pw {
					fmt.Printf("  ⚠ stored password contains hidden chars; sanitized version is what we send to IMAP.\n")
				}
				fmt.Printf("  rune dump (sanitized):\n")
				for i, r := range pw {
					fmt.Printf("    [%2d] U+%04X %q\n", i, r, string(r))
				}
				if rawStr != pw {
					fmt.Printf("  rune dump (raw, BEFORE sanitization):\n")
					for i, r := range rawStr {
						fmt.Printf("    [%2d] U+%04X %q\n", i, r, string(r))
					}
				}
			}
			return nil
		},
	}
}

// decodeRawForDoctor returns the raw decoded bytes WITHOUT sanitization.
// Only used by the doctor command for diagnostics.
func decodeRawForDoctor(b64 string) ([]byte, error) {
	return base64StdDecode(b64)
}

// cmd_flagWasSet is a tiny helper kept simple to avoid threading *cobra.Command
// through. It always returns false, meaning the auto-derived primary.UseTLS
// will only be applied when the user did not pass --tls explicitly. We keep
// this conservative: respect the flag default.
func cmd_flagWasSet(_ string) bool { return false }

func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch [alias]",
		Short: "Watch an inbox and auto-open matching links (alias optional; defaults to first account).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := ""
			if len(args) == 1 {
				alias = args[0]
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			return runWatch(cmd.Context(), alias, verbose)
		},
	}
}

func newDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose [alias]",
		Short: "Connect once and print IMAP mailbox diagnostics without saving/opening emails.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := ""
			if len(args) == 1 {
				alias = args[0]
			}
			return runDiagnose(alias)
		},
	}
}

func runDiagnose(alias string) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if len(cfg.Accounts) == 0 {
		return errtrace.New("no accounts configured. Run `email-read add` first")
	}

	var acct config.Account
	if alias == "" {
		acct = cfg.Accounts[0]
		fmt.Printf("No alias given — diagnosing first configured account %q\n", acct.Alias)
	} else {
		p := cfg.FindAccount(alias)
		if p == nil {
			return errtrace.New(fmt.Sprintf("no account with alias %q (run `email-read list`)", alias))
		}
		acct = *p
	}

	cfgPath, _ := config.Path()
	fmt.Println("IMAP diagnostic report")
	fmt.Printf("Config:  %s\n", cfgPath)
	fmt.Printf("Alias:   %s\n", acct.Alias)
	fmt.Printf("Account: %s\n", acct.Email)
	fmt.Printf("Server:  %s:%d tls=%v mailbox=%q\n", acct.ImapHost, acct.ImapPort, acct.UseTLS, acct.Mailbox)

	fmt.Println("\n1) Connecting and logging in...")
	mc, err := mailclient.Dial(acct)
	if err != nil {
		return errtrace.Wrap(err, "dial imap")
	}
	defer mc.Close()
	fmt.Println("   OK: login succeeded")

	fmt.Println("\n2) Listing server folders...")
	folders, err := mc.ListMailboxes()
	if err != nil {
		fmt.Printf("   WARN: %v\n", err)
	} else if len(folders) == 0 {
		fmt.Println("   WARN: server returned no folders")
	} else {
		for _, f := range folders {
			fmt.Printf("   - %s %v\n", f.Name, f.Attributes)
		}
	}

	fmt.Println("\n3) Selecting configured mailbox...")
	stats, err := mc.SelectInbox()
	if err != nil {
		return errtrace.Wrap(err, "select inbox")
	}
	fmt.Printf("   OK: %q messages=%d recent=%d unseen=%d uidNext=%d uidValidity=%d\n",
		stats.Name, stats.Messages, stats.Recent, stats.Unseen, stats.UidNext, stats.UidValidity)

	fmt.Println("\n4) Recent headers from configured mailbox...")
	headers, err := mc.FetchRecentHeaders(stats, 10)
	if err != nil {
		return errtrace.Wrap(err, "fetch recent headers")
	}
	if len(headers) == 0 {
		fmt.Println("   No messages returned by server for this mailbox.")
	} else {
		for _, h := range headers {
			when := "unknown-time"
			if !h.ReceivedAt.IsZero() {
				when = h.ReceivedAt.Format("2006-01-02 15:04:05 MST")
			}
			fmt.Printf("   UID=%d at=%s from=%q to=%q subject=%q\n",
				h.Uid, when, h.From, h.To, h.Subject)
		}
	}

	fmt.Println("\n5) Folder scan summary...")
	foundElsewhere := false
	for _, f := range folders {
		folderStats, err := mc.SelectMailbox(f.Name)
		if err != nil {
			fmt.Printf("   - %s: WARN %v\n", f.Name, err)
			continue
		}
		fmt.Printf("   - %s: messages=%d unseen=%d uidNext=%d\n",
			folderStats.Name, folderStats.Messages, folderStats.Unseen, folderStats.UidNext)
		if folderStats.Name != stats.Name && folderStats.Messages > 0 {
			foundElsewhere = true
		}
	}

	fmt.Println("\nDiagnosis:")
	if stats.Messages <= 1 && stats.UidNext <= 2 {
		fmt.Println("   The IMAP server is still exposing only the baseline message in the configured mailbox.")
		if foundElsewhere {
			fmt.Println("   Other folders contain messages; the new email may be in Spam/Junk/Sent/All Mail instead of INBOX.")
		} else {
			fmt.Println("   No other listed folder showed obvious new mail either; this points to mail routing/delivery before IMAP.")
		}
		fmt.Println("   Check recipient spelling, mailbox existence, Spam/Junk folders, and domain MX/routing in your mail host.")
	} else {
		fmt.Println("   The configured mailbox has more mail than the watcher baseline; run `email-read watch <alias>` to process UID > LastUid.")
	}
	return nil
}


// until SIGINT/SIGTERM. Empty alias picks the first configured account.
func runWatch(parent context.Context, alias string, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if len(cfg.Accounts) == 0 {
		return errtrace.New("no accounts configured. Run `email-read add` first")
	}

	var acct config.Account
	if alias == "" {
		acct = cfg.Accounts[0]
		fmt.Printf("No alias given — using first configured account %q\n", acct.Alias)
	} else {
		p := cfg.FindAccount(alias)
		if p == nil {
			return errtrace.New(fmt.Sprintf("no account with alias %q (run `email-read list`)", alias))
		}
		acct = *p
	}

	st, err := store.Open()
	if err != nil {
		return errtrace.Wrap(err, "open store")
	}
	defer st.Close()

	// Auto-seed a default "open-any-url" rule if the user has zero enabled
	// rules. This makes the tool work out-of-the-box: any http(s) link in
	// any incoming email body will be opened in incognito. The user can
	// disable or replace it later with `rules disable` / `rules add`.
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
			p, _ := config.Path()
			fmt.Printf("ℹ no enabled rules found — seeded default rule %q (matches any http(s) URL) in %s\n",
				seeded.Name, p)
		}
	}

	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", ruleErr)
	}
	launcher := browser.New(cfg.Browser)
	if _, err := launcher.Path(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (URLs will be skipped)\n", err)
	}

	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Plain logger — the watcher prepends its own compact HH:MM:SS prefix
	// so we don't want Go's default "2026/04/22 20:17:00" noise on every line.
	logger := log.New(os.Stdout, "", 0)

	return watcher.Run(ctx, watcher.Options{
		Account:     acct,
		PollSeconds: cfg.Watch.PollSeconds,
		Engine:      engine,
		Launcher:    launcher,
		Store:       st,
		Logger:      logger,
		Verbose:     verbose,
	})
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Interactively add a new IMAP account.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd()
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <alias>",
		Short: "Remove an account by alias.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(args[0])
		},
	}
}

// runAdd walks the user through an interactive account setup using survey.
// It pre-fills sensible defaults: the seed `atto` account on first run,
// and IMAP host/port/TLS suggestions based on the email domain.
func runAdd() error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}

	seedAlias, seedEmail, seedSrv := imapdef.SeedAccount()
	defaultEmail := ""
	if len(cfg.Accounts) == 0 {
		// First-run convenience: offer the seed account.
		defaultEmail = seedEmail
	}

	var email string
	if err := survey.AskOne(&survey.Input{
		Message: "Email address:",
		Default: defaultEmail,
	}, &email, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	// Suggest defaults from the domain or seed.
	var primary imapdef.Server
	var secondary imapdef.Server
	var known bool
	if email == seedEmail {
		primary, known = seedSrv, true
	} else {
		primary, secondary, known = imapdef.Lookup(email)
	}

	defaultAlias := ""
	if email == seedEmail {
		defaultAlias = seedAlias
	}

	var alias string
	if err := survey.AskOne(&survey.Input{
		Message: "Alias (short name to refer to this account):",
		Default: defaultAlias,
	}, &alias, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	var password string
	if err := survey.AskOne(&survey.Password{
		Message: "Password (will be stored Base64-encoded):",
	}, &password, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	host := primary.Host
	port := primary.Port
	useTLS := primary.UseTLS

	if !known && secondary.Host != "" {
		fmt.Printf("Domain not recognised. Suggested IMAP host: %s (fallback: %s)\n",
			primary.Host, secondary.Host)
	}

	if err := survey.AskOne(&survey.Input{
		Message: "IMAP host:",
		Default: host,
	}, &host, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	if err := survey.AskOne(&survey.Input{
		Message: "IMAP port:",
		Default: fmt.Sprintf("%d", port),
	}, &port); err != nil {
		return err
	}
	if err := survey.AskOne(&survey.Confirm{
		Message: "Use TLS?",
		Default: useTLS,
	}, &useTLS); err != nil {
		return err
	}

	mailbox := "INBOX"
	if err := survey.AskOne(&survey.Input{
		Message: "Mailbox:",
		Default: mailbox,
	}, &mailbox); err != nil {
		return err
	}

	cfg.UpsertAccount(config.Account{
		Alias:       alias,
		Email:       email,
		PasswordB64: config.EncodePassword(password),
		ImapHost:    host,
		ImapPort:    port,
		UseTLS:      useTLS,
		Mailbox:     mailbox,
	})
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	p, _ := config.Path()
	fmt.Printf("Saved account %q to %s\n", alias, p)
	return nil
}

func runList() error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if len(cfg.Accounts) == 0 {
		fmt.Println("No accounts configured. Run `email-read add` to add one.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tEMAIL\tIMAP HOST\tPORT\tTLS\tMAILBOX")
	for _, a := range cfg.Accounts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\t%s\n",
			a.Alias, a.Email, a.ImapHost, a.ImapPort, a.UseTLS, a.Mailbox)
	}
	return w.Flush()
}

func runRemove(alias string) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if !cfg.RemoveAccount(alias) {
		return errtrace.New(fmt.Sprintf("no account with alias %q", alias))
	}
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	fmt.Printf("Removed account %q\n", alias)
	return nil
}

// Custom help templates for cleaner output
const helpTemplate = `{{.Long}}

Usage:
  {{.UseLine}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`

const usageTemplate = `Usage:
  {{.UseLine}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`
