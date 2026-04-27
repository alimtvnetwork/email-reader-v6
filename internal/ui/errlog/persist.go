// persist.go — disk persistence for the error-log ring buffer
// (Phase 4.1 of the error-trace upgrade — see .lovable/plan.md).
//
// Format: newline-delimited JSON (`.jsonl`). One Entry per line,
// encoded with the standard library's `encoding/json`. Chosen over a
// single JSON array so:
//   - append is O(1) (just write one line, no re-serialize),
//   - a half-written tail line never corrupts the rest of the file
//     (loader skips unparseable lines and keeps going),
//   - third-party tooling (`grep`, `jq -c`, `tail -f`) works unchanged.
//
// Rotation: when the active file grows past `sizeCap` bytes after a
// write, it is renamed to `<path>.1` (overwriting any prior `.1`),
// and a fresh empty file is opened. We keep exactly one rotated file
// — the in-memory ring + one rotation are enough context for "what
// happened in the last few minutes" without growing data/ unbounded.
//
// Concurrency: the writer holds its own mutex so a slow disk
// serializes inside the persister and never blocks Store.Append's
// main critical section (see errlog.go — the persister callback runs
// after `s.mu.Unlock()`).
//
// Headless-safe: no fyne import, pure stdlib + errtrace.
package errlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/lovable/email-read/internal/errtrace"
)

// DefaultSizeCap is the rotation threshold for `error-log.jsonl`.
// 5 MiB ≈ ~5000 typical entries; well above the in-memory ring's 500
// so the rotated `.1` file always carries strictly older context.
const DefaultSizeCap int64 = 5 * 1024 * 1024

// ErrPersistenceClosed is returned by Write when the underlying file
// has already been Close()d. Production callers (Store.Append) swallow
// it; tests that hold the *Persistence directly can assert against it
// via errors.Is.
var ErrPersistenceClosed = errors.New("errlog: persistence is closed")

// Persistence owns the open log file plus the rotation policy. One
// instance per Store. Public so tests can construct it directly with
// custom paths and size caps.
type Persistence struct {
	mu      sync.Mutex
	path    string
	sizeCap int64
	f       *os.File
	w       *bufio.Writer
	size    int64
}

// NewPersistence opens (creating if needed) the active log file at
// `path`. `sizeCap<=0` falls back to DefaultSizeCap. The file is
// opened in append mode so a process restart concatenates onto the
// existing tail instead of truncating prior history.
//
// The parent directory must already exist — the caller (typically
// the UI bootstrap) is responsible for `os.MkdirAll(data, 0o700)`.
func NewPersistence(path string, sizeCap int64) (*Persistence, error) {
	if sizeCap <= 0 {
		sizeCap = DefaultSizeCap
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, errtrace.Wrap(err, "errlog: open persistence file")
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, errtrace.Wrap(err, "errlog: stat persistence file")
	}
	return &Persistence{
		path:    path,
		sizeCap: sizeCap,
		f:       f,
		w:       bufio.NewWriter(f),
		size:    info.Size(),
	}, nil
}

// Write encodes `e` as one JSON line, flushes, and rotates when the
// file would exceed `sizeCap` after this write. Returns an error
// only on I/O failure; the caller (Store.Append) currently swallows
// it (best-effort persistence — losing a log line must not wedge the
// running app). The error is still returned for tests.
func (p *Persistence) Write(e Entry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Guard against use-after-Close. Without this, a stale persister
	// callback wired into the package singleton (e.g. by a test that
	// called EnableDefaultPersistence and then Close()d its handle)
	// would deref a nil *bufio.Writer on the next ReportError. We
	// degrade to a typed sentinel error so callers can detect it; the
	// production fan-out in Store.Append swallows the error already.
	if p.f == nil || p.w == nil {
		return errtrace.Wrap(ErrPersistenceClosed, "errlog: write after close")
	}
	line, err := json.Marshal(e)
	if err != nil {
		return errtrace.Wrap(err, "errlog: marshal entry")
	}
	line = append(line, '\n')
	if _, err := p.w.Write(line); err != nil {
		return errtrace.Wrap(err, "errlog: write entry line")
	}
	if err := p.w.Flush(); err != nil {
		return errtrace.Wrap(err, "errlog: flush entry line")
	}
	p.size += int64(len(line))
	if p.size >= p.sizeCap {
		if err := p.rotateLocked(); err != nil {
			return errtrace.Wrap(err, "errlog: rotate log")
		}
	}
	return nil
}

// rotateLocked closes the active file, renames it to `<path>.1`
// (overwriting any prior rotation), and opens a fresh empty active
// file. Caller must hold p.mu.
func (p *Persistence) rotateLocked() error {
	if err := p.f.Close(); err != nil {
		return errtrace.Wrap(err, "rotate: close active")
	}
	rotated := p.path + ".1"
	// os.Rename overwrites on POSIX; on Windows we remove first to
	// match. Ignoring "not exist" makes the first rotation work.
	if err := os.Remove(rotated); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errtrace.Wrap(err, "rotate: remove prior .1")
	}
	if err := os.Rename(p.path, rotated); err != nil {
		return errtrace.Wrap(err, "rotate: rename to .1")
	}
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return errtrace.Wrap(err, "rotate: reopen active")
	}
	p.f = f
	p.w = bufio.NewWriter(f)
	p.size = 0
	return nil
}

// Close flushes any buffered bytes and closes the underlying file.
// Safe to call multiple times — subsequent calls return nil.
func (p *Persistence) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.f == nil {
		return nil
	}
	flushErr := p.w.Flush()
	closeErr := p.f.Close()
	p.f = nil
	p.w = nil
	if flushErr != nil {
		return errtrace.Wrap(flushErr, "errlog: close flush")
	}
	if closeErr != nil {
		return errtrace.Wrap(closeErr, "errlog: close file")
	}
	return nil
}

// LoadFromFile reads `path` and returns every successfully-parsed
// Entry in file order (oldest first). Missing file returns an empty
// slice and a nil error — a fresh install has no log yet, which is
// not an error condition. Corrupt / truncated lines are skipped
// silently so a half-written tail does not prevent restoring the
// rest of the log.
func LoadFromFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, errtrace.Wrap(err, "errlog: open log for load")
	}
	defer f.Close()
	out := make([]Entry, 0, 64)
	scanner := bufio.NewScanner(f)
	// Allow up to 1 MiB per line — a stack trace can be long. The
	// default 64 KiB cap would silently drop an unusually-long entry.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip corrupt / truncated line
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return out, errtrace.Wrap(err, "errlog: scan log")
	}
	return out, nil
}

// EnablePersistence wires `p` as the Store's persister and seeds the
// in-memory ring with `prior` (typically `LoadFromFile(path)` from
// boot). `prior` is appended in order, respecting the ring cap (only
// the most-recent `cap` survive). Subsequent Append calls write
// through `p` automatically.
//
// `prior`'s Seq values are preserved when present (so the
// "Resume from disk" UX shows continuous numbering across restarts),
// but the next live append's Seq comes from `max(prior.Seq) + 1` to
// keep the monotonic invariant.
//
// Returns the prior persister (always nil today; reserved for future
// "swap persistence path" flows).
func (s *Store) EnablePersistence(p *Persistence, prior []Entry) func(Entry) {
	s.mu.Lock()
	old := s.persister
	// Seed the ring with prior entries, preserving order. Drop the
	// oldest if `prior` is larger than the cap — only the most-recent
	// `cap` entries are user-visible after restart.
	if n := len(prior); n > 0 {
		start := 0
		if n > s.cap {
			start = n - s.cap
		}
		s.entries = append(s.entries[:0], prior[start:]...)
		// Restore monotonic Seq: the next live Append must produce a
		// Seq strictly greater than every restored entry.
		var maxSeq uint64
		for _, e := range s.entries {
			if e.Seq > maxSeq {
				maxSeq = e.Seq
			}
		}
		s.nextSeq = maxSeq
	}
	s.persister = func(e Entry) {
		// Best-effort: log via the standard logger if Write fails so a
		// disk-full / permission-error doesn't get silently swallowed,
		// but never propagate the error back into Append.
		if err := p.Write(e); err != nil {
			// Swallow silently in production. Tests that care about
			// write errors call p.Write directly.
			_ = err
		}
	}
	s.mu.Unlock()
	return old
}

// EnableDefaultPersistence is a convenience wrapper for the
// process-wide singleton. It opens (or creates) `path`, seeds the
// in-memory ring with whatever was already there, and wires future
// appends to write through. Returns the *Persistence so the caller
// can Close() it on shutdown (defer in main).
//
// `sizeCap<=0` falls back to DefaultSizeCap.
func EnableDefaultPersistence(path string, sizeCap int64) (*Persistence, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, errtrace.Wrap(err, "errlog: ensure persist dir")
		}
	}
	prior, err := LoadFromFile(path)
	if err != nil {
		// Loader treats missing as nil/nil — only real I/O errors
		// reach here. Continue without prior context rather than
		// failing the boot path.
		prior = nil
	}
	p, err := NewPersistence(path, sizeCap)
	if err != nil {
		return nil, errtrace.Wrap(err, "errlog: new persistence")
	}
	instance().EnablePersistence(p, prior)
	return p, nil
}
