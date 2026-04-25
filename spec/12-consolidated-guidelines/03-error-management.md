# Consolidated: Error Management — Complete Reference

**Version:** 3.2.0  
**Updated:** 2026-04-16  
**Source Module:** [`spec/03-error-manage/`](../03-error-manage/00-overview.md)

---

## Purpose

This is the **standalone consolidated reference** for all error management. An AI reading only this file must be able to enforce every rule without consulting source specs.

> 🔴 **#1 PRIORITY** — Error management is the highest priority specification. It must be implemented from the very first line of code.

---

## Three-Tier Error Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Frontend (React/TypeScript)                   │
│  ┌─────────────────┐    ┌──────────────────┐    ┌───────────────┐   │
│  │ API Client       │───▸│ Error Store       │───▸│ Global Error  │   │
│  │ (parseEnvelope)  │    │ (captureError)    │    │ Modal (tabs)  │   │
│  └─────────────────┘    └──────────────────┘    └───────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                              ▲ Universal Response Envelope
┌─────────────────────────────────────────────────────────────────────┐
│                        Backend (Go)                                  │
│  ┌─────────────────┐    ┌──────────────────┐    ┌───────────────┐   │
│  │ apperror.Wrap() │───▸│ Session Logger    │───▸│ error.log.txt │   │
│  │ + .WithContext() │    │ (per-request ID)  │    │ (deduped)     │   │
│  └─────────────────┘    └──────────────────┘    └───────────────┘   │
│         │ DelegatedRequestServer Builder                             │
│         │ • Captures: endpoint, method, statusCode                   │
│         │ • Captures: requestBody, response, stackTrace              │
│         │ • Injects into Envelope.Errors block                       │
└─────────────────────────────────────────────────────────────────────┘
                              ▲ REST API (JSON)
┌─────────────────────────────────────────────────────────────────────┐
│              Delegated Server (PHP / Chrome Extension / Other)       │
│  ┌─────────────────┐    ┌──────────────────┐    ┌───────────────┐   │
│  │ safe_execute()  │───▸│ FileLogger        │───▸│ stacktrace.txt│   │
│  │ catch Throwable │    │ (6-frame backtrace)│   │ error.txt     │   │
│  └─────────────────┘    └──────────────────┘    └───────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### Request Chain (3-Hop)

```
React Frontend → Go Backend → Delegated Server (PHP/other)
     │                │                │
     │ GET /api/v1/   │ GET /wp-json/  │
     │ sites/1/data   │ plugin/v1/data │
     │───────────────▸│───────────────▸│
     │                │     HTTP 403   │
     │                │◀──────────────│
     │   HTTP 500     │               │
     │   Envelope     │               │
     │◀──────────────│               │
     │ captureError() │               │
```

| Tier | Layer | Responsibility | Technology |
|------|-------|----------------|------------|
| 1 | Delegated Server (PHP) | Structured error responses with error codes, `TypedQuery` result envelopes | PHP `ResponseKeyType` enum, `DbResult<T>`, `DbResultSet<T>`, `DbExecResult` |
| 2 | Go Backend | Error wrapping, stack traces, typed error codes, structured logging | `apperror` package with `Result[T]`, `Wrap()`, `WithContext()` |
| 3 | Frontend | Error store, Global Error Modal, toast notifications, retry logic | React error boundary, Zustand error store, Sonner toasts |

---

## Universal Response Envelope

**All APIs MUST return this envelope.** No exceptions.

### Success Response

```json
{
  "Status": {
    "IsSuccess": true,
    "Code": 200,
    "Message": "OK"
  },
  "Attributes": {
    "RequestedAt": "/plugin/v1/status",
    "Duration": "45ms",
    "TotalRecords": 150,
    "Limit": 50,
    "Offset": 0
  },
  "Results": [{ "..." }]
}
```

### Error Response

```json
{
  "Status": {
    "IsSuccess": false,
    "Code": 500,
    "Message": "Database connection failed"
  },
  "Error": {
    "ErrorCode": 1001,
    "ErrorType": "DATABASE_ERROR",
    "Detail": "Connection timeout after 5s",
    "StackTrace": "..."
  },
  "Errors": {
    "Backend": ["apperror stack frame 1", "..."],
    "BackendMessage": "open database: file locked",
    "DelegatedRequestServer": {
      "Endpoint": "https://example.com/wp-json/plugin/v1/data",
      "Method": "GET",
      "StatusCode": 403,
      "Response": "{ ... raw JSON ... }",
      "StackTrace": "PHP stack trace...",
      "RequestBody": null
    }
  },
  "MethodsStack": ["Handler.ServeHTTP", "Router.Route", "Middleware.Auth"],
  "SessionId": "sess_abc123",
  "Attributes": { "RequestedAt": "/api/v1/sites/1/data" },
  "Results": []
}
```

### Envelope Key Rules

- **HTTP status is the primary indicator** — frontend checks 2xx, not `IsSuccess`
- **`Results` is always an array** — even for single items
- **`Attributes` always includes `RequestedAt`** — the route path
- All response keys use **PascalCase** via `ResponseKeyType` enum
- **`Errors.DelegatedRequestServer`** — present when Go proxied to a downstream server that failed
- **`MethodsStack`** — Go call chain that handled the request

---

## Go `apperror` Package

### Single Return Value — `Result[T]`

Go functions **never** return `(T, error)`. They return `apperror.Result[T]` which is a single value containing either the result or the error.

```go
// ❌ FORBIDDEN — dual return
func GetUser(id string) (*User, error) { ... }

// ✅ REQUIRED — single Result[T]
func GetUser(id string) apperror.Result[*User] { ... }
```

### Result[T] API

| Method | Purpose |
|--------|---------|
| `result.HasError()` | Check if operation failed |
| `result.Value()` | Get the success value (only after HasError check) |
| `result.Error()` | Get the error (only when HasError is true) |
| `result.PropagateError()` | Re-wrap and return error to caller |

### Error Wrapping — Always `apperror.Wrap()`

```go
// ❌ FORBIDDEN — raw fmt.Errorf
return fmt.Errorf("failed to open: %w", err)

// ✅ REQUIRED — structured wrapping with code
return apperror.Wrap(
    err,
    ErrDbOpen,
    "open database",
).WithContext("path", dbPath)
```

### Error Context Methods

| Method | Purpose | Example |
|--------|---------|---------|
| `.WithContext(key, value)` | Add key-value context | `.WithContext("path", "/db/main.db")` |
| `.WithPath(path)` | Shorthand for file path context | `.WithPath(configPath)` |
| `.WithStatusCode(code)` | Attach HTTP status | `.WithStatusCode(404)` |

### Error Propagation Pattern

```go
result := doSomething()
if result.HasError() {
    return result.PropagateError()
}
value := result.Value()
```

### 🔴 CODE RED — Swallowed Errors

Swallowing errors is the **highest severity violation**.

```go
// ❌ CODE RED — swallowed error (underscore discard)
result, _ := doSomething()

// ❌ CODE RED — empty catch
} catch (e) {}

// ❌ CODE RED — generic message without path
return errors.New("file not found")  // WHICH file?

// ✅ REQUIRED — always include context
return apperror.Wrap(err, ErrFileNotFound, "read config").WithPath(configPath)
```

---

## Tier 1: Delegated Server Error Handling (PHP)

### Safe Execution Pattern

Every REST endpoint handler is wrapped in `safeExecute`:

```php
public function handleRequest(WP_REST_Request $request): WP_REST_Response {
    return $this->safeExecute(
        fn() => $this->executeRequest($request),
        'request-name',
    );
}
```

`safeExecute` catches `Throwable`, logs via `FileLogger`, and returns a structured error envelope. Debug mode controls whether stack traces appear in the response.

### FileLogger — Two-Tier Logging

| Tier | Class | When Available | Purpose |
|------|-------|---------------|---------|
| 1 | `error_log()` / `ErrorLogHelper` | Always | Fallback before autoloader loads |
| 2 | `FileLogger` (singleton) | After autoloader | Primary structured log with rotation, dedup, stack traces |

**Log files** (under `wp-content/uploads/{plugin-slug}/logs/`):

| File | Contents |
|------|----------|
| `info.log` | All successful operations + debug entries (when debug mode ON) |
| `error.log` | All errors + exceptions with stack traces |
| `stacktrace.log` | Dedicated stack trace file (6-frame limit) |

### TypedQuery — Database Result Envelopes

| Class | Purpose | Key Methods |
|-------|---------|-------------|
| `DbResult<T>` | Single-row query | `isDefined()`, `isEmpty()`, `hasError()`, `isSafe()`, `value()`, `error()`, `stackTrace()` |
| `DbResultSet<T>` | Multi-row query | `hasAny()`, `isEmpty()`, `count()`, `hasError()`, `isSafe()`, `items()`, `first()` |
| `DbExecResult` | INSERT/UPDATE/DELETE | `isEmpty()`, `hasError()`, `isSafe()`, `affectedRows()`, `lastInsertId()` |

```php
$tq = new TypedQuery($pdo);

// Single row
$result = $tq->queryOne(
    'SELECT * FROM plugins WHERE id = :id',
    [':id' => $id],
    fn(array $row): PluginInfo => PluginInfo::fromRow($row),
);

if ($result->hasError()) { /* handle */ }
if ($result->isEmpty()) { /* not found */ }
$plugin = $result->value();
```

---

## DelegatedRequestServer — Proxy Error Capture

When the Go backend proxies to a downstream server (PHP, Node.js, etc.) and the downstream server fails, the Go backend captures full diagnostics:

| Field | Type | Description |
|-------|------|-------------|
| `Endpoint` | string | Full URL of the downstream request |
| `Method` | string | HTTP method used (GET, POST, etc.) |
| `StatusCode` | int | HTTP status from downstream |
| `Response` | string | Raw response body from downstream |
| `StackTrace` | string | Downstream stack trace (if provided) |
| `RequestBody` | string/null | Request body sent (POST/PUT/PATCH only) |

This block appears in `Envelope.Errors.DelegatedRequestServer` and is essential for debugging proxy-chain errors.

---

## Error Code Registry

### Structure

- Master registry: `error-codes-master.json`
- Ranges assigned per module to prevent collisions
- Overlap validator script enforces no duplicate ranges

### Module Ranges

| Range | Prefix | Module |
|-------|--------|--------|
| 1000–1999 | `GEN` | General/Shared (Core, Database, Auth, File System) |
| 2000–2999 | `SM` | Spec Management Software |
| 7000–7099 | `GS` | GSearch CLI Core |
| 7100–7599 | `BR` | BRun CLI |
| 7600–7919 | `GS` | GSearch sub-modules (Movie, BI, Multi-Source, etc.) |
| 8000–8399 | `NF` | Nexus Flow |
| 9000–9499 | `AB` | AI Bridge Core |
| 9500–9599 | `PS` | PowerShell Integration |
| 9600–9999 | `AB` | AI Bridge Extended modules |
| 10000–10499 | `WPB` | WP Plugin Builder |
| 11000–11999 | `SRC` | Spec Reverse CLI |
| 12000–12599 | `WSP` | WP SEO Publish |
| 13000–13999 | `WPP` | WP Plugin Publish |
| 14000–14499 | `AIT` | AI Transcribe CLI |
| 14500–14999 | `EQM` | Exam Manager |
| 15000–15999 | `LM` | Link Manager |
| 16000–16799 | `SM-CG` | SM Code Generation |
| 17000–17999 | `SM-PE` | SM Project Editor |
| 19000–19019 | `AB` | AI Bridge Lovable Reasoning |
| 20000–20999 | `AB-TR` | AI Bridge Non-Vector RAG |

### Code Registration Format

```json
{
  "Code": 1001,
  "Name": "ErrDbConnectionTimeout",
  "Module": "core/database",
  "Severity": "error",
  "Message": "Database connection timed out",
  "Resolution": "Check database path exists and is not locked"
}
```

### Overlap Prevention Rules

- Each module owns a contiguous integer range — no gaps, no overlaps
- Collision validator script runs in CI
- When ranges collide, the newer module gets reassigned (13 resolutions documented)
- PS/AB SEO share 9500-9540 intentionally — distinguishable by format (prefixed vs flat integer)

---

## Frontend Error Handling

### Error Store (Zustand)

```typescript
interface ErrorState {
  errors: AppError[];
  addError: (error: AppError) => void;
  clearErrors: () => void;
  dismissError: (id: string) => void;
}
```

### Global Error Modal

- Displays when `errors.length > 0`
- **Tabbed interface:** Overview, Backend Details, Delegated Server, Session Info
- Shows error code, message, and suggested resolution
- **Copy button (split button):** Main click → Compact Report (instant), dropdown → Full Report / With Backend Logs / error.log.txt / log.txt
- **Download menu:** Full Bundle (ZIP), error.log.txt, log.txt, Report (.md)
- Dismiss button clears the error
- Auto-dismiss for non-critical errors after 5 seconds

### Compact Report (Default Copy Format)

The default copy output includes:
- Error code, message, HTTP status
- Request URL, method, timestamp
- **Delegated Server Info** (built from `CapturedError.envelopeErrors.DelegatedRequestServer`) — no API call needed
- Frontend execution chain

### Toast Notifications

| Type | Color | Auto-dismiss |
|------|-------|-------------|
| Success | Green | 3s |
| Warning | Yellow | 5s |
| Error | Red | Persists until dismissed |
| Info | Blue | 3s |

---

## Notification Colors

| Notification Type | Color | Hex |
|-------------------|-------|-----|
| Success | Green | `#22c55e` |
| Warning | Amber | `#f59e0b` |
| Error | Red | `#ef4444` |
| Info | Blue | `#3b82f6` |

---

## Debugging Cheat Sheet

### Initialization Order (ALL Languages)

```
1. Configuration    → Load env vars and config files FIRST
2. Directories      → Ensure all required directories exist
3. Database         → Initialize connections (only after dirs exist)
4. Services         → Initialize business logic components
5. Server/App       → Start ONLY after all dependencies ready
```

### Common Pitfalls

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| "Backend disconnected" but backend is running | Response format mismatch | Compare handler output to frontend detection logic |
| 404 on API base URL | No index route registered | Check router for `GET /api/v1` handler |
| VITE_API_URL shows wrong value | Resolved vs raw env confusion | Distinguish raw env var from resolved origin |
| HTML instead of JSON | SPA fallback serving `index.html` | Check if route exists in backend router |
| CORS errors | Missing CORS headers | Check backend CORS middleware configuration |
| 401/403 on protected routes | Token not sent or expired | Check Authorization header, token validity |
| Plugin won't activate | PDO SQLite extension missing | Check `extension_loaded('pdo_sqlite')` |
| Database connection fails | Directory permissions | Check path exists and is writable |

### Quick Debug Commands

```bash
# Go — check server
curl -s http://localhost:8080/api/v1/health | jq .
tail -f logs/app.log

# PHP — check logs
cat wp-content/uploads/{plugin-slug}/logs/error.log

# TypeScript — check API
console.log(import.meta.env.VITE_API_URL)
```

---

## Core Principles

1. **Never assume — always verify** both backend response and frontend detection
2. **HTTP status is primary indicator** — use 2xx, not response body fields
3. **Structured errors only** — no unstructured error strings
4. **Every error gets context** — path, entity ID, operation name
5. **Stack traces in development** — stripped in production responses
6. **Envelope is universal** — every API endpoint returns the same structure
7. **Delegated server errors propagate** — Go captures and forwards downstream diagnostics

---

## Forbidden Patterns

| Pattern | Why | Required Alternative |
|---------|-----|---------------------|
| `catch (Exception $e)` | Misses PHP 7+ `Error` types | `catch (Throwable $e)` |
| `error_log()` for diagnostics | No structure | `FileLogger` / `Logger` |
| Empty `catch` block | Swallowed error — CODE RED | Handle or re-throw |
| `fmt.Errorf("failed: %w", err)` | No error code | `apperror.Wrap(err, ErrCode, "context")` |
| `result, _ := fn()` | Swallowed error — CODE RED | Check `result.HasError()` |
| Generic "file not found" | No path context — CODE RED | Include exact file path |
| `(T, error)` dual return in Go | Breaks single-return pattern | `apperror.Result[T]` |
| Inline error strings | Not machine-parseable | Error code constants |

---

## Validation

Run `linter-scripts/validate-guidelines.py` — zero **CODE-RED** violations required.

---

## Cross-References

| Topic | Source Location |
|-------|----------------|
| Error Handling Reference | `03-error-manage/02-error-architecture/01-error-handling-reference.md` |
| Error Code Registry | `03-error-manage/03-error-code-registry/01-registry.md` |
| Debugging Cheat Sheet | `03-error-manage/01-error-resolution/02-debugging-cheat-sheet.md` |
| Error Modal Copy Formats | `03-error-manage/02-error-architecture/04-error-modal/01-copy-formats/` |
| Notification Colors | `03-error-manage/02-error-architecture/03-notification-colors.md` |
| Go Delegation Fix | `03-error-manage/02-error-architecture/02-go-delegation-fix.md` |
| Collision Resolution | `03-error-manage/03-error-code-registry/03-collision-resolution-summary.md` |
| Overlap Validator | `03-error-manage/03-error-code-registry/05-overlap-validator.md` |
| Session-Based Logging | `03-error-manage/07-logging-and-diagnostics/02-session-based-logging.md` |

---

*Consolidated error management — v3.2.0 — 2026-04-16*
