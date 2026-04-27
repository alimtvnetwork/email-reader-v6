---
suggestionId: 20260427-rotate-seeded-credentials-broader-scope
createdAt: 2026-04-27T15:50:00Z
source: Lovable (Slice #185 finding)
affectedProject: email-read
status: open
priority: HIGH (security)
---

## Description
The "Rotate seeded credentials in spec" suggestion (originally filed 2026-04-21 in `.lovable/suggestions.md`) is **broader than just the legacy spec**. Slice #185 audit found PII in **3 locations**, only 1 of which is sandbox-safe to redact:

| File | Line | Content | Sandbox-safe to redact? |
|---|---|---|---|
| `spec/21-app/legacy/spec.md` | 119–121 | alias `atto`, email `lovable.admin@attobondcleaning.store`, host `mail.attobondcleaning.store` | ✅ **YES** — pure documentation, no code path reads this. **DONE in Slice #185.** |
| `internal/config/seed.go` | 28–34 | Same alias/email/host **+ plaintext password `ZPb*sz=d!cEE_Wgc` wrapped in U+2060 chars (line 33)** | ❌ **NO** — ships in the binary as `DefaultSeedAccounts[0]`. Removing breaks the seed-on-fresh-install feature without the user's consent. **Needs user action.** |
| `internal/imapdef/imapdef.go` | 50–54 + `imapdef_test.go` 16–23 | `SeedAccount()` returns same alias/email/host (NO password). Test asserts the host. | ⚠️ **PARTIAL** — code change is safe IF user agrees the seed account should change. Test would need to update in lockstep. |

## Why this is HIGH priority
Line 33 of `internal/config/seed.go` ships a **real, plaintext IMAP app password** in every compiled binary. Anyone with read access to the GitHub repo OR a built artefact has full inbox access to `lovable.admin@attobondcleaning.store`. The U+2060 wrappers stripped by `SanitizePassword` are a code defence against a different bug (chat copy-paste) — they do NOT obscure the password from a casual `grep`.

## Required user action (cannot be done by AI)
1. **Rotate the IMAP app password** for `lovable.admin@attobondcleaning.store` in the upstream provider (cPanel / Mailcow / etc.) — invalidates the leaked value.
2. **Decide the new seed policy** (one of):
   * **(a) Remove seed entirely.** `DefaultSeedAccounts = []Account{}` — fresh installs get an empty list; user must add their own account. Cleanest.
   * **(b) Replace with a placeholder.** Email `you@example.com`, no password. Forces user to edit before first run, but discoverable.
   * **(c) Keep but rotate + redact.** Replace email/host with a project-owned dev mailbox, password injected via build flag `-ldflags '-X internal/config.SeedPassword=...'`. Most work, most professional.
3. **Confirm rename of `SeedAccount()` return** in `internal/imapdef/imapdef.go` (drop the personal alias).

## After user verdict, the AI-doable cleanup is one slice
* Edit `internal/config/seed.go` per chosen policy.
* Edit `internal/imapdef/imapdef.go` `SeedAccount()` to match.
* Edit `internal/imapdef/imapdef_test.go` assertions.
* Run `nix run nixpkgs#go -- test -tags nofyne ./internal/config/ ./internal/imapdef/` to verify.

## Slice #185 status
* ✅ Legacy spec PII redacted (3 cells in `spec/21-app/legacy/spec.md` table).
* ❌ Source-code rotation **NOT executed** — needs user verdict (a) / (b) / (c) above.
