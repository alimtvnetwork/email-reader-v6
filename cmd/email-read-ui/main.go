// Command email-read-ui is the Fyne desktop frontend for email-read.
//
// All real logic lives in internal/ui so that package can be unit-tested
// without the cgo display libraries that linking this binary requires.
//
// Run locally:
//
//	go run ./cmd/email-read-ui
//
// Cross-compile single-file binaries (handled by Step 27):
//
//	fyne package -os darwin  -src ./cmd/email-read-ui
//	fyne package -os windows -src ./cmd/email-read-ui
//	fyne package -os linux   -src ./cmd/email-read-ui
//
// Build tag: gated off when -tags nofyne is passed so headless CI can run
// `go test ./...` (the rest of the tree) without the cgo display libs.
//go:build !nofyne

package main

import "github.com/lovable/email-read/internal/ui"

func main() { ui.Run() }
