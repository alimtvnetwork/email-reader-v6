// errors_tail.go — `email-read errors tail` CLI subcommand.
//
// Phase 4.3 of the error-trace logging upgrade (.lovable/plan.md).
// Reads the persisted error-log ring from `<dataDir>/error-log.jsonl`,
// prints entries to stdout in oldest→newest file order, and (with -f)
// follows the file for live appends — same shape as `tail -f`.
//
// Output format mirrors what the in-app Error Log view shows: a
// header line ("[seq] timestamp  component  summary") followed by the
// indented errtrace.Format output. Two blank lines between
// entries so external tools (`grep -A`, `less`) read cleanly.
//
// We deliberately re-use `errlog.LoadFromFile` so the parser logic
// (skip corrupt lines, 1 MiB max line size, etc.) stays in one place.
// `-f` is implemented as a poll loop on file size — simpler than
// fsnotify and good enough for a 1 Hz refresh on a log that grows by
// at most a handful of entries per minute.
package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/ui/errlog"
)

// newErrorsCmd registers the `errors` command group. Today it only
// exposes `tail`; reserved for future siblings like `clear` or
// `export`.
func newErrorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "errors",
		Short: "Inspect the persisted error log (data/error-log.jsonl).",
		Long: `Tools for the on-disk error log written by the desktop UI.
The log file is the same one the in-app "Diagnostics → Error Log"
view persists across restarts.`,
	}
	cmd.AddCommand(newErrorsTailCmd())
	return cmd
}

// newErrorsTailCmd registers `email-read errors tail [--follow] [--lines N]`.
func newErrorsTailCmd() *cobra.Command {
	var (
		follow bool
		lines  int
	)
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Print the persisted error log to stdout (optionally follow new entries).",
		Long: `Read data/error-log.jsonl and print every entry in oldest→newest
order. With --follow (-f), keep the process alive and print new
entries as they're appended by the running UI — handy when reproducing
a bug in another window.

Each entry prints a header line followed by the indented
errtrace.Format output:

  [42] 2026-04-27T10:15:03Z  emails  failed to load thread: …
      at internal/ui/views/emails.go:117 (loadThread)
      at internal/store/emails.go:88 (FetchOne)
      …

Examples:
  email-read errors tail            # one-shot dump
  email-read errors tail -f         # follow live
  email-read errors tail --lines 20 # last 20 entries only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runErrorsTail(cmd.Context(), os.Stdout, follow, lines)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow the log and print new entries as they're appended")
	cmd.Flags().IntVarP(&lines, "lines", "n", 0, "print only the last N entries (0 = all)")
	return cmd
}

// errLogPathResolver is overridable in tests so we can point
// runErrorsTail at a fixture file without wrestling with config's
// executable-relative DataDir(). Production stays on config.DataDir().
var errLogPathResolver = func() (string, error) {
	dir, err := config.DataDir()
	if err != nil {
		return "", errtrace.Wrap(err, "errors tail: data dir")
	}
	return filepath.Join(dir, "error-log.jsonl"), nil
}

// runErrorsTail is the testable core: takes an io.Writer + flags so
// unit tests can capture output and avoid touching real stdout.
func runErrorsTail(parent context.Context, out io.Writer, follow bool, lines int) error {
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	path, err := errLogPathResolver()
	if err != nil {
		return errtrace.Wrap(err, "errors tail: resolve path")
	}

	entries, err := errlog.LoadFromFile(path)
	if err != nil {
		return errtrace.Wrapf(err, "errors tail: load %s", path)
	}
	if lines > 0 && len(entries) > lines {
		entries = entries[len(entries)-lines:]
	}
	if len(entries) == 0 && !follow {
		fmt.Fprintf(out, "(no entries in %s)\n", path)
		return nil
	}
	var lastSeq uint64
	for _, e := range entries {
		writeEntry(out, e)
		if e.Seq > lastSeq {
			lastSeq = e.Seq
		}
	}
	if !follow {
		return nil
	}

	logger := log.New(os.Stderr, "", 0)
	logger.Printf("(following %s — Ctrl-C to stop)", path)

	// Poll the file every second. Cheaper and simpler than fsnotify
	// for a log that grows slowly. We re-load the whole file and
	// emit every entry whose Seq > lastSeq — that way a rotation
	// (active file truncated to 0, new entries start fresh) still
	// streams correctly because Seq is monotonic across rotations.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fresh, err := errlog.LoadFromFile(path)
			if err != nil {
				// Transient read error — log to stderr, keep polling.
				logger.Printf("(read error: %v)", err)
				continue
			}
			for _, e := range fresh {
				if e.Seq <= lastSeq {
					continue
				}
				writeEntry(out, e)
				lastSeq = e.Seq
			}
		}
	}
}

// writeEntry renders one Entry in the documented header + indented
// trace format. Trailing blank line separates entries when reading
// with `less` / `grep -A`.
func writeEntry(out io.Writer, e errlog.Entry) {
	fmt.Fprintf(out, "[%d] %s  %s  %s\n",
		e.Seq,
		e.Timestamp.UTC().Format(time.RFC3339),
		nonEmpty(e.Component, "-"),
		e.Summary,
	)
	trace := strings.TrimSpace(e.Trace)
	// Skip the trace block when it's empty or identical to Summary —
	// the header already shows that text and a duplicate line is noise.
	if trace != "" && trace != strings.TrimSpace(e.Summary) {
		for _, line := range strings.Split(trace, "\n") {
			fmt.Fprintf(out, "    %s\n", line)
		}
	}
	fmt.Fprintln(out)
}

// nonEmpty returns s when non-empty, else fallback. Keeps the column
// alignment readable when an old entry was logged without a Component.
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
