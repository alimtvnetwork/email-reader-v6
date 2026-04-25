# Spec — Golang Email Reader CLI (`email-read`)

## 1. Overview
A small Windows-first Go CLI named **`email-read`** that:
- Manages multiple IMAP email accounts via a JSON config (with aliases).
- Watches an inbox in near real-time and persists every email into SQLite + filesystem.
- Auto-opens URLs from incoming emails in **Chrome incognito** based on user-defined regex rules.
- Exports the SQLite store to CSV on demand.
- Ships with a PowerShell bootstrap script that `git pull`s, builds the EXE, and registers it on `PATH`.

## 2. Deployment Layout

Everything is portable. The deployed folder structure (created by the PowerShell script) is:

```
email-reader-cli/                <- deploy root, added to PATH
├── email-read.exe               <- the CLI binary
├── data/
│   ├── config.json              <- accounts + rules
│   └── emails.db                <- SQLite store
└── email/
    └── <alias>/
        └── <YYYY-MM-DD>/
            └── <MessageId>.eml  <- raw saved emails
```

- `data/` and `email/` live **next to the EXE**.
- The PS1 ensures `email-reader-cli/` is in the user `PATH` so `email-read` works from any directory.

## 3. PowerShell Bootstrap (`run.ps1` at repo root)

Responsibilities:
1. `git pull` latest in the repo.
2. `go build -o ./email-reader-cli/email-read.exe ./cmd/email-read`.
3. Ensure `data/` and `email/` folders exist inside `email-reader-cli/`.
4. Add the absolute path of `email-reader-cli/` to the **user** `PATH` env var (idempotent — skip if already present).
5. Print a success line with the deploy path.

## 4. CLI Surface

Binary name: `email-read`

| Command | Description |
|---|---|
| `email-read add` | Interactive prompt: email, password, alias. Auto-detects IMAP server from domain; user can override host/port/TLS. Saves to `config.json`. |
| `email-read list` | Lists configured aliases + email + IMAP host. |
| `email-read remove <alias>` | Removes an account from config. |
| `email-read <alias>` | **Watch mode.** Connects via IMAP, polls every 2–3s for new mail, stores in SQLite + filesystem, evaluates rules, opens matching URLs in Chrome incognito. Runs until Ctrl+C. |
| `email-read` (no args) | Uses the **first** account in config as default alias for watch mode. |
| `email-read export-csv` | Exports the SQLite `Emails` table to `./data/export-<timestamp>.csv` relative to the current working directory. |
| `email-read rules list` | Shows configured rules and enabled state. |
| `email-read rules enable <name>` / `disable <name>` | Toggles a rule. |

## 5. Config File — `data/config.json`

```json
{
  "accounts": [
    {
      "alias": "work",
      "email": "me@company.com",
      "passwordB64": "cGFzc3dvcmQ=",
      "imapHost": "outlook.office365.com",
      "imapPort": 993,
      "useTLS": true,
      "mailbox": "INBOX"
    }
  ],
  "rules": [
    {
      "name": "open-magic-links",
      "enabled": true,
      "fromRegex": "noreply@.*",
      "subjectRegex": "(?i)sign.?in|magic link",
      "bodyRegex": ".*",
      "urlRegex": "https://app\\.example\\.com/auth\\?token=[A-Za-z0-9_-]+"
    }
  ],
  "watch": {
    "pollSeconds": 3
  },
  "browser": {
    "chromePath": "",
    "incognitoArg": ""
  }
}
```

### Browser config
- `chromePath` — optional absolute path to the Chrome executable. If empty, auto-detect (see §6).
- `incognitoArg` — optional override for the private-mode flag. If empty, auto-pick based on the detected browser:
  - Chrome / Chromium / Edge → `--incognito`
  - Brave → `--incognito`
  - Firefox (fallback only) → `-private-window`

### Password handling
- Stored as **Base64** in `passwordB64`. (Acknowledged: encoding, not encryption.)
- Decoded in-memory only when connecting to IMAP.

### IMAP auto-detection (built-in defaults by domain)
| Domain | Host | Port | TLS |
|---|---|---|---|
| gmail.com / googlemail.com | imap.gmail.com | 993 | yes |
| outlook.com / hotmail.com / live.com | outlook.office365.com | 993 | yes |
| yahoo.com | imap.mail.yahoo.com | 993 | yes |
| icloud.com / me.com | imap.mail.me.com | 993 | yes |
| fastmail.com | imap.fastmail.com | 993 | yes |
| _other_ | guess `mail.<domain>` :993 TLS, then `imap.<domain>` :993 TLS, prompt to override |

Cpanel-hosted domains (very common for custom domains) use `mail.<domain>`, so we now try that **first** before falling back to `imap.<domain>`.

User can override host/port/TLS during `add`.

### Seed/test account (from uploaded mobileconfig)
For dev/testing, the following account is pre-known and will be offered as the default in `email-read add` if the user just presses Enter:

| Field | Value |
|---|---|
| alias | `atto` |
| email | `lovable.admin@attobondcleaning.store` |
| imapHost | `mail.attobondcleaning.store` |
| imapPort | `993` |
| useTLS | `true` |
| mailbox | `INBOX` |
| password | (provided by user, stored Base64 in `passwordB64`) |

> ⚠️ Security note: this password is in the repo's spec file because the user supplied it explicitly for this prototype. For real deployment it must be rotated and only stored inside `data/config.json` (which is gitignored), never in source.

## 6. Rules Engine

Evaluated against every newly-fetched email **after** it is persisted.

For each enabled rule, ALL of the following must match (empty/missing pattern = match-any):
- `fromRegex` against the `From` header
- `subjectRegex` against the `Subject` header
- `bodyRegex` against the plain-text body (fall back to stripped HTML)

If matched, `urlRegex` is run against the body; **every** distinct match is opened via:

```
<chromePath> --incognito --new-window <url>
```

### Chrome auto-detection (cross-platform)

Resolution order in `internal/browser`:

1. **Config override** — if `config.browser.chromePath` is set and the file exists, use it verbatim.
2. **Env var** — `EMAIL_READ_CHROME` if set and exists.
3. **OS-specific defaults** (first existing path wins):

   **Windows** (`runtime.GOOS == "windows"`)
   - `%ProgramFiles%\Google\Chrome\Application\chrome.exe`
   - `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`
   - `%LocalAppData%\Google\Chrome\Application\chrome.exe`
   - `%ProgramFiles%\Chromium\Application\chrome.exe`
   - `%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe` (fallback)
   - `%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe` (fallback)

   **macOS** (`runtime.GOOS == "darwin"`)
   - `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
   - `$HOME/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
   - `/Applications/Chromium.app/Contents/MacOS/Chromium`
   - `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser` (fallback)
   - `/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge` (fallback)

   **Linux** (`runtime.GOOS == "linux"`) — try `exec.LookPath` for each name in order:
   - `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`, `brave-browser`, `microsoft-edge`
   - Also probe `/usr/bin/`, `/usr/local/bin/`, `/snap/bin/`, `/var/lib/flatpak/exports/bin/` for the same names.

4. **PATH lookup** — `exec.LookPath("chrome")` as a last resort.
5. If nothing is found, log a warning, persist the URL to `OpenedUrls` with `OpenedAt = NULL` (or a `Skipped` flag), and continue watching — never crash the watcher.

The detected path is **cached** for the process lifetime. The chosen incognito flag is selected from the executable's basename (chrome/chromium/msedge/brave → `--incognito`, firefox → `-private-window`), unless `config.browser.incognitoArg` overrides it.

Each opened URL is recorded in the `OpenedUrls` table so we never re-open the same URL twice.

## 7. SQLite Schema (`data/emails.db`)

All column names in **PascalCase** as requested.

### Table `Emails`
| Column | Type | Notes |
|---|---|---|
| Id | INTEGER PK AUTOINCREMENT | |
| Alias | TEXT | account alias |
| MessageId | TEXT UNIQUE | RFC822 Message-ID |
| Uid | INTEGER | IMAP UID |
| FromAddr | TEXT | |
| ToAddr | TEXT | comma-joined |
| CcAddr | TEXT | |
| Subject | TEXT | |
| BodyText | TEXT | plain text |
| BodyHtml | TEXT | raw HTML if present |
| ReceivedAt | DATETIME | |
| FilePath | TEXT | path to saved `.eml` |
| CreatedAt | DATETIME DEFAULT CURRENT_TIMESTAMP | |

### Table `WatchState`
Tracks last-seen position per alias for fast incremental polling.
| Column | Type | Notes |
|---|---|---|
| Alias | TEXT PK | |
| LastUid | INTEGER | highest UID seen |
| LastSubject | TEXT | |
| LastReceivedAt | DATETIME | |
| UpdatedAt | DATETIME | |

### Table `OpenedUrls`
Dedup ledger for opened links.
| Column | Type | Notes |
|---|---|---|
| Id | INTEGER PK AUTOINCREMENT | |
| EmailId | INTEGER FK Emails.Id | |
| RuleName | TEXT | |
| Url | TEXT | |
| OpenedAt | DATETIME DEFAULT CURRENT_TIMESTAMP | |

Unique index on `(EmailId, Url)`.

## 8. Watch Loop

1. Read `WatchState.LastUid` for the alias (0 if first run).
2. IMAP `SELECT INBOX` → search `UID <last+1>:*`.
3. For each new message:
   - Save raw `.eml` to `email/<alias>/<YYYY-MM-DD>/<safe-message-id>.eml`.
   - Insert row into `Emails`.
   - Run rules → open matched URLs in Chrome incognito → record in `OpenedUrls`.
4. Update `WatchState`.
5. Sleep `watch.pollSeconds` (default 3). Repeat.

Graceful shutdown on Ctrl+C (logout IMAP, close DB).

## 9. CSV Export

`email-read export-csv` writes `./data/export-<timestamp>.csv` relative to the **current working directory** (NOT the EXE folder), creating `./data/` if missing. Columns mirror the `Emails` table (PascalCase headers).

## 10. Tech Choices

- Go 1.22+
- IMAP: `github.com/emersion/go-imap` v1
- Message parsing: `github.com/emersion/go-message`
- SQLite: `modernc.org/sqlite` (pure Go — no CGO, easy Windows builds)
- CLI: `github.com/spf13/cobra`
- Interactive prompt: `github.com/AlecAivazis/survey/v2`

## 11. Repo Structure

```
/
├── run.ps1
├── go.mod
├── go.sum
├── cmd/
│   └── email-read/
│       └── main.go
├── internal/
│   ├── config/        # load/save config.json, password b64
│   ├── imapdef/       # IMAP server auto-detection map
│   ├── store/         # SQLite open + migrations + queries
│   ├── mailclient/    # IMAP connect, fetch, parse
│   ├── watcher/       # poll loop
│   ├── rules/         # regex rule evaluation
│   ├── browser/       # Chrome incognito launcher
│   └── exporter/      # CSV export
└── spec/21-golang-email-reader/
    ├── spec.md
    └── plan.md
```

## 12. Out of Scope (v1)
- Sending email.
- OAuth (Gmail/Outlook with 2FA require app passwords — documented in README).
- Browsers other than Chrome/Chromium/Edge/Brave (Firefox is a soft fallback only).
- Encryption beyond Base64 (explicitly declined by user).
- GUI.
