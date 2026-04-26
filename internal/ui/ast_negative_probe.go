//go:build verify_ast_guard_negative

package ui

import "github.com/lovable/email-read/internal/core"

// Synthetic violation: calls a deleted core symbol. Build-tagged so
// it never compiles in real builds, but go/parser ignores build tags
// so the AST guard SHOULD see and flag this call.
var _ = core.ListEmails
