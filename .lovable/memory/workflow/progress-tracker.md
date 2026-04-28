---
name: Implementation progress tracker (spec/21-app)
description: Canonical denominator for the "% done" signal shown after every slice. Update on every slice.
type: feature
---
# Implementation Progress Tracker — spec/21-app

**Last updated:** 2026-04-28 (after **Slice #204 — IMAP STARTTLS fallback for networks timing out on 993**: Verified the admin mailbox logs in successfully from the sandbox on `mail.attobondcleaning.store:993`; added fallback so watcher/Test Connection retry `mail.attobondcleaning.store:143` with STARTTLS when implicit-TLS port 993 times out. Verified focused `go test -tags nofyne ./internal/mailclient ./internal/watcher`.)
**Overall: 100% done · 0% remaining (original roadmap complete) · UX bugfix slices applied through #204.**
