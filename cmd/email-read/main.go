package main

import (
	"fmt"
	"os"

	"github.com/lovable/email-read/internal/cli"
	"github.com/lovable/email-read/internal/errtrace"
)

// Version is the CLI version. Bumped per release.
// 0.19.0 — Phase 1 of the Fyne UI plan complete: business logic extracted to
// internal/core (accounts, rules, emails, read, export, diagnose) and the
// watcher now exposes a structured event Bus. CLI behavior unchanged.
const Version = "0.19.0"

func main() {
	root := cli.NewRoot(Version)
	if err := root.Execute(); err != nil {
		// Render the full error chain with file:line frames for each wrap.
		fmt.Fprintln(os.Stderr, errtrace.Format(err))
		os.Exit(1)
	}
}
