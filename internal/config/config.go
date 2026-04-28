// Package config handles loading and saving the email-read CLI configuration
// from data/config.json (located next to the executable).
package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/lovable/email-read/internal/errtrace"
)

// Account represents a single IMAP account.
type Account struct {
	Alias       string `json:"alias"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"` // optional human label (e.g. "Work — Sales")
	PasswordB64 string `json:"passwordB64"`
	ImapHost    string `json:"imapHost"`
	ImapPort    int    `json:"imapPort"`
	UseTLS      bool   `json:"useTLS"`
	Mailbox     string `json:"mailbox"`
}

// Rule represents one regex-based auto-open rule.
type Rule struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	FromRegex    string `json:"fromRegex"`
	SubjectRegex string `json:"subjectRegex"`
	BodyRegex    string `json:"bodyRegex"`
	UrlRegex     string `json:"urlRegex"`
}

// Watch holds polling configuration.
type Watch struct {
	PollSeconds int `json:"pollSeconds"`
}

// DefaultWatchPollSeconds is the user-requested default Watch cadence.
const DefaultWatchPollSeconds = 5

// Watch poll bounds keep Settings validation and runtime fallbacks from
// accepting zero/negative tight loops while still allowing fast 5s polling.
const (
	MinWatchPollSeconds = 1
	MaxWatchPollSeconds = 60
)

// Browser holds Chrome/Chromium launcher configuration.
type Browser struct {
	ChromePath   string `json:"chromePath"`
	IncognitoArg string `json:"incognitoArg"`
}

// Config is the full on-disk config schema.
type Config struct {
	Accounts []Account `json:"accounts"`
	Rules    []Rule    `json:"rules"`
	Watch    Watch     `json:"watch"`
	Browser  Browser   `json:"browser"`
}

var (
	// mu serialises a single Load or Save against concurrent callers.
	mu sync.Mutex

	// writeMu serialises an entire read-modify-write transaction
	// against the config file. Acquire via WithWriteLock when a caller
	// needs to Load, mutate, and Save without another goroutine
	// interleaving its own Save in between.
	//
	// CF-A2 (spec/21-app/02-features/07-settings/99-consistency-report.md):
	// without this lock, Settings.Save and AddAccount/RemoveAccount can
	// race and silently lose either side's update because each one's
	// Load+Save pair is only individually serialised by `mu`.
	writeMu sync.Mutex
)

// WithWriteLock runs fn while holding the process-wide config write
// lock. Use this to make Load+mutate+Save atomic across goroutines.
// fn must NOT block on anything that itself calls back into config
// (deadlock risk).
func WithWriteLock(fn func()) {
	writeMu.Lock()
	defer writeMu.Unlock()
	fn()
}

// Default returns a sensible empty config.
func Default() *Config {
	return &Config{
		Accounts: []Account{},
		Rules:    []Rule{},
		Watch:    Watch{PollSeconds: DefaultWatchPollSeconds},
		Browser:  Browser{},
	}
}

// SanitizePassword strips leading/trailing whitespace AND zero-width / format
// Unicode characters that frequently sneak in via copy-paste from chat / web
// (e.g. U+2060 WORD JOINER, U+200B ZERO-WIDTH SPACE, U+FEFF BOM, U+00A0 NBSP).
// IMAP servers reject the literal bytes, so we sanitize defensively at every
// boundary. Returns the cleaned password.
func SanitizePassword(s string) string {
	// First trim ASCII whitespace.
	s = strings.TrimSpace(s)
	// Then strip leading/trailing zero-width or format runes.
	s = strings.TrimFunc(s, func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}
		// Format category (Cf) covers WORD JOINER, ZWSP, ZWNJ, BOM, etc.
		if unicode.Is(unicode.Cf, r) {
			return true
		}
		switch r {
		case '\u00A0', '\u2007', '\u202F', '\u3000':
			return true
		}
		return false
	})
	return s
}

// EncodePassword base64-encodes a plaintext password for storage.
// Note: this is encoding, not encryption — explicitly accepted by the user.
// The input is sanitized to remove invisible characters that copy-paste from
// chat/markdown commonly introduces (U+2060 word joiner, U+200B ZWSP, etc.).
func EncodePassword(plain string) string {
	return base64.StdEncoding.EncodeToString([]byte(SanitizePassword(plain)))
}

// DecodePassword base64-decodes a stored password back to plaintext.
// The result is sanitized as a defense-in-depth measure for accounts that
// were stored before sanitization was added on the encode side.
func DecodePassword(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", errtrace.Wrap(err, "decode password")
	}
	return SanitizePassword(string(b)), nil
}

// DecodeRawPassword base64-decodes a stored password WITHOUT sanitization.
// Used by `core.Doctor` to expose what's truly stored on disk so the user
// can see hidden whitespace / zero-width chars before they hit IMAP.
// Never call this for actual login — use DecodePassword.
func DecodeRawPassword(b64 string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, errtrace.Wrap(err, "decode raw password")
	}
	return b, nil
}

// ExeDir returns the directory that holds the running executable.
// Falls back to the current working directory in tests / `go run`.
func ExeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return os.Getwd()
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	return filepath.Dir(resolved), nil
}

// DataDir returns the data/ directory next to the executable, creating it if needed.
func DataDir() (string, error) {
	root, err := ExeDir()
	if err != nil {
		return "", errtrace.Wrap(err, "exe dir")
	}
	dir := filepath.Join(root, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errtrace.Wrap(err, "create data dir")
	}
	return dir, nil
}

// EmailDir returns the email/ directory next to the executable, creating it if needed.
func EmailDir() (string, error) {
	root, err := ExeDir()
	if err != nil {
		return "", errtrace.Wrap(err, "exe dir")
	}
	dir := filepath.Join(root, "email")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errtrace.Wrap(err, "create email dir")
	}
	return dir, nil
}

// Path returns the absolute path of data/config.json.
func Path() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", errtrace.Wrap(err, "path: data dir")
	}
	return filepath.Join(d, "config.json"), nil
}

// Load reads config.json from disk. If it does not exist, a default config is returned.
func Load() (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	p, err := Path()
	if err != nil {
		return nil, errtrace.Wrap(err, "load: path")
	}
	b, err := os.ReadFile(p)
	var c Config
	switch {
	case err != nil && os.IsNotExist(err):
		c = *Default()
	case err != nil:
		return nil, errtrace.Wrapf(err, "read config %s", p)
	default:
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, errtrace.Wrapf(err, "parse config %s", p)
		}
	}
	if c.Watch.PollSeconds <= 0 {
		c.Watch.PollSeconds = DefaultWatchPollSeconds
	}
	if c.Accounts == nil {
		c.Accounts = []Account{}
	}
	if c.Rules == nil {
		c.Rules = []Rule{}
	}
	// Seed default accounts (skipping any the user has tombstoned).
	// Persist immediately so the seeded entries survive even if the
	// caller never triggers a Save of its own.
	if applySeedDefaults(&c) {
		if err := saveLocked(&c, p); err != nil {
			// Non-fatal: surface the seeded copy in memory anyway so
			// the UI is usable; next Save attempt will retry.
			_ = err
		}
	}
	return &c, nil
}

// saveLocked writes c to path p. Caller MUST already hold mu (used by
// Load's seed-write path to avoid double-locking).
func saveLocked(c *Config, p string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return errtrace.Wrap(err, "marshal config")
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errtrace.Wrapf(err, "write config %s", tmp)
	}
	return os.Rename(tmp, p)
}

// Save atomically writes the config to disk with pretty-printed JSON.
func Save(c *Config) error {
	mu.Lock()
	defer mu.Unlock()

	p, err := Path()
	if err != nil {
		return errtrace.Wrap(err, "save: path")
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return errtrace.Wrap(err, "marshal config")
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errtrace.Wrapf(err, "write config %s", tmp)
	}
	if err := os.Rename(tmp, p); err != nil {
		return errtrace.Wrapf(err, "rename config %s -> %s", tmp, p)
	}
	return nil
}

// FindAccount returns a pointer to the account with the given alias, or nil.
func (c *Config) FindAccount(alias string) *Account {
	for i := range c.Accounts {
		if c.Accounts[i].Alias == alias {
			return &c.Accounts[i]
		}
	}
	return nil
}

// RemoveAccount removes the account with the given alias. Returns true if removed.
func (c *Config) RemoveAccount(alias string) bool {
	for i := range c.Accounts {
		if c.Accounts[i].Alias == alias {
			c.Accounts = append(c.Accounts[:i], c.Accounts[i+1:]...)
			return true
		}
	}
	return false
}

// UpsertAccount adds or replaces an account by alias.
func (c *Config) UpsertAccount(a Account) {
	if existing := c.FindAccount(a.Alias); existing != nil {
		*existing = a
		return
	}
	c.Accounts = append(c.Accounts, a)
}

// FindRule returns a pointer to the rule with the given name, or nil.
func (c *Config) FindRule(name string) *Rule {
	for i := range c.Rules {
		if c.Rules[i].Name == name {
			return &c.Rules[i]
		}
	}
	return nil
}
