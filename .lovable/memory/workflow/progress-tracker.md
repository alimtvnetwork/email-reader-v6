---
name: Implementation progress tracker (spec/21-app)
description: Canonical denominator for the "% done" signal shown after every slice. Update on every slice.
type: feature
---
# Implementation Progress Tracker — spec/21-app

**Last updated:** 2026-04-28 (after **Slice #209 — Account Test timeout RCA/message fix**: `ER-ACC-22201` remains the Test Connection wrapper, but `core.wrapTestConnectionError` now branches on the wrapped `ER-MAIL-*` cause so timeout/dial failures say endpoint unreachable instead of falsely saying the server rejected the login. Added memory rule and regression coverage.)
**Overall: 100% done · 0% remaining (original roadmap complete) · UX bugfix/RCA slices applied through #209.**
