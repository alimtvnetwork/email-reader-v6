// Package specaudit holds slice-#119 spec-vs-code coverage guards.
//
// Production code does not import this package — it exists solely
// so its `*_test.go` files have a stable home outside any feature
// package. Mirrors the layout of `internal/ui/accessibility/`,
// which uses the same "package = test home" pattern for guards
// that span the whole tree.
//
// The guards live in their own package (rather than as `*_test.go`
// files in `internal/core/` or `internal/store/`) for two reasons:
//
//  1. They scan the entire repository — the spec tree under
//     `spec/21-app/` and every `*.go` file outside it. Pinning that
//     scan to a specific feature package would create a false
//     coupling and force every feature CI run to repeat the work.
//  2. The audit's allowlist (`coverageGapAllowlist` in
//     `coverage_audit_test.go`) must shrink monotonically across
//     unrelated slices. A neutral host package keeps the allowlist
//     out of any feature's review surface — anyone can shrink it
//     in a one-line diff without touching unrelated code.
//
// This file holds only the package doc + a single sentinel constant
// so `go test` has something to compile against. All real surface
// is in the test files alongside.
package specaudit

// SliceID names the slice that introduced this package, so a
// future archaeologist can `rg slice119Anchor` to find the audit
// surface without grepping through git history.
const SliceID = "slice-119-spec-coverage-audit"
