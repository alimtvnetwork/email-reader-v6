# Feature 06 — Tools

**Version:** 1.0.0
**Updated:** 2026-04-25
**Surface:** Fyne UI (CLI has equivalent commands)

---

## Purpose

Surface the one-shot CLI commands (`read`, `export-csv`, `diagnose`) as inline forms in the UI so non-CLI users can run them without a terminal.

## Sub-tools

### 6a. Read (one-shot fetch)
- Fields: alias (dropdown), limit (int, default 10).
- Submit → calls `core.ReadOnce(alias, limit)` → streams results into a log panel below the form.

### 6b. Export CSV
- Fields: alias (dropdown), date range (from/to), output path (file-save dialog).
- Submit → `core.ExportCSV(alias, from, to, path)`.
- Show row count + final path on success.

### 6c. Diagnose
- Fields: alias (dropdown).
- Submit → `core.Diagnose(alias)` → renders a checklist:
  - TCP connect ✓
  - TLS handshake ✓
  - IMAP LOGIN ✓
  - INBOX SELECT ✓ (messages=N, uidNext=M)
  - MX lookup ✓
- Each step row turns red with the error message on failure.

## Layout

```
┌─ Tools ──────────────────────────────────────────┐
│ [Read] [Export CSV] [Diagnose]      ← sub-nav    │
│                                                   │
│ ┌─ Read ────────────────────────────┐            │
│ │ Alias: [work ▾]   Limit: [10]     │            │
│ │ [Run]                             │            │
│ └───────────────────────────────────┘            │
│                                                   │
│ ┌─ Output ──────────────────────────┐            │
│ │ ...streaming log lines...         │            │
│ └───────────────────────────────────┘            │
└───────────────────────────────────────────────────┘
```

## Backend (core API)

All three already exist (`internal/core/read.go`, `export.go`, `diagnose.go`). Required: each MUST accept a `chan<- string` for streaming output so the UI can render incrementally.

## Acceptance criteria

| # | Criterion |
|---|-----------|
| AC-T1 | Each tool shows its output as it streams (not only on completion). |
| AC-T2 | Export CSV file picker defaults to `./data/export-<timestamp>.csv` per the CLI behavior. |
| AC-T3 | Diagnose checklist renders all 5 steps in order; first failure halts further steps. |
| AC-T4 | Running a tool does not block the UI — the tool button changes to "Cancel" and re-enables on completion. |
| AC-T5 | A successful Diagnose result is cached for 60s per alias to avoid repeated IMAP login storms. |
