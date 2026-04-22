package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/exporter"
	"github.com/lovable/email-read/internal/store"
)

func newRulesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rules",
		Short: "Manage auto-open rules.",
	}
	c.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List configured rules and their enabled state.",
			RunE:  func(cmd *cobra.Command, args []string) error { return runRulesList() },
		},
		&cobra.Command{
			Use:   "enable <name>",
			Short: "Enable a rule by name.",
			Args:  cobra.ExactArgs(1),
			RunE:  func(cmd *cobra.Command, args []string) error { return runRulesToggle(args[0], true) },
		},
		&cobra.Command{
			Use:   "disable <name>",
			Short: "Disable a rule by name.",
			Args:  cobra.ExactArgs(1),
			RunE:  func(cmd *cobra.Command, args []string) error { return runRulesToggle(args[0], false) },
		},
	)
	return c
}

func runRulesList() error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	if len(cfg.Rules) == 0 {
		fmt.Println("No rules configured. Edit data/config.json to add some.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tENABLED\tFROM\tSUBJECT\tURL")
	for _, r := range cfg.Rules {
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%s\n",
			r.Name, r.Enabled, ellipsis(r.FromRegex, 28),
			ellipsis(r.SubjectRegex, 28), ellipsis(r.UrlRegex, 40))
	}
	return w.Flush()
}

func runRulesToggle(name string, enabled bool) error {
	cfg, err := config.Load()
	if err != nil {
		return errtrace.Wrap(err, "load config")
	}
	r := cfg.FindRule(name)
	if r == nil {
		return errtrace.New(fmt.Sprintf("no rule with name %q", name))
	}
	r.Enabled = enabled
	if err := config.Save(cfg); err != nil {
		return errtrace.Wrap(err, "save config")
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	fmt.Printf("Rule %q %s\n", name, state)
	return nil
}

func newExportCsvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export-csv",
		Short: "Export the Emails table to ./data/export-<timestamp>.csv (relative to cwd).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportCsv(cmd.Context())
		},
	}
}

func runExportCsv(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	st, err := store.Open()
	if err != nil {
		return errtrace.Wrap(err, "open store")
	}
	defer st.Close()
	path, err := exporter.ExportCSV(ctx, st)
	if err != nil {
		return errtrace.Wrap(err, "export csv")
	}
	fmt.Printf("Exported to %s\n", path)
	return nil
}

func ellipsis(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
