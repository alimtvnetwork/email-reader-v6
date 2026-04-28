// Package config — default account seeding.
//
// Some accounts are pre-provisioned for the user (e.g. the demo / testing
// account they shipped with the build). They should appear automatically
// the first time the app runs, but once the user deletes one, it must
// stay deleted across restarts. We persist a tombstone file
// `data/seeded-deleted.json` with the aliases the user has removed; the
// seeding step skips any alias listed there.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lovable/email-read/internal/errtrace"
)

// SeedPasswordEnvVar is the environment variable consulted at seed time
// for the default account's IMAP password. We deliberately do NOT ship a
// plaintext password in the binary — operators must supply one via env
// for the seed to be applied. If unset/empty, the seed entry is skipped
// (the app simply starts with no pre-provisioned account).
const SeedPasswordEnvVar = "EMAIL_READ_SEED_PASSWORD"

// DefaultSeedAccounts is the list of accounts pre-provisioned for the
// user. Passwords are sourced at runtime from SeedPasswordEnvVar — see
// resolveSeedPassword. Entries with an empty resolved password are
// skipped by applySeedDefaults.
//
// Add new entries here to ship them with the build. Remove entries here
// only when you also want existing installs to stop seeing them on a
// fresh install.
var DefaultSeedAccounts = []Account{
	{
		Alias:       "admin",
		Email:       "lovable.admin@attobondcleaning.store",
		DisplayName: "Admin (default)",
		// PasswordB64 intentionally empty — populated at seed time from
		// SeedPasswordEnvVar via resolveSeedPassword.
		PasswordB64: "",
		ImapHost:    "mail.attobondcleaning.store",
		ImapPort:    993,
		UseTLS:      true,
		Mailbox:     "INBOX",
	},
}

// resolveSeedPassword returns the EncodePassword'd value of the env-var
// password, or "" if the env var is unset/empty. Sanitization (U+2060
// wrappers etc.) is handled inside EncodePassword.
func resolveSeedPassword() string {
	raw := os.Getenv(SeedPasswordEnvVar)
	if raw == "" {
		return ""
	}
	return EncodePassword(raw)
}

// tombstonePath returns the absolute path of the seeded-deleted tombstone
// file. Lives next to config.json in data/.
func tombstonePath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "seeded-deleted.json"), nil
}

// loadTombstones reads the set of aliases the user has explicitly removed.
// Missing file ⇒ empty set (no error).
func loadTombstones() (map[string]bool, error) {
	p, err := tombstonePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var aliases []string
	if err := json.Unmarshal(b, &aliases); err != nil {
		// Corrupt tombstone: treat as empty rather than blocking startup.
		return map[string]bool{}, nil
	}
	out := make(map[string]bool, len(aliases))
	for _, a := range aliases {
		out[a] = true
	}
	return out, nil
}

// MarkSeedDeleted records that the user removed a default-seed account so
// it will not be re-seeded on subsequent loads. No-op if the alias is not
// part of DefaultSeedAccounts. Safe to call concurrently — write is
// atomic via tmp+rename.
func MarkSeedDeleted(alias string) error {
	if !isDefaultSeed(alias) {
		return nil
	}
	tomb, err := loadTombstones()
	if err != nil {
		return errtrace.Wrap(err, "MarkSeedDeleted.loadTombstones")
	}
	if tomb[alias] {
		return nil
	}
	tomb[alias] = true
	aliases := make([]string, 0, len(tomb))
	for a := range tomb {
		aliases = append(aliases, a)
	}
	b, err := json.MarshalIndent(aliases, "", "  ")
	if err != nil {
		return errtrace.Wrap(err, "MarkSeedDeleted.marshal")
	}
	p, err := tombstonePath()
	if err != nil {
		return errtrace.Wrap(err, "MarkSeedDeleted.tombstonePath")
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errtrace.Wrap(err, "MarkSeedDeleted.writeTmp")
	}
	return errtrace.Wrap(os.Rename(tmp, p), "MarkSeedDeleted.rename")
}

// isDefaultSeed reports whether alias is part of the default seed set.
func isDefaultSeed(alias string) bool {
	for _, a := range DefaultSeedAccounts {
		if a.Alias == alias {
			return true
		}
	}
	return false
}

// applySeedDefaults inserts any DefaultSeedAccounts that are not already
// present in c.Accounts AND have not been tombstoned by the user. Returns
// true if c was mutated (caller should Save). Pure in-memory operation —
// it never writes config.json itself.
//
// Test seam: when the env var EMAIL_READ_DISABLE_SEED is set to a
// truthy value ("1"/"true"), seeding is fully disabled. Tests that
// rely on a pristine empty config (e.g. withIsolatedConfig in
// internal/core) flip this so deleted-and-reloaded config files do
// not re-acquire the demo account between subtests.
func applySeedDefaults(c *Config) bool {
	if v := os.Getenv("EMAIL_READ_DISABLE_SEED"); v == "1" || v == "true" {
		return false
	}
	tomb, err := loadTombstones()
	if err != nil {
		// If we cannot read tombstones, do NOT seed — better to show no
		// account than to keep resurrecting one the user deleted.
		return false
	}
	mutated := false
	for _, seed := range DefaultSeedAccounts {
		if tomb[seed.Alias] {
			continue
		}
		if c.FindAccount(seed.Alias) != nil {
			continue
		}
		if seed.PasswordB64 == "" {
			pw := resolveSeedPassword()
			if pw == "" {
				// No env-supplied password ⇒ skip seeding this entry
				// rather than persist an unusable account.
				continue
			}
			seed.PasswordB64 = pw
		}
		c.Accounts = append(c.Accounts, seed)
		mutated = true
	}
	return mutated
}
