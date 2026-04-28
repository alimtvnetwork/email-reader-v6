// Package mockimap provides a minimal in-process IMAP server for E2E
// tests of the watcher loop. It listens on 127.0.0.1:0, speaks just
// enough of RFC 3501 to satisfy github.com/emersion/go-imap/client
// (CAPABILITY, LOGIN, SELECT, NOOP, FETCH, LOGOUT) and lets tests
// inject deliveries to drive watcher.Run() through real polling cycles
// without touching a real mail server.
//
// Scope per E2E starter slice E.1: mock server + Listen() + Stop();
// E.2 adds the FETCH plumbing exercised by the watcher; E.3 adds delivery
// injection; E.4 adds AUTHENTICATE failure injection for negative-path
// tests. This file ships E.1 only — additional commands land in follow-up
// slices keyed off the same Server type so the API stays stable.
//
// NOT a real IMAP server. Specifically out of scope: TLS, IDLE, multi-
// folder hierarchies, message bodies, search, flags beyond \Seen,
// SASL, and any concurrency contract beyond "one client at a time".
package mockimap

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/lovable/email-read/internal/errtrace"
)

// Message is the minimal envelope mockimap returns from FETCH. Bodies are
// not modeled — watcher only consumes header fields today, so adding body
// support belongs in the slice that needs it.
type Message struct {
	UID        uint32
	From, To   string
	Subject    string
	ReceivedAt string // RFC822-ish; passed through verbatim to FETCH responses
	Seen       bool
}

// Mailbox is a flat ordered list of messages with a stable UidNext.
// Tests mutate it via Server.Deliver(); the server reads it under lock.
type Mailbox struct {
	Name    string
	Msgs    []Message
	UidNext uint32
}

// Server is a one-shot listener you Start() once per test and Stop() in
// a defer. Concurrent test reuse is not supported — spin up a new one.
type Server struct {
	// Login credentials the client must present. Empty Password means
	// any password is accepted (handy for happy-path tests).
	User, Password string

	mu       sync.Mutex
	mailbox  Mailbox
	ln       net.Listener
	stopCh   chan struct{}
	doneCh   chan struct{}
	addr     string
	failNext string // when non-empty, next LOGIN returns this error code
}

// New builds a server with one mailbox ("INBOX") seeded with `seed`. The
// server is NOT started — call Start() to bind a port.
func New(user, password string, seed []Message) *Server {
	mb := Mailbox{Name: "INBOX", Msgs: append([]Message(nil), seed...), UidNext: 1}
	for i := range mb.Msgs {
		if mb.Msgs[i].UID == 0 {
			mb.Msgs[i].UID = mb.UidNext
		}
		if mb.Msgs[i].UID >= mb.UidNext {
			mb.UidNext = mb.Msgs[i].UID + 1
		}
	}
	return &Server{User: user, Password: password, mailbox: mb,
		stopCh: make(chan struct{}), doneCh: make(chan struct{})}
}

// Start binds 127.0.0.1:0, returns the host:port the client should dial,
// and spawns the accept loop. Safe to call once.
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", errtrace.Wrap(err, "mockimap: listen")
	}
	s.ln = ln
	s.addr = ln.Addr().String()
	go s.acceptLoop()
	return s.addr, nil
}

// Addr returns host:port. Empty before Start().
func (s *Server) Addr() string { return s.addr }

// Stop closes the listener and blocks until the accept loop exits.
func (s *Server) Stop() {
	select {
	case <-s.stopCh:
		return // already stopped
	default:
	}
	close(s.stopCh)
	if s.ln != nil {
		_ = s.ln.Close()
	}
	<-s.doneCh
}

// Deliver appends a new message and bumps UidNext. Thread-safe; callable
// at any time during a test, including while a client is mid-FETCH.
func (s *Server) Deliver(m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.UID == 0 {
		m.UID = s.mailbox.UidNext
	}
	if m.UID >= s.mailbox.UidNext {
		s.mailbox.UidNext = m.UID + 1
	}
	s.mailbox.Msgs = append(s.mailbox.Msgs, m)
}

// FailNextLogin makes the very next LOGIN respond with `BAD <reason>`.
// Cleared after one use. Used by E.4 negative-path scaffolding.
func (s *Server) FailNextLogin(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNext = reason
}

// MessageCount returns the current mailbox size. Useful for assertions
// that don't want to crack the protocol.
func (s *Server) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mailbox.Msgs)
}

// --- accept loop -----------------------------------------------------------

func (s *Server) acceptLoop() {
	defer close(s.doneCh)
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed by Stop()
		}
		go s.serveConn(conn)
	}
}

func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	// Greeting per RFC 3501 §7.1.
	_, _ = w.WriteString("* OK [CAPABILITY IMAP4rev1] mockimap ready\r\n")
	_ = w.Flush()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if !s.handle(strings.TrimRight(line, "\r\n"), w) {
			return
		}
		_ = w.Flush()
	}
}

// handle dispatches one tagged command. Returns false to close the conn
// (after LOGOUT or on protocol error). Kept tiny — each command branches
// to a helper; new commands land as new helpers in follow-up slices.
func (s *Server) handle(line string, w *bufio.Writer) bool {
	tag, cmd, rest := splitCommand(line)
	if tag == "" {
		_, _ = w.WriteString("* BAD empty tag\r\n")
		return true
	}
	switch strings.ToUpper(cmd) {
	case "CAPABILITY":
		_, _ = w.WriteString("* CAPABILITY IMAP4rev1\r\n")
		_, _ = fmt.Fprintf(w, "%s OK CAPABILITY completed\r\n", tag)
	case "LOGIN":
		s.handleLogin(tag, rest, w)
	case "SELECT", "EXAMINE":
		s.handleSelect(tag, w)
	case "NOOP":
		_, _ = fmt.Fprintf(w, "%s OK NOOP completed\r\n", tag)
	case "LOGOUT":
		_, _ = w.WriteString("* BYE mockimap closing\r\n")
		_, _ = fmt.Fprintf(w, "%s OK LOGOUT completed\r\n", tag)
		return false
	default:
		// Unknown commands respond BAD so the client surfaces a real error
		// rather than hanging. New commands (FETCH, UID FETCH) land here
		// in slice E.2.
		_, _ = fmt.Fprintf(w, "%s BAD unknown command %q (mockimap E.1 scope)\r\n", tag, cmd)
	}
	return true
}

func (s *Server) handleLogin(tag, rest string, w *bufio.Writer) {
	s.mu.Lock()
	failReason := s.failNext
	s.failNext = ""
	want := s.Password
	user := s.User
	s.mu.Unlock()

	if failReason != "" {
		_, _ = fmt.Fprintf(w, "%s NO [AUTHENTICATIONFAILED] %s\r\n", tag, failReason)
		return
	}
	gotUser, gotPass := parseLoginArgs(rest)
	if user != "" && gotUser != user {
		_, _ = fmt.Fprintf(w, "%s NO [AUTHENTICATIONFAILED] unknown user\r\n", tag)
		return
	}
	if want != "" && gotPass != want {
		_, _ = fmt.Fprintf(w, "%s NO [AUTHENTICATIONFAILED] bad password\r\n", tag)
		return
	}
	_, _ = fmt.Fprintf(w, "%s OK LOGIN completed\r\n", tag)
}

func (s *Server) handleSelect(tag string, w *bufio.Writer) {
	s.mu.Lock()
	mb := s.mailbox
	s.mu.Unlock()
	_, _ = fmt.Fprintf(w, "* %d EXISTS\r\n", len(mb.Msgs))
	_, _ = w.WriteString("* 0 RECENT\r\n")
	_, _ = w.WriteString("* FLAGS (\\Seen)\r\n")
	_, _ = fmt.Fprintf(w, "* OK [UIDVALIDITY 1] mockimap\r\n")
	_, _ = fmt.Fprintf(w, "* OK [UIDNEXT %d] next\r\n", mb.UidNext)
	_, _ = fmt.Fprintf(w, "%s OK [READ-WRITE] SELECT completed\r\n", tag)
}

// splitCommand parses `<tag> <CMD> <rest>` (rest may be empty).
func splitCommand(line string) (tag, cmd, rest string) {
	parts := strings.SplitN(line, " ", 3)
	switch len(parts) {
	case 0:
		return
	case 1:
		return parts[0], "", ""
	case 2:
		return parts[0], parts[1], ""
	default:
		return parts[0], parts[1], parts[2]
	}
}

// parseLoginArgs extracts user + password from `LOGIN "u" "p"` or
// `LOGIN u p` (unquoted). Sufficient for what go-imap/client emits.
func parseLoginArgs(rest string) (user, pass string) {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, `"`) {
		// Two quoted strings.
		end := strings.Index(rest[1:], `"`)
		if end < 0 {
			return
		}
		user = rest[1 : 1+end]
		rest = strings.TrimSpace(rest[2+end:])
		if strings.HasPrefix(rest, `"`) {
			end2 := strings.Index(rest[1:], `"`)
			if end2 < 0 {
				return
			}
			pass = rest[1 : 1+end2]
		}
		return
	}
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) >= 1 {
		user = parts[0]
	}
	if len(parts) == 2 {
		pass = parts[1]
	}
	return
}
