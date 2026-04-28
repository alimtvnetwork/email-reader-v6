# email-read

**Version:** 0.28.0

A small Windows-first Go CLI that watches IMAP inboxes, persists every email to SQLite + disk, and auto-opens URLs from matching emails in **Chrome incognito** based on regex rules.

- Multi-account support (Gmail, Outlook, Yahoo, iCloud, Fastmail, custom domains)
- Near-real-time polling (default every 3s)
- Raw `.eml` archive on disk + structured rows in SQLite
- Regex rule engine → auto-opens magic-link / verification URLs
- CSV export of the entire `Emails` table
- One-command bootstrap via `run.ps1`

---

## 1. Prerequisites (Windows)

Install these first (one-time setup):

| Tool | Why | Download |
|---|---|---|
| **Git for Windows** | `git pull` in `run.ps1` | <https://git-scm.com/download/win> |
| **Go 1.22+** | Builds the EXE | <https://go.dev/dl/> |
| **Google Chrome** (or Edge / Brave) | Opens matched URLs in incognito | <https://www.google.com/chrome/> |
| **PowerShell 5.1+** | Runs `run.ps1` | Built into Windows 10/11 |

Verify after installing:

```powershell
git --version
go version
```

### App passwords (Gmail / Outlook)

Gmail and Outlook both block plain IMAP login when 2FA is enabled. You must generate an **app password** and use that in `email-read add` instead of your real password.

- **Gmail** — <https://myaccount.google.com/apppasswords> (2FA must be on)
- **Outlook / Hotmail** — <https://account.microsoft.com/security> → *Advanced security options* → *App passwords*
- **Yahoo** — *Account Security* → *Generate app password*
- **Custom domains (cPanel/Plesk)** — use your normal mailbox password; host is usually `mail.<yourdomain>`

---

## 2. Install — one command

Clone the repo, then from a PowerShell window inside it:

```powershell
.\run.ps1
```

If PowerShell blocks the script, allow it for the current user once:

```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

`run.ps1` will:

1. `git pull` the latest code
2. `go build` → `email-reader-cli\email-read.exe`
3. Create `email-reader-cli\data\` and `email-reader-cli\email\`
4. Add `email-reader-cli\` to your **user `PATH`** (idempotent — safe to re-run)

Optional flags:

```powershell
.\run.ps1 -SkipPull          # don't run git pull
.\run.ps1 -SkipPathUpdate    # don't touch PATH
```

### ⚠️ Reopen your terminal after the first install

Windows only loads the new `PATH` into **new** processes. After the first `.\run.ps1`:

1. Close the current PowerShell window.
2. Open a fresh PowerShell (Start → "PowerShell").
3. Verify it works from any folder:

```powershell
email-read --version
```

> Re-runs of `run.ps1` (for updates) do **not** require reopening the terminal — only the very first install does.

---

## 3. Deployment Layout

After `run.ps1`, you'll have:

```
email-reader-cli\           <- on your PATH
├── email-read.exe
├── data\
│   ├── config.json         <- accounts + rules (gitignored)
│   └── emails.db           <- SQLite store
└── email\
    └── <alias>\
        └── <YYYY-MM-DD>\
            └── <MessageId>.eml
```

---

## 4. Command Reference

Run from **any folder** once `PATH` is set.

| Command | What it does |
|---|---|
| `email-read add` | Interactive prompt: email, password, alias. Auto-detects IMAP host from domain. |
| `email-read list` | Lists configured accounts. |
| `email-read remove <alias>` | Removes an account from `config.json`. |
| `email-read <alias>` | **Watch mode** — polls IMAP, saves new mail, runs rules, opens URLs. Ctrl+C to stop. |
| `email-read` (no args) | Watch mode using the **first** account in config. |
| `email-read watch <alias>` | Same as `email-read <alias>`, explicit form. |
| `email-read rules list` | Show all rules and enabled state. |
| `email-read rules enable <name>` | Enable a rule by name. |
| `email-read rules disable <name>` | Disable a rule by name. |
| `email-read export-csv` | Writes `.\data\export-<timestamp>.csv` in the **current** directory. |
| `email-read --version` | Print CLI version. |

### Typical first-time flow

```powershell
# 1. Add an account (interactive)
email-read add

# 2. Confirm it's saved
email-read list

# 3. Start watching (Ctrl+C to stop)
email-read work
```

---

## 5. Sample `config.json`

`run.ps1` does not create this file — `email-read add` does. For reference, here's what a populated `data\config.json` looks like:

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
      "subjectRegex": "(?i)sign.?in|magic link|verify",
      "bodyRegex": ".*",
      "urlRegex": "https://app\\.example\\.com/auth\\?token=[A-Za-z0-9_-]+"
    },
    {
      "name": "open-stripe-receipts",
      "enabled": false,
      "fromRegex": "receipts\\+.*@stripe\\.com",
      "subjectRegex": "(?i)receipt",
      "bodyRegex": ".*",
      "urlRegex": "https://pay\\.stripe\\.com/receipts/[A-Za-z0-9_/-]+"
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

### Rules cheat-sheet

- An empty/missing pattern means **match anything** for that field.
- All of `fromRegex`, `subjectRegex`, `bodyRegex` must match for a rule to fire.
- `urlRegex` is then run over the body — every distinct URL it captures is opened in incognito.
- Each opened URL is recorded in `OpenedUrls` so the same link is **never opened twice**.
- Edit `data\config.json` directly to add new rules, or toggle them with `email-read rules enable/disable`.

### Browser override

If Chrome lives in a non-standard location, set either:

- `config.browser.chromePath` to the absolute path, or
- the `EMAIL_READ_CHROME` environment variable

Auto-detection order: config → env var → standard install paths → `PATH`.

---

## 6. Updating

From the repo folder:

```powershell
.\run.ps1
```

That's it. No terminal reopen needed for updates.

---

## 7. Uninstall

1. Remove `email-reader-cli\` from your user `PATH` (System Properties → Environment Variables).
2. Delete the `email-reader-cli\` folder.
3. Optionally delete the cloned repo.

---

## 8. Troubleshooting

| Symptom | Fix |
|---|---|
| `email-read : The term ... is not recognized` | Reopen PowerShell — `PATH` only loads in new processes after first install. |
| `run.ps1 cannot be loaded because running scripts is disabled` | Run `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` once. |
| `go: command not found` | Install Go 1.22+ and reopen PowerShell. |
| IMAP `LOGIN failed` on Gmail/Outlook | You need an **app password**, not your real password. See §1. |
| URLs aren't opening | Check `email-read rules list`, confirm the rule is enabled and `urlRegex` matches. Check Chrome is installed or set `EMAIL_READ_CHROME`. |
| Watcher seems stuck | Increase verbosity by tailing `data\emails.db` (`OpenedUrls` table) — every fetched batch logs to stdout. |
