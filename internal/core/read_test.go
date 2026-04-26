package core

import (
	"context"
	"testing"
)

// We can't easily exercise the full ReadEmail pipeline without spinning up
// IMAP + browser, so these tests focus on the failure-fast paths that don't
// touch the network: missing account, missing email row.

func TestReadEmail_UnknownAccount(t *testing.T) {
	withIsolatedConfig(t, func() {
		r := ReadEmail(context.Background(), "ghost", 1, nil)
		if !r.HasError() {
			t.Fatal("expected error for unknown alias")
		}
	})
}

func TestReadEmail_EmitNilSafe(t *testing.T) {
	// Passing emit=nil must not panic — the function installs a no-op.
	withIsolatedConfig(t, func() {
		// No account configured → returns the same error as above.
		// The call should still complete (not panic) when emit is nil.
		_ = ReadEmail(context.Background(), "ghost", 1, nil)
	})
}
