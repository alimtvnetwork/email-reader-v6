# 01 — Tokens

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

Every design token used by the Fyne UI: colors (Dark + Light), typography sizes, spacing scale, corner radii, elevation, and motion. Tokens are exposed as Go constants in `internal/ui/theme/tokens.go` and consumed via `theme.Color(...)`, `theme.Size(...)`, etc.

Every numeric value here is normative. Adding a new token requires bumping this spec.

Cross-references:
- Implementation contract: [`./02-theme-implementation.md`](./02-theme-implementation.md)
- Consolidated source of truth: [`../12-consolidated-guidelines/16-app-design-system-and-ui.md`](../12-consolidated-guidelines/16-app-design-system-and-ui.md)

---

## 1. Naming Convention

| Token kind | Go constant prefix | Example |
|---|---|---|
| Color | `Color…` | `ColorPrimary`, `ColorWatchDotWatching` |
| Size (px) | `Size…` | `SizePadding`, `SizeTextBody` |
| Radius | `Radius…` | `RadiusCard`, `RadiusButton` |
| Elevation (shadow) | `Elev…` | `ElevCard`, `ElevDialog` |
| Motion (duration ms) | `Motion…` | `MotionFast`, `MotionPulse` |

All constants are typed: `type ColorName string`, `type SizeName string`, etc. — never raw `string`. Compile-time misspellings impossible.

---

## 2. Color Tokens

### 2.1 Semantic Surface & Foreground

Values are HSL-equivalent `color.NRGBA` (alpha = 255 unless noted). Each token has both `Dark` and `Light` variants — selection is driven by the active `ThemeMode` (see Settings spec).

| Token | Role | Dark (`R G B`) | Light (`R G B`) |
|---|---|---|---|
| `ColorBackground` | Window/canvas | `15  17  21` | `250 250 252` |
| `ColorSurface` | Card / panel | `23  25  31` | `255 255 255` |
| `ColorSurfaceMuted` | Subdued panel | `30  33  40` | `244 245 248` |
| `ColorBorder` | 1 px separators | `46  49  58` | `224 226 232` |
| `ColorForeground` | Primary text | `235 237 242` | `15  17  21` |
| `ColorForegroundMuted` | Secondary text | `155 160 170` | `90  96  110` |
| `ColorForegroundDisabled` | Disabled text | `90  96  110` | `170 175 185` |

### 2.2 Brand & Action

| Token | Role | Dark | Light |
|---|---|---|---|
| `ColorPrimary` | Primary action / focus ring | `82  136 255` | `42  100 245` |
| `ColorPrimaryForeground` | Text on Primary | `255 255 255` | `255 255 255` |
| `ColorAccent` | Highlight / link | `170 130 255` | `122  82  220` |

### 2.3 Status

| Token | Role | Dark | Light |
|---|---|---|---|
| `ColorSuccess` | Success badge / dot | `74  200 130` | `34  160  90` |
| `ColorWarning` | Warning badge / dot | `240 175  60` | `200 140  20` |
| `ColorError`   | Error text / dot   | `240  90 105` | `200  40  60` |
| `ColorInfo`    | Info badge / dot   | `82  170 240` | `30  130 210` |

### 2.4 Sidebar

| Token | Dark | Light |
|---|---|---|
| `ColorSidebar` | `19  21  26` | `247 248 251` |
| `ColorSidebarForeground` | `200 205 215` | `60  66  78` |
| `ColorSidebarItemHover` | `34  37  46` | `230 233 240` |
| `ColorSidebarItemActive` | `46  56  90` | `220 230 252` |
| `ColorSidebarItemActiveForeground` | `255 255 255` | `42  100 245` |
| `ColorSidebarBorder` | `46  49  58` | `224 226 232` |

### 2.5 Watch — status dots (resolves OI-1)

Used by the `WatchDot` widget (see `04-components.md` §6). Animation behavior is fixed in `04-…` §6.

| Token | Dark | Light | Notes |
|---|---|---|---|
| `ColorWatchDotIdle` | `120 125 135` (grey 500) | `140 145 155` | static |
| `ColorWatchDotStarting` | `100 170 240` (blue 400) | `60  140 220` | pulses 2 Hz |
| `ColorWatchDotWatching` | `74  200 130` (green 500) | `34  160  90` | static |
| `ColorWatchDotReconnecting` | `240 175  60` (amber 500) | `200 140  20` | pulses 1 Hz |
| `ColorWatchDotStopping` | `145 195 245` (blue 300) | `90  170 230` | pulses 2 Hz |
| `ColorWatchDotError` | `240  90 105` (red 500) | `200  40  60` | static |

### 2.6 Watch — raw log (resolves OI-1)

| Token | Dark | Light | Use |
|---|---|---|---|
| `ColorRawLogHeartbeat` | `90  96  110` (grey 400, dim) | `150 155 165` | per-poll heartbeat lines |
| `ColorRawLogNewMail`   | `235 237 242` (foreground full) | `15  17  21` | new-message lines |
| `ColorRawLogError`     | `240  90 105` (red 400) | `200  40  60` | error lines |
| `ColorRawLogTimestamp` | `120 125 135` (grey 500) | `140 145 155` | leading `[hh:mm:ss]` prefix |

### 2.7 Rules / badges

| Token | Dark | Light | Use |
|---|---|---|---|
| `ColorRuleMatchBadge` | `170 130 255` (accent purple) | `122  82  220` | "matched" badge in Cards tab |
| `ColorBadgeNeutralBg` | `46  49  58` | `224 226 232` | neutral chip background |
| `ColorBadgeNeutralFg` | `200 205 215` | `60  66  78` | neutral chip text |

### 2.8 Code / monospace surfaces

| Token | Dark | Light | Use |
|---|---|---|---|
| `ColorCodeBg` | `19  21  26` | `244 245 248` | `<pre>`-style blocks (raw email, log viewer) |
| `ColorCodeBorder` | `46  49  58` | `224 226 232` | code block border |
| `ColorCodeLineHighlight` | `30  33  40` | `234 240 250` | hovered line in raw log |
| `ColorCodeSelection` | `46  72 130` | `190 215 250` | text selection inside code |

### 2.9 Tools — Diagnostics step dots (resolves OI-1, Tools 06)

Used by the diagnostics-step indicator in `internal/ui/views/tools_diagnose.go`. Mapped from `core.DiagnosticsStep*` enum values via the table in `spec/21-app/02-features/06-tools/02-frontend.md` §3.

| Token | Dark | Light | Use |
|---|---|---|---|
| `ColorDiagStepPending` | `120 125 135` (grey 400) | `170 175 185` | step queued, not yet started |
| `ColorDiagStepRunning` | `100 170 240` (blue 400) | `60  140 220` | spinner active; pulses 2 Hz |
| `ColorDiagStepPass`    | `74  200 130` (green 500) | `34  160  90` | step succeeded |
| `ColorDiagStepFail`    | `240  90 105` (red 500)   | `200  40  60` | step failed |
| `ColorDiagStepSkipped` | `90  96  110` (grey 300)  | `190 195 205` | step skipped because a prior dependency failed |

### 2.10 Tools — OpenUrl provenance badges (resolves OI-1, Tools 06)

Used by the OpenUrl provenance chip in `internal/ui/views/tools_openurl.go` and the recent-opened-urls table. Selection is driven by `core.OpenedUrl.Origin`.

| Token | Dark | Light | Use |
|---|---|---|---|
| `ColorOpenUrlSafe`    | `74  200 130` (green 500) | `34  160  90` | green chip — `OriginRule` (allow-listed by a saved rule) |
| `ColorOpenUrlManual`  | `240 175  60` (amber 500) | `200 140  20` | amber chip — `OriginManual` (operator pasted/typed) |
| `ColorOpenUrlDeduped` | `100 170 240` (blue 400)  | `60  140 220` | blue chip — request collapsed via `IxOpenedUrlsUnique` |

### 2.11 Token totals

47 color tokens × 2 variants = 94 concrete values. Adding any token requires updating both variants AND `97-acceptance-criteria.md` parity table.

### 2.12 Named-alias carve-out (AC-DS-05 narrowing)

A naive "no two tokens share the same RGB triple in the same variant" rule (the original AC-DS-05 wording) is **too strong**: the palette intentionally re-uses semantic values across categories so that a status concept (e.g. "watcher is healthy") and the underlying status color (e.g. `Success`) stay visually unified — changing one MUST change both. Forcing every duplicate to be a unique RGB triple would split the palette into ~13 fork colors that drift apart over time.

**Rule (replaces the naive form):**

> No two distinct tokens share the same RGB triple in the same variant **unless the pair is listed in the alias registry below**. The registry is normative — adding or removing a row requires bumping this spec's MINOR version. The acceptance test (`Test_Tokens_NoDuplicateValues`) reads from the same registry that ships in `internal/ui/theme/aliases.go::NamedAliases`, so spec and code cannot drift.

**Why a registry (not a per-token `AliasOf` field):** keeping the alias relation in one auditable table makes review trivial ("did this PR add a new alias? show diff of `aliases.go`") and lets the test file present a precise failure: *"WatchDotWatching duplicates Success in Dark — this pair is **not** in NamedAliases. Either register the alias or pick a distinct RGB."* A per-token field scatters the same information across 13 sites and makes the "add one token, accidentally collide" failure mode silent.

**Alias registry — pairs are unordered; each row is one allowed RGB collision in the named scope.**

The registry is split into three scopes:
- **Both** — pair shares RGB in **both** Dark and Light variants. Most-stable form.
- **DarkOnly** — pair shares RGB in Dark only; Light variants differ deliberately (e.g. Slice #118d palette tunes for WCAG).
- **LightOnly** — pair shares RGB in Light only; Dark variants differ deliberately.

When several tokens form a clique (`A=B=C` in some variant) the registry lists every unordered pair (`(A,B), (A,C), (B,C)`) so the test message can name the precise colliding pair.

**Both-variant aliases (13 pairs):**

| Pair | Why aliased |
|---|---|
| `ColorAccent` ↔ `ColorRuleMatchBadge` | Rule-match badge inherits the accent hue. |
| `ColorError` ↔ `ColorRawLogError` | Error log lines = Error-red. |
| `ColorError` ↔ `ColorWatchDotError` | Error dot = Error-red. |
| `ColorRawLogError` ↔ `ColorWatchDotError` | Implied by the `Error` 3-clique above; listed for pair-precise test messages. |
| `ColorWarning` ↔ `ColorWatchDotReconnecting` | Reconnecting = Warning-amber. |
| `ColorForeground` ↔ `ColorRawLogNewMail` | New-mail lines render at full text contrast. |
| `ColorBorder` ↔ `ColorSidebarBorder` | Sidebar separator uses the standard 1 px border. |
| `ColorBorder` ↔ `ColorCodeBorder` | Code surfaces use the standard 1 px border. |
| `ColorBorder` ↔ `ColorBadgeNeutralBg` | Neutral badge sits on the same tone as borders. |
| `ColorSidebarBorder` ↔ `ColorCodeBorder` | Implied by the `Border` 4-clique; listed for pair precision. |
| `ColorSidebarBorder` ↔ `ColorBadgeNeutralBg` | Implied by the `Border` 4-clique; listed for pair precision. |
| `ColorCodeBorder` ↔ `ColorBadgeNeutralBg` | Implied by the `Border` 4-clique; listed for pair precision. |
| `ColorBadgeNeutralFg` ↔ `ColorSidebarForeground` | Neutral badge text matches sidebar muted text. |

**Dark-only aliases (5 pairs):**

| Pair | Why aliased (Dark) | Why distinct in Light |
|---|---|---|
| `ColorSuccess` ↔ `ColorWatchDotWatching` | Healthy watcher = Success-green. | Light `ColorSuccess` was darkened by Slice #118d (WCAG `Success on Background`); the watch-dot kept the brighter green for at-a-glance scanning. |
| `ColorForegroundDisabled` ↔ `ColorRawLogHeartbeat` | Heartbeat lines fade to disabled-text grey. | Light heartbeat is intentionally lifted (150 vs 170 RGB) for legibility on the lighter `ColorCodeBg`. |
| `ColorPrimaryForeground` ↔ `ColorSidebarItemActiveForeground` | Both are pure white on Dark. | Light `SidebarItemActiveForeground` becomes `ColorPrimary` blue (active item on a light card needs hue, not white-on-blue). |
| `ColorSidebar` ↔ `ColorCodeBg` | Both use the deep `(19,21,26)` panel tone on Dark. | Light `ColorSidebar` is a tinted off-white; `ColorCodeBg` stays a soft grey for code legibility. |
| `ColorSurfaceMuted` ↔ `ColorCodeLineHighlight` | Both use the subdued panel tone on Dark. | Light `ColorCodeLineHighlight` is a soft blue-tint to disambiguate from regular muted surfaces. |

**Light-only aliases (4 pairs):**

| Pair | Why aliased (Light) | Why distinct in Dark |
|---|---|---|
| `ColorPrimaryForeground` ↔ `ColorSurface` | Both are pure white on Light (cards on white pages). | Dark `ColorSurface` is the dark card tone, not white. |
| `ColorPrimary` ↔ `ColorSidebarItemActiveForeground` | Active item label = primary-action blue on Light. | Dark uses pure white text on the active sidebar item (see Dark-only counterpart). |
| `ColorSurfaceMuted` ↔ `ColorCodeBg` | Code surfaces match the muted panel tone on Light. | Dark `ColorCodeBg` matches `ColorSidebar` instead. |
| `ColorWatchDotIdle` ↔ `ColorRawLogTimestamp` | Both use the same neutral grey on Light. | Dark `ColorRawLogTimestamp` was lifted by Slice #118d to (140,145,155) for WCAG against `ColorCodeBg`; `ColorWatchDotIdle` kept (120,125,135). |

**22 registered aliases total** (13 + 5 + 4). The registry validator in `aliases.go` enforces (a) every Both-pair holds in BOTH variants, (b) every DarkOnly-pair holds in Dark AND fails in Light (asymmetry is a test signal — a "DarkOnly" row that accidentally became symmetric must be promoted to Both, otherwise the registry is misleading), (c) symmetric for LightOnly, (d) no reflexive entries, (e) pair canonicalization (unordered: `(A,B)` and `(B,A)` collapse to one row).



**Test contract:** `Test_Tokens_NoDuplicateValues` (AC-DS-05)
1. For every pair `(a, b)` of distinct `ColorName`s and every variant `v ∈ {Dark, Light}`:
   - if `palette[v][a] == palette[v][b]` and `(a, b)` is **not** in `NamedAliases` → fail with the precise pair + variant.
2. For every entry in `NamedAliases`:
   - if the alias is "both variants": both Dark and Light RGB triples MUST be equal — otherwise the alias is bogus and fails.
   - if the alias is "Dark only" / "Light only": the named variant MUST be equal AND the other variant MUST be **distinct** — otherwise the asymmetry tag is wrong.
3. No reflexive entries (`(X, X)`) and no duplicate pairs (canonicalized as alphabetical).

This shape moves from binary to **registry-aware**: the test cannot be silenced by adding a duplicate, it can only be satisfied by either picking a distinct RGB or explicitly registering the alias with a one-line rationale review.

---

## 3. Typography

Fonts ship in-binary via `theme.TextFont()`/`theme.TextMonospaceFont()`.

| Family | Source | Used for |
|---|---|---|
| **Inter** (Variable) | `internal/ui/theme/fonts/Inter-Variable.ttf` (OFL) | All UI text |
| **JetBrains Mono** (Variable) | `internal/ui/theme/fonts/JetBrainsMono-Variable.ttf` (OFL) | Code, raw log, monospace fields |

### 3.1 Type scale

| Token | Size (px) | Weight | Line height | Use |
|---|---|---|---|---|
| `SizeTextPageTitle`    | 30 | 700 | 1.20 | Page titles (Dashboard, Settings…) |
| `SizeTextSectionTitle` | 20 | 600 | 1.30 | Card-section titles |
| `SizeTextCardTitle`    | 16 | 600 | 1.40 | Card headers |
| `SizeTextBody`         | 14 | 400 | 1.55 | Default body text |
| `SizeTextCaption`      | 12 | 400 | 1.45 | Captions, timestamps, helper text |
| `SizeTextCode`         | 13 | 400 | 1.65 | Raw log, code surfaces |
| `SizeTextButton`       | 14 | 500 | 1.20 | Button label |

Fyne `theme.SizeName` mapping in `02-theme-implementation.md` §3 — the Fyne built-ins (`SizeNameText`, `SizeNameHeadingText`, `SizeNameSubHeadingText`, `SizeNameCaptionText`) are routed to the closest entry above; custom sizes (`SizeTextPageTitle` etc.) are exposed via a typed accessor.

### 3.2 Tabular numerics

All numeric columns (e.g., Email row count, OpenedUrl count) use the `tnum` OpenType feature flag set on Inter via `text.NewWithFeatures("…", "tnum")`. Required so right-aligned counts don't jitter on update.

---

## 4. Spacing

4 px base unit. All inter-element gaps and inner padding MUST snap to this scale.

| Token | px | Use |
|---|---|---|
| `SizeSpacing0` | 0  | reset |
| `SizeSpacing1` | 4  | tight icon-to-text |
| `SizeSpacing2` | 8  | default form gap |
| `SizeSpacing3` | 12 | sidebar item padding |
| `SizeSpacing4` | 16 | between cards |
| `SizeSpacing5` | 24 | card internal padding |
| `SizeSpacing6` | 32 | between page sections |
| `SizeSpacing7` | 48 | hero / empty-state padding |

---

## 5. Corner Radius

| Token | px | Use |
|---|---|---|
| `RadiusNone`   | 0  | dividers, full-width elements |
| `RadiusSmall`  | 4  | input, badge, chip |
| `RadiusButton` | 6  | all standard buttons |
| `RadiusCard`   | 10 | cards, panels |
| `RadiusDialog` | 14 | modal dialogs |
| `RadiusPill`   | 999 | status pills, watch dots (rendered as circles) |

---

## 6. Elevation

Fyne does not natively render box-shadows; elevation is implemented as a 1 px outer border + a translucent shadow rectangle behind the element (`canvas.NewRectangle` with low alpha). The token captures the alpha and offset only.

| Token | Border alpha | Shadow alpha | Y-offset px | Use |
|---|---|---|---|---|
| `ElevFlat`    | 0    | 0    | 0 | base, no chrome |
| `ElevCard`    | 0.10 | 0.06 | 1 | default card |
| `ElevHover`   | 0.14 | 0.10 | 2 | hover state |
| `ElevPopover` | 0.18 | 0.18 | 4 | dropdowns, popovers |
| `ElevDialog`  | 0.22 | 0.28 | 8 | modal dialogs |

Implementation helper: `widgets.WithElevation(child, theme.ElevCard)` returns a `*fyne.Container` with the layered rectangle.

---

## 7. Motion

| Token | Duration (ms) | Easing | Use |
|---|---|---|---|
| `MotionInstant`  | 0   | n/a                          | reduced-motion override |
| `MotionFast`     | 120 | `fyne.AnimationEaseOut`      | hover, focus ring |
| `MotionMedium`   | 220 | `fyne.AnimationEaseInOut`    | tab switch, card expand |
| `MotionSlow`     | 360 | `fyne.AnimationEaseInOut`    | view transitions |
| `MotionPulse1Hz` | 1000 (period) | `fyne.AnimationEaseInOut` | `WatchDot` reconnecting |
| `MotionPulse2Hz` |  500 (period) | `fyne.AnimationEaseInOut` | `WatchDot` starting / stopping |

`internal/ui/anim/pulse.go` is the **only** caller of `canvas.NewColorRGBAAnimation` for status dots (Watch frontend §3.5). Verified by AST scan.

When `accessibility.PrefersReducedMotion()` returns true (`05-accessibility.md` §5), all motion tokens collapse to `MotionInstant` and pulses become solid colors.

---

## 8. Density Modes

The user can pick **Comfortable** (default) or **Compact** (Settings → Appearance, future MINOR — tracked OI-DS-1). Compact multiplies all `SizeSpacing*` and `SizeText*` by `0.875` (rounded to nearest px). Token IDs do not change; only the resolved value differs. The theme's `Size(name)` method reads the active density at call time.

Today (v1.0.0) only Comfortable ships. The plumbing is reserved.
