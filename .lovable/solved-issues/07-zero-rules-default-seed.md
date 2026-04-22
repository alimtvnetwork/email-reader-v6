# 07 — `0 enabled rules` blocked incognito open even when browser was ready

## Symptom
```
[admin] browser ready: /Applications/Google Chrome.app/.../Google Chrome (incognito flag="--incognito")
[admin] ✉ 1 new message(s)
    rules: 0 enabled rules — nothing to evaluate (add one in data/config.json)
```
User saw email arrive, file saved with the new readable name, but no link
opened in incognito. Cause: `data/config.json` had `rules: []`, and we
required the user to hand-edit JSON to add one.

## Fix (v0.17.0)
1. **Auto-seed** `default-open-any-url` rule in both `runWatch` and `runRead`
   when `countEnabledRules(cfg.Rules) == 0`. Regex:
   `https?://[^\s<>"'\)\]]+` — opens any http(s) URL in any incoming email.
2. **`rules add`** non-interactive command for custom rules:
   `email-read rules add --name X --url-regex '...'` (plus optional
   `--from-regex`, `--subject-regex`, `--body-regex`, `--disabled`).
3. Logged a clear `ℹ no enabled rules found — seeded default rule` line so
   the user knows the seed happened and where the file lives.

## Files
- `internal/cli/cli.go` — seed before building engine in `runWatch`.
- `internal/cli/read.go` — same seed in `runRead`.
- `internal/cli/rules_export.go` — `countEnabledRules` helper + `rules add`.
- `cmd/email-read/main.go` — version → `0.17.0`.

## Next time
Rebuild: `.\run.ps1` (or the user's Mac equivalent). Then just
`email-read watch admin` — no JSON editing required.
