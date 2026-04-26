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
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/imapdef"
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
// Delegates the heavy lifting to internal/core so the same code powers the UI.
func runAddQuick(email, alias, password, host string, port int, useTLS bool, mailbox string) error {
	r := core.AddAccount(core.AccountInput{
		Alias:          alias,
		Email:          email,
		PlainPassword:  password,
		ImapHost:       host,
		ImapPort:       port,
		UseTLS:         useTLS,
		UseTLSExplicit: false, // CLI flag default is true; keep legacy behavior
		Mailbox:        mailbox,
	})
	if r.HasError() {
		return r.PropagateError()
	}
	res := r.Value()
	if res.HiddenCharsRem > 0 {
		fmt.Printf("⚠ password contained %d hidden char(s) (whitespace / zero-width). Sanitized before storing.\n", res.HiddenCharsRem)
	}
	a := res.Account
	pw, _ := config.DecodePassword(a.PasswordB64)
	fmt.Printf("Saved account %q (%s) to %s — IMAP not verified.\n", a.Alias, a.Email, res.ConfigPath)
	fmt.Printf("  host=%s port=%d tls=%v mailbox=%s pw_len=%d\n", a.ImapHost, a.ImapPort, a.UseTLS, a.Mailbox, len(pw))
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

// (cmd_flagWasSet was removed when runAddQuick moved to core.AddAccount.)

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

// runDiagnose is now a thin wrapper around core.Diagnose. It renders each
// structured event in the same step-numbered style the CLI used previously.
func runDiagnose(alias string) error {
	cfgPath, _ := config.Path()
	fmt.Println("IMAP diagnostic report")
	fmt.Printf("Config:  %s\n", cfgPath)

	step := 0
	res := core.Diagnose(alias, func(ev core.DiagnoseEvent) {
		switch ev.Kind {
		case core.DiagnoseEventStart:
			a := ev.Account
			if alias == "" {
				fmt.Printf("No alias given — diagnosing first configured account %q\n", a.Alias)
			}
			fmt.Printf("Alias:   %s\n", a.Alias)
			fmt.Printf("Account: %s\n", a.Email)
			fmt.Printf("Server:  %s:%d tls=%v mailbox=%q\n", a.ImapHost, a.ImapPort, a.UseTLS, a.Mailbox)
			step = 1
			fmt.Printf("\n%d) Connecting and logging in...\n", step)
		case core.DiagnoseEventLoginOK:
			fmt.Println("   OK: login succeeded")
			step++
			fmt.Printf("\n%d) Listing server folders...\n", step)
		case core.DiagnoseEventFolders:
			for _, f := range ev.Folders {
				fmt.Printf("   - %s %v\n", f.Name, f.Attributes)
			}
			step++
			fmt.Printf("\n%d) Selecting configured mailbox...\n", step)
		case core.DiagnoseEventFoldersWarn:
			fmt.Printf("   WARN: %s\n", ev.Message)
			step++
			fmt.Printf("\n%d) Selecting configured mailbox...\n", step)
		case core.DiagnoseEventInbox:
			s := ev.Stats
			fmt.Printf("   OK: %q messages=%d recent=%d unseen=%d uidNext=%d uidValidity=%d\n",
				s.Name, s.Messages, s.Recent, s.Unseen, s.UidNext, s.UidValidity)
			step++
			fmt.Printf("\n%d) Recent headers from configured mailbox...\n", step)
		case core.DiagnoseEventHeaders:
			if len(ev.Headers) == 0 {
				fmt.Println("   No messages returned by server for this mailbox.")
			} else {
				for _, h := range ev.Headers {
					when := "unknown-time"
					if !h.ReceivedAt.IsZero() {
						when = h.ReceivedAt.Format("2006-01-02 15:04:05 MST")
					}
					fmt.Printf("   UID=%d at=%s from=%q to=%q subject=%q\n",
						h.Uid, when, h.From, h.To, h.Subject)
				}
			}
			step++
			fmt.Printf("\n%d) Folder scan summary...\n", step)
		case core.DiagnoseEventFolderStat:
			s := ev.Stats
			fmt.Printf("   - %s: messages=%d unseen=%d uidNext=%d\n",
				s.Name, s.Messages, s.Unseen, s.UidNext)
		case core.DiagnoseEventFolderWarn:
			fmt.Printf("   - %s\n", ev.Message)
		case core.DiagnoseEventSummary:
			fmt.Println("\nDiagnosis:")
			fmt.Printf("   %s\n", ev.Message)
			fmt.Println("   Check recipient spelling, mailbox existence, Spam/Junk folders, and domain MX/routing in your mail host.")
		}
	})
	return res.PropagateError()
}


// until SIGINT/SIGTERM. Empty alias picks the first configured account.
func runWatch(parent context.Context, alias string, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	acct, err := resolveWatchAccount(cfg, alias)
	if err != nil {
		return err
	}

	st, err := store.Open()
	if err != nil {
		return errtrace.Wrap(err, "open store")
	}
	defer st.Close()

	seedDefaultRuleIfMissing(cfg)
	engine, launcher := buildWatchEngineAndLauncher(cfg)

	ctx, stop := newWatchContext(parent)
	defer stop()

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

// resolveWatchAccount picks the watch target by alias (or first when empty).
func resolveWatchAccount(cfg *config.Config, alias string) (config.Account, error) {
	if len(cfg.Accounts) == 0 {
		return config.Account{}, errtrace.New("no accounts configured. Run `email-read add` first")
	}
	if alias == "" {
		acct := cfg.Accounts[0]
		fmt.Printf("No alias given — using first configured account %q\n", acct.Alias)
		return acct, nil
	}
	p := cfg.FindAccount(alias)
	if p == nil {
		return config.Account{}, errtrace.New(fmt.Sprintf("no account with alias %q (run `email-read list`)", alias))
	}
	return *p, nil
}

// seedDefaultRuleIfMissing inserts a permissive default rule when the user
// has zero enabled rules, so the tool works out-of-the-box.
func seedDefaultRuleIfMissing(cfg *config.Config) {
	if countEnabledRules(cfg.Rules) != 0 {
		return
	}
	seeded := config.Rule{
		Name:     "default-open-any-url",
		Enabled:  true,
		UrlRegex: `https?://[^\s<>"'\)\]]+`,
	}
	cfg.Rules = append(cfg.Rules, seeded)
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save seeded rule: %v\n", err)
		return
	}
	p, _ := config.Path()
	fmt.Printf("ℹ no enabled rules found — seeded default rule %q (matches any http(s) URL) in %s\n",
		seeded.Name, p)
}

// buildWatchEngineAndLauncher initializes the rules engine and browser launcher,
// printing warnings (but not failing) if either has setup issues.
func buildWatchEngineAndLauncher(cfg *config.Config) (*rules.Engine, *browser.Launcher) {
	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", ruleErr)
	}
	launcher := browser.New(cfg.Browser)
	if _, err := launcher.Path(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (URLs will be skipped)\n", err)
	}
	return engine, launcher
}

// newWatchContext wraps the parent context with SIGINT/SIGTERM cancellation.
func newWatchContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
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
// addAccountDefaults bundles per-prompt defaults derived from config + IMAP catalog.
type addAccountDefaults struct {
	email, alias       string
	primary, secondary imapdef.Server
	known              bool
}

// resolveAddDefaults computes the seed/lookup-derived defaults for the `add` flow.
func resolveAddDefaults(cfg config.Config, email string) addAccountDefaults {
	seedAlias, seedEmail, seedSrv := imapdef.SeedAccount()
	d := addAccountDefaults{}
	if email == "" && len(cfg.Accounts) == 0 {
		d.email = seedEmail
	}
	if email == seedEmail {
		d.primary, d.known, d.alias = seedSrv, true, seedAlias
		return d
	}
	d.primary, d.secondary, d.known = imapdef.Lookup(email)
	return d
}

// promptAddIdentity asks for email + alias + password.
func promptAddIdentity(cfg config.Config) (email, alias, password string, d addAccountDefaults, err error) {
	d = resolveAddDefaults(cfg, "")
	if err = survey.AskOne(&survey.Input{Message: "Email address:", Default: d.email},
		&email, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	d = resolveAddDefaults(cfg, email)
	if err = survey.AskOne(&survey.Input{Message: "Alias (short name to refer to this account):", Default: d.alias},
		&alias, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	err = survey.AskOne(&survey.Password{Message: "Password (will be stored Base64-encoded):"},
		&password, survey.WithValidator(survey.Required))
	return
}

// promptAddServer asks for host/port/tls/mailbox using the resolved defaults.
func promptAddServer(d addAccountDefaults) (host string, port int, useTLS bool, mailbox string, err error) {
	host, port, useTLS, mailbox = d.primary.Host, d.primary.Port, d.primary.UseTLS, "INBOX"
	if !d.known && d.secondary.Host != "" {
		fmt.Printf("Domain not recognised. Suggested IMAP host: %s (fallback: %s)\n",
			d.primary.Host, d.secondary.Host)
	}
	if err = survey.AskOne(&survey.Input{Message: "IMAP host:", Default: host},
		&host, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	if err = survey.AskOne(&survey.Input{Message: "IMAP port:", Default: fmt.Sprintf("%d", port)}, &port); err != nil {
		return
	}
	if err = survey.AskOne(&survey.Confirm{Message: "Use TLS?", Default: useTLS}, &useTLS); err != nil {
		return
	}
	err = survey.AskOne(&survey.Input{Message: "Mailbox:", Default: mailbox}, &mailbox)
	return
}

func runAdd() error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	email, alias, password, defs, err := promptAddIdentity(*cfg)
	if err != nil {
		return err
	}
	host, port, useTLS, mailbox, err := promptAddServer(defs)
	if err != nil {
		return err
	}
	r := core.AddAccount(core.AccountInput{
		Alias: alias, Email: email, PlainPassword: password,
		ImapHost: host, ImapPort: port, UseTLS: useTLS,
		UseTLSExplicit: true, Mailbox: mailbox,
	})
	if r.HasError() {
		return r.PropagateError()
	}
	res := r.Value()
	fmt.Printf("Saved account %q to %s\n", res.Account.Alias, res.ConfigPath)
	return nil
}

func runList() error {
	r := core.ListAccounts()
	if r.HasError() {
		return r.PropagateError()
	}
	accts := r.Value()
	if len(accts) == 0 {
		fmt.Println("No accounts configured. Run `email-read add` to add one.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tEMAIL\tIMAP HOST\tPORT\tTLS\tMAILBOX")
	for _, a := range accts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\t%s\n",
			a.Alias, a.Email, a.ImapHost, a.ImapPort, a.UseTLS, a.Mailbox)
	}
	return w.Flush()
}

func runRemove(alias string) error {
	if r := core.RemoveAccount(alias); r.HasError() {
		return r.PropagateError()
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
