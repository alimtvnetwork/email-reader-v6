---
name: Sidebar hit-area & visual polish (Slice #213)
description: Misclick fix between adjacent nav rows; padded row template + active-row caret + indented headers; pure helper SidebarRowText for headless tests.
type: feature
---
# Slice #213 — Sidebar hit-area & visual polish (LOCKED 2026-04-29)

## RCA recap (user-reported "Diagnose opens when I click Error Log")
- The old sidebar list template was `widget.NewLabel("template")` —
  Fyne reported MinSize ≈ 14px tall at default density, while the
  painted row box stretched to ~28px, so users could only "feel"
  the click target on the top half of every row.
- No "you are here" indicator: theme's row-highlight tint is subtle
  on dark mode, so users had no pre-click confirmation of which
  row they were aiming at.
- Group header rows ("Diagnostics") looked identical-but-italic to
  nav rows, providing no visual sectioning cue.

## What changed
- `internal/ui/sidebar.go`:
  - Row template now wraps the label in `container.New(layout.NewPaddedLayout(), lbl)`
    so every row gets `theme.Padding()*2` extra vertical breathing
    room (consistent ~24-30px hit target across the whole list).
  - New `activeRow int` tracked in the binder closure; `OnSelected`
    updates it then `list.Refresh()` to repaint the caret prefix.
  - Binder closure delegates to new `renderSidebarRow` helper.
  - Pre-selection loop also seeds `activeRow` so the caret is
    correct on first render.
- `internal/ui/sidebar_render.go` (NEW, fyne-free):
  - `SidebarRowText(row, items, badgeFor, active) string` — single
    canonical formatter:
    - Header rows: `"  " + name` (2-space inset, italic+bold style).
    - Nav rows (active): `"▸ " + label` + bold style.
    - Nav rows (inactive): `"  " + label` (caret-width inset so
      active/inactive rows share a left-edge column — no horizontal
      jitter when selection moves).
- `internal/ui/sidebar_render_test.go` (NEW, headless): locks all
  four formatting paths + out-of-range guard.

## Verification
- `nix run nixpkgs#go -- test -tags nofyne ./...` — all 22 packages green.
- `internal/ui` tests: 5 new SidebarRowText cases pass.
- Existing `formatNavRowLabel` tests untouched (badge suffix logic
  is wrapped, not replaced).

## Lessons re-confirmed
- Fyne `widget.List` row hit-area = template MinSize. Wrap labels
  in a padded container — don't try `SetItemHeight` per-row, that
  fights with the virtualization scroll math.
- Pre-click affordance > post-click highlight. The "▸" caret is a
  zero-cost UX win in dark themes where the theme's highlight tint
  is barely visible.

## Roadmap context
**Phase 2 of 6** in the user-locked UX redesign roadmap (2026-04-29).
**Remaining queued — execute on "next":**
  - **P3** — Error Log per-row Copy + per-row Delete (in-memory only).
  - **P4** — Dashboard redesign → card-grid layout.
  - **P5** — Accounts redesign → card-rows.
  - **P6** — Theme polish (focus ring, hover bg, button consistency).
