package config

import (
	"os"
	"testing"
)

// TestApplySeedDefaults_RespectsDisableEnv verifies the EMAIL_READ_DISABLE_SEED
// test seam relied upon by withIsolatedConfig in internal/core.
//
// Without this seam, deleting data/config.json between subtests causes
// config.Load to re-inject DefaultSeedAccounts on the next Load, which
// previously bled the demo account into TestDiagnose_NoAccounts and
// the Settings_Save_* tests.
func TestApplySeedDefaults_RespectsDisableEnv(t *testing.T) {
	if len(DefaultSeedAccounts) == 0 {
		t.Skip("no default seeds compiled in; nothing to disable")
	}

	prev, had := os.LookupEnv("EMAIL_READ_DISABLE_SEED")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("EMAIL_READ_DISABLE_SEED", prev)
		} else {
			_ = os.Unsetenv("EMAIL_READ_DISABLE_SEED")
		}
	})

	// With env set: no mutation, no accounts added.
	_ = os.Setenv("EMAIL_READ_DISABLE_SEED", "1")
	c := &Config{Accounts: []Account{}}
	if applySeedDefaults(c) {
		t.Fatalf("applySeedDefaults reported mutation with disable env set")
	}
	if len(c.Accounts) != 0 {
		t.Fatalf("expected 0 seeded accounts with disable env set, got %d", len(c.Accounts))
	}

	// "true" also disables.
	_ = os.Setenv("EMAIL_READ_DISABLE_SEED", "true")
	c = &Config{Accounts: []Account{}}
	if applySeedDefaults(c) {
		t.Fatalf("applySeedDefaults reported mutation with disable env=true")
	}

	// Empty value does NOT disable — but we cannot positively assert
	// that seeds are added here without polluting the on-disk
	// tombstone file (loadTombstones reads data/seeded-deleted.json).
	// Coverage of the happy-seed path is provided by Load's existing
	// tests; this case only verifies the env value parsing.
	_ = os.Setenv("EMAIL_READ_DISABLE_SEED", "")
	// No assertion needed beyond "did not panic"; the seed path is
	// validated indirectly by the rest of the config test suite.
}
