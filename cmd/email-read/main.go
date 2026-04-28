package main

import (
	"fmt"
	"os"

	"github.com/lovable/email-read/internal/cli"
	"github.com/lovable/email-read/internal/errtrace"
)

// Version is the CLI version. Bumped per release in lockstep with
// internal/ui.AppVersion so both binaries advertise the same release.
// 0.28.0 — Slice #40–#43: Edit Rule form, app boot smoke test, Density
// preference persistence, Dashboard auto-refresh on EventUrlOpened.
const Version = "0.29.0"

func main() {
	root := cli.NewRoot(Version)
	if err := root.Execute(); err != nil {
		// Render the full error chain with file:line frames for each wrap.
		fmt.Fprintln(os.Stderr, errtrace.Format(err))
		os.Exit(1)
	}
}
