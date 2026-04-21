package main

import (
	"fmt"
	"os"

	"github.com/lovable/email-read/internal/cli"
)

// Version is the CLI version. Bumped per release.
const Version = "0.7.0"

func main() {
	root := cli.NewRoot(Version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
