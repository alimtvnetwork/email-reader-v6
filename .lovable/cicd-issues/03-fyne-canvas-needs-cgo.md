# 03 — Fyne UI requires cgo + display server

## Description
The Fyne UI (`cmd/email-read-ui` + `internal/ui/`) needs cgo, OpenGL,
and a display server (X11/Wayland on Linux, Cocoa on macOS, Win32 on
Windows). The Lovable sandbox has none of these, so canvas-bound tests
cannot run here.

## Impact
- ~50 acceptance rows (AC-SF 21, AC-DS canvas ~22, AC-SX-06 frontend 1,
  some AC-PROJ E2E) are blocked from being closed in this sandbox.
- All sandbox `go` invocations must use `-tags nofyne` to skip Fyne
  imports and exercise the headless code paths only.

## Workaround
- Headless code paths are verified under `-tags nofyne`.
- Canvas-bound rows are tracked in the deferred bucket
  "Slice #118e — Fyne canvas harness" in `mem://workflow/01-status`.
- User runs the binary on real hardware for the boot smoke test
  (procedure in `mem://decisions/05-desktop-run-procedure`).

## Status
🚫 Blocked in sandbox; no workaround possible without a workstation
with cgo + GL.
