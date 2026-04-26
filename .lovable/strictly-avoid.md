# Strictly avoid

Hard prohibitions for this project. Violating these will break builds, leak secrets, or contradict explicit user preferences.

- **Do NOT touch `.release/` folder.** User preference (read-only domain).
- **Do NOT modify read-only files:** `.gitignore`, `bun.lock`, `bun.lockb`, `package-lock.json`, `.lovable/user-preferences`.
- **Do NOT introduce CGO dependencies.** SQLite must remain pure-Go (`modernc.org/sqlite`) so `go build` works on any host without a C toolchain.
- **Do NOT run `go build` from the sandbox.** No Go toolchain is preinstalled and the Fyne packages need GL/X11 dev headers that the sandbox lacks. The user runs `run.ps1` locally for builds. **Exception:** `go vet -tags nofyne ./...` and `go test -tags nofyne ./...` ARE allowed via `nix run nixpkgs#go -- ...` — they cover all 16 internal packages without GL deps and are the canonical verification path for headless changes.
- **Do NOT commit plaintext passwords.** All IMAP passwords in `config.json` are Base64-encoded via `internal/config` helpers.
- **Do NOT split plans or suggestions into multiple files.** Single source of truth: `.lovable/plan.md`, `.lovable/suggestions.md`.
- **Do NOT create `.lovable/memories/`** (with trailing s). Correct path is `.lovable/memory/`.
- **Do NOT append boilerplate blocks** ("If you have any question..." or "Do you understand?..."). User preference.
- **Do NOT bump version by less than minor on code changes.** User preference: every code change bumps at least minor version of the CLI (`cmd/email-read/main.go` `Version` constant).
- **Do NOT invest in the React/Vite scaffold (`src/`)** unless explicitly asked. The product is the Go CLI.
- **Do NOT write polling/watch loops that are silent on success.** Every cycle must emit at least one heartbeat log line including a server-side counter (e.g. `messages=N uidNext=M`). See `solved-issues/02-watcher-silent-on-healthy-idle.md`.
- **Do NOT investigate the Base64 password layer when IMAP returns `AUTHENTICATIONFAILED`.** It is always a wrong-password input issue. See `solved-issues/01-imap-auth-failed-wrong-password.md`. Always verify with raw `openssl s_client` first.
