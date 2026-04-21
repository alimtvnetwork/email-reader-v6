# email-read Testing Guide

## Prerequisites
- macOS with Go 1.22+ installed
- Chrome browser installed
- Git access to the repo

---

## Test 1: Build & Basic CLI

### Step 1.1: Run the build script
**Action:** Execute the PowerShell build script
```bash
cd /Users/ab_mahin/Downloads/email-reader-v1
./run.ps1
```

**Expected Result:**
- Script downloads Go dependencies
- Builds `email-read` binary successfully
- Creates `email-reader-cli/` folder with binary
- No build errors

**What success looks like:**
```
==> Building email-read
==> Resolving Go module dependencies (go mod tidy)
...
==> Build complete: email-reader-cli/email-read
```

---

## Test 2: Account Management

### Step 2.1: Add an email account
**Action:** Run add command
```bash
./email-reader-cli/email-read add
```

**Expected interaction:**
1. Enter email: `loveable.engineer.v005@attobondcleaning.store`
2. Enter alias: `alex`
3. Enter password: [your password]
4. Press Enter to accept default IMAP host: `mail.attobondcleaning.store`
5. Press Enter for port: `993`
6. Press Enter for TLS: `Yes`
7. Press Enter for mailbox: `INBOX`

**What success looks like:**
```
Saved account "alex" to /Users/ab_mahin/Downloads/email-reader-v1/email-reader-cli/data/config.json
```

**Verify:** Check the config file exists
```bash
cat email-reader-cli/data/config.json
```
You should see your account with Base64-encoded password.

---

### Step 2.2: List accounts
**Action:** Run list command
```bash
./email-reader-cli/email-read list
```

**Expected Result:**
```
ALIAS  EMAIL                                    IMAP HOST                    PORT  TLS  MAILBOX
alex   loveable.engineer.v005@attobondcleaning.store  mail.attobondcleaning.store  993   true INBOX
```

---

### Step 2.3: Version check
**Action:** Check version
```bash
./email-reader-cli/email-read --version
```

**Expected Result:**
```
email-read v0.8.0
```

---

## Test 3: Watch Mode (Core Feature)

### Step 3.1: Start watching
**Action:** Start watch mode with alias
```bash
./email-reader-cli/email-read alex
```

**Expected Result:**
```
No alias given — using first configured account "alex"
2025/01/XX XX:XX:XX press Ctrl+C to stop
2025/01/XX XX:XX:XX [alex] starting watcher (poll=3s, host=mail.attobondcleaning.store)
2025/01/XX XX:XX:XX [alex] baseline set to UID=0 (skipping history)
```

**What this means:**
- ✅ CLI is running
- ✅ Connected to IMAP server
- ✅ Baseline UID set (first run behavior - skips old emails)

---

### Step 3.2: Send a test email
**Action:** Send an email to `loveable.engineer.v005@attobondcleaning.store`

**Expected Result in terminal:**
```
2025/01/XX XX:XX:XX [alex] fetched 1 new message(s)
2025/01/XX XX:XX:XX [alex] saved uid=123 subj="Your test email subject"
```

---

### Step 3.3: Verify email storage
**Action:** Check the email was saved

**SQLite database:**
```bash
sqlite3 email-reader-cli/data/emails.db "SELECT Id, Alias, Subject, FromAddr FROM Emails;"
```

**Expected Result:**
```
1|alex|Your test email subject|your-email@example.com
```

**File system:**
```bash
ls -la email-reader-cli/email/alex/$(date +%Y-%m-%d)/
```

**Expected Result:**
```
-rw-r--r--  1 user  staff  2048 Jan XX XX:XX 123.eml
```

---

## Test 4: Rules & Auto-Open URL

### Step 4.1: Configure a rule
**Action:** Edit `email-reader-cli/data/config.json` and add a rule:

```json
{
  "rules": [
    {
      "name": "test-rule",
      "enabled": true,
      "fromRegex": "",
      "subjectRegex": "test",
      "bodyRegex": "",
      "urlRegex": "https?://.*"
    }
  ]
}
```

This rule will:
- Match emails with "test" in the subject
- Extract any URL from the body

---

### Step 4.2: Test URL auto-opening
**Action:** Send an email to your inbox with:
- Subject: `Test email with link`
- Body containing: `Click here: https://www.google.com`

**Expected Result in terminal:**
```
2025/01/XX XX:XX:XX [alex] fetched 1 new message(s)
2025/01/XX XX:XX:XX [alex] saved uid=124 subj="Test email with link"
2025/01/XX XX:XX:XX [alex] opened https://www.google.com (rule=test-rule)
```

**Expected browser behavior:**
- Chrome opens in incognito mode
- Navigates to `https://www.google.com`

---

### Step 4.3: Verify URL deduplication
**Action:** Wait 3 seconds (next poll cycle)

**Expected Result:**
- The same URL does NOT open again
- Terminal shows no new "opened" message for the same email

**What this proves:**
- ✅ `OpenedUrls` table is working
- ✅ Same URL from same email won't reopen

---

## Test 5: Rules Management

### Step 5.1: List rules
**Action:** Run rules list command
```bash
./email-reader-cli/email-read rules list
```

**Expected Result:**
```
NAME        ENABLED  FROM                SUBJECT             BODY                URL
--          -----    ----                -------             ----                ---
test-rule   true     ""                  "test"              ""                  "https?://.*"
```

---

### Step 5.2: Disable a rule
**Action:** Disable the test rule
```bash
./email-reader-cli/email-read rules disable test-rule
```

**Expected Result:**
```
Rule "test-rule" disabled.
```

---

### Step 5.3: Enable a rule
**Action:** Re-enable the rule
```bash
./email-reader-cli/email-read rules enable test-rule
```

**Expected Result:**
```
Rule "test-rule" enabled.
```

---

## Test 6: CSV Export

### Step 6.1: Export emails to CSV
**Action:** Run export command
```bash
./email-reader-cli/email-read export-csv
```

**Expected Result:**
```
Exported 2 emails to /Users/ab_mahin/Downloads/email-reader-v1/email-reader-cli/data/export-20250121143000.csv
```

---

### Step 6.2: Verify CSV content
**Action:** Check the exported file
```bash
cat email-reader-cli/data/export-*.csv
```

**Expected Result:**
```csv
Id,Alias,MessageId,Uid,FromAddr,ToAddr,CcAddr,Subject,BodyText,BodyHtml,ReceivedAt,FilePath
1,alex,<message-id@example.com>,123,sender@example.com,loveable.engineer.v005@attobondcleaning.store,,Your test email subject,,,<timestamp>,email/alex/2025-01-21/123.eml
```

---

## Test 7: Clean Shutdown

### Step 7.1: Stop the watcher
**Action:** Press `Ctrl+C` in the terminal running watch mode

**Expected Result:**
```
2025/01/XX XX:XX:XX [alex] watcher stopped: context canceled
```

**What this proves:**
- ✅ Graceful shutdown works
- ✅ No data corruption

---

## Test 8: Account Removal (Optional)

### Step 8.1: Remove account
**Action:** Remove the test account
```bash
./email-reader-cli/email-read remove alex
```

**Expected Result:**
```
Removed account "alex"
```

---

## Quick Reference: Expected File Structure

After successful testing, your directory should look like:

```
email-reader-v1/
├── email-reader-cli/
│   ├── email-read              # CLI binary
│   ├── data/
│   │   ├── config.json         # Account + rules config
│   │   ├── emails.db           # SQLite database
│   │   └── export-*.csv        # CSV exports
│   └── email/
│       └── alex/
│           └── 2025-01-21/
│               ├── 123.eml     # Raw email files
│               └── 124.eml
└── ...
```

---

## Troubleshooting Checklist

| Issue | Solution |
|-------|----------|
| `email-read: command not found` | Use full path: `./email-reader-cli/email-read` |
| `no accounts configured` | Run `./email-reader-cli/email-read add` first |
| `connection refused` | Check IMAP host/port/TLS settings |
| `authentication failed` | Verify password is correct |
| `Chrome not found` | Set env: `export EMAIL_READ_CHROME=/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome` |
| Rules not triggering | Check regex pattern, ensure rule is `enabled: true` |
| URLs not opening | Check Chrome path, verify incognito works manually |

---

## Test Summary Checklist

- [ ] Build succeeds without errors
- [ ] `add` command creates account interactively
- [ ] `list` shows configured accounts
- [ ] `--version` shows version number
- [ ] Watch mode starts and connects to IMAP
- [ ] New emails are fetched and logged
- [ ] Emails saved to SQLite (verify with query)
- [ ] Emails saved as .eml files (verify with ls)
- [ ] Rules match emails correctly
- [ ] URLs open in Chrome incognito
- [ ] Same URL doesn't reopen (deduplication)
- [ ] `rules list` shows all rules
- [ ] `rules disable` / `rules enable` work
- [ ] `export-csv` creates CSV file
- [ ] CSV contains correct email data
- [ ] Ctrl+C shuts down gracefully

---

**Last Updated:** 2026-04-21 (UTC+8)
