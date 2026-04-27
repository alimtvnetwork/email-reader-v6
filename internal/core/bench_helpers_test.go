// bench_helpers_test.go — Slice #116 shared per-feature bench/perf
// helpers. **Restoration of the file from Slice #116** (it was lost
// in a workspace revert per `mem://workspace-revert-on-resume`).
//
// `withIsolatedConfigTB` widens `withIsolatedConfig`'s `*testing.T`
// argument to `testing.TB` so the same back-up/restore plumbing
// works for both `*testing.T` perf-gate tests and `*testing.B`
// benchmarks. Implementation is otherwise byte-identical to
// `withIsolatedConfig` in `accounts_test.go` — same on-disk back-up
// path, same Cleanup contract, same wipe-before-fn behaviour.
//
// Why a separate symbol (vs editing `withIsolatedConfig` in place)
// — touching the existing helper would force every existing caller
// to re-vet, and the migration risk isn't worth saving 25 LOC. The
// new symbol is bench-only and the existing one is unchanged.
package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// withIsolatedConfigTB backs up data/config.json (if any), runs fn,
// and restores it. Same contract as withIsolatedConfig but accepts
// `testing.TB` so it can be called from both `*testing.T` and
// `*testing.B`.
func withIsolatedConfigTB(tb testing.TB, fn func()) {
	tb.Helper()
	p, err := config.Path()
	if err != nil {
		tb.Fatalf("config path: %v", err)
	}
	backup := p + ".testbackup"
	if data, err := os.ReadFile(p); err == nil {
		if werr := os.WriteFile(backup, data, 0o600); werr != nil {
			tb.Fatalf("backup config: %v", werr)
		}
	}
	tb.Cleanup(func() {
		if data, err := os.ReadFile(backup); err == nil {
			_ = os.WriteFile(p, data, 0o600)
			_ = os.Remove(backup)
		} else {
			_ = os.Remove(p)
		}
	})
	_ = os.Remove(p)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	fn()
}
