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

### 2.9 Token totals

39 color tokens × 2 variants = 78 concrete values. Adding any token requires updating both variants AND `97-acceptance-criteria.md` parity table.

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
