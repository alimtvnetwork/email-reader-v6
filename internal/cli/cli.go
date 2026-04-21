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
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

// NewRoot builds the root cobra command. The watch behavior (default subcommand
// when an alias is given) is wired in a later step.
func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "email-read [alias]",
		Short:         "Watch IMAP inboxes and auto-open links from matching emails.",
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
	root.AddCommand(newAddCmd(), newListCmd(), newRemoveCmd(),
		newWatchCmd(), newRulesCmd(), newExportCsvCmd())
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

// runWatch wires config → store → rules → browser → watcher and runs
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
