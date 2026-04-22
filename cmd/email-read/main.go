package main

import (
	"fmt"
	"os"

	"github.com/lovable/email-read/internal/cli"
	"github.com/lovable/email-read/internal/errtrace"
)

// Version is the CLI version. Bumped per release in lockstep with
// internal/ui.AppVersion so both binaries advertise the same release.
// 0.23.0 — Phase 3 / Step 12: Emails view (list + detail + clickable links).
const Version = "0.23.0"

func main() {
	root := cli.NewRoot(Version)
	if err := root.Execute(); err != nil {
		// Render the full error chain with file:line frames for each wrap.
		fmt.Fprintln(os.Stderr, errtrace.Format(err))
		os.Exit(1)
	}
}
