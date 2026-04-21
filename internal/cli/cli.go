// Package cli wires the Cobra command tree for the email-read CLI.
package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/imapdef"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

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
			return runWatch(cmd.Context(), alias)
		},
	}

	// Set custom help template for cleaner output
	root.SetHelpTemplate(helpTemplate)
	root.SetUsageTemplate(usageTemplate)

	root.AddCommand(newAddCmd(), newListCmd(), newRemoveCmd(),
		newWatchCmd(), newDiagnoseCmd(), newRulesCmd(), newExportCsvCmd())
	return root
}

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
			return runWatch(cmd.Context(), alias)
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
		return err
	}
	if len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured. Run `email-read add` first")
	}

	var acct config.Account
	if alias == "" {
		acct = cfg.Accounts[0]
		fmt.Printf("No alias given — diagnosing first configured account %q\n", acct.Alias)
	} else {
		p := cfg.FindAccount(alias)
		if p == nil {
			return fmt.Errorf("no account with alias %q (run `email-read list`)", alias)
		}
		acct = *p
	}

	cfgPath, _ := config.Path()
	fmt.Println("IMAP diagnostic report")
	fmt.Printf("Config:  %s\n", cfgPath)
	fmt.Printf("Alias:   %s\n", acct.Alias)
	fmt.Printf("Account: %s\n", acct.Email)
	fmt.Printf("Server:  %s:%d tls=%v mailbox=%q\n", acct.ImapHost, acct.ImapPort, acct.UseTLS, acct.Mailbox)

	start := context.Background()
	_ = start
	fmt.Println("\n1) Connecting and logging in...")
	mc, err := mailclient.Dial(acct)
	if err != nil {
		return err
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
		return err
	}
	fmt.Printf("   OK: %q messages=%d recent=%d unseen=%d uidNext=%d uidValidity=%d\n",
		stats.Name, stats.Messages, stats.Recent, stats.Unseen, stats.UidNext, stats.UidValidity)

	fmt.Println("\n4) Recent headers from configured mailbox...")
	headers, err := mc.FetchRecentHeaders(stats, 10)
	if err != nil {
		return err
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

	fmt.Println("\nDiagnosis:")
	if stats.Messages <= 1 && stats.UidNext <= 2 {
		fmt.Println("   The IMAP server is still exposing only the baseline message in this mailbox.")
		fmt.Println("   If your mail UI shows a newer Gmail message, it is not visible in this IMAP mailbox/folder yet.")
		fmt.Println("   Check Spam/Junk/All Mail folders, recipient spelling, and domain MX/routing in your mail host.")
	} else {
		fmt.Println("   The server has more mail than the watcher baseline; run `email-read watch <alias>` to process UID > LastUid.")
	}
	return nil
}


// until SIGINT/SIGTERM. Empty alias picks the first configured account.
func runWatch(parent context.Context, alias string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured. Run `email-read add` first")
	}

	var acct config.Account
	if alias == "" {
		acct = cfg.Accounts[0]
		fmt.Printf("No alias given — using first configured account %q\n", acct.Alias)
	} else {
		p := cfg.FindAccount(alias)
		if p == nil {
			return fmt.Errorf("no account with alias %q (run `email-read list`)", alias)
		}
		acct = *p
	}

	st, err := store.Open()
	if err != nil {
		return err
	}
	defer st.Close()

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

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("press Ctrl+C to stop")

	return watcher.Run(ctx, watcher.Options{
		Account:     acct,
		PollSeconds: cfg.Watch.PollSeconds,
		Engine:      engine,
		Launcher:    launcher,
		Store:       st,
		Logger:      logger,
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
		return err
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
		return err
	}
	p, _ := config.Path()
	fmt.Printf("Saved account %q to %s\n", alias, p)
	return nil
}

func runList() error {
	cfg, err := config.Load()
	if err != nil {
		return err
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
		return err
	}
	if !cfg.RemoveAccount(alias) {
		return fmt.Errorf("no account with alias %q", alias)
	}
	if err := config.Save(cfg); err != nil {
		return err
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
