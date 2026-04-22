// Package config handles loading and saving the email-read CLI configuration
// from data/config.json (located next to the executable).
package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/lovable/email-read/internal/errtrace"
)

// Account represents a single IMAP account.
type Account struct {
	Alias       string `json:"alias"`
	Email       string `json:"email"`
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
	mu sync.Mutex
)

// Default returns a sensible empty config.
func Default() *Config {
	return &Config{
		Accounts: []Account{},
		Rules:    []Rule{},
		Watch:    Watch{PollSeconds: 3},
		Browser:  Browser{},
	}
}

// EncodePassword base64-encodes a plaintext password for storage.
// Note: this is encoding, not encryption — explicitly accepted by the user.
func EncodePassword(plain string) string {
	return base64.StdEncoding.EncodeToString([]byte(plain))
}

// DecodePassword base64-decodes a stored password back to plaintext.
func DecodePassword(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", errtrace.Wrap(err, "decode password")
	}
	return string(b), nil
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
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, errtrace.Wrapf(err, "read config %s", p)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, errtrace.Wrapf(err, "parse config %s", p)
	}
	if c.Watch.PollSeconds <= 0 {
		c.Watch.PollSeconds = 3
	}
	if c.Accounts == nil {
		c.Accounts = []Account{}
	}
	if c.Rules == nil {
		c.Rules = []Rule{}
	}
	return &c, nil
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
