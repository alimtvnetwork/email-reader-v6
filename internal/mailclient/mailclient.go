// Package mailclient connects to an IMAP server, fetches new messages by UID,
// parses them into a normalized struct, and saves the raw .eml to disk under
// email/<alias>/<YYYY-MM-DD>/<safe-message-id>.eml.
package mailclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	netmail "net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // register extra charsets
	"github.com/emersion/go-message/mail"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// Message is the parsed representation of an email used by downstream layers.
type Message struct {
	Uid        uint32
	MessageId  string
	From       string
	To         string
	Cc         string
	Subject    string
	BodyText   string
	BodyHtml   string
	ReceivedAt time.Time
	Raw        []byte
}

// Client wraps an IMAP connection for one account.
type Client struct {
	acct config.Account
	c    *client.Client
}

// DefaultDialTimeout caps the TCP/TLS handshake for live watcher
// dials. Without this, a silently-dropping IMAP host (firewall,
// routing blackhole, port closed) makes the watcher block on the OS
// default (~75s on Linux) before reporting `[ER-MAIL-21201]`.
// Keep this generous: some shared-hosting mail servers and home networks
// can take longer than a few seconds even when the endpoint is healthy.
const DefaultDialTimeout = 30 * time.Second

// Dial opens an IMAP connection and logs in.
func Dial(acct config.Account) (*Client, error) {
	return DialContext(context.Background(), acct)
}

// DialContext opens an IMAP connection and logs in, closing the underlying
// socket promptly when ctx is cancelled. This lets Watch.Stop interrupt an
// in-flight dial/greeting/login instead of waiting for the timeout budget.
func DialContext(ctx context.Context, acct config.Account) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := dialAccountWithFallback(ctx, acct)
	if err != nil {
		return nil, err
	}
	pwd, err := config.DecodePassword(acct.PasswordB64)
	if err != nil {
		_ = c.Logout()
		return nil, errtrace.Wrap(err, "decode password")
	}
	if err := c.Login(acct.Email, pwd); err != nil {
		_ = c.Logout()
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, errtrace.Wrapf(err, "imap login %s", acct.Email)
	}
	return &Client{acct: acct, c: c}, nil
}

func dialAccountWithFallback(ctx context.Context, acct config.Account) (*client.Client, error) {
	c, err := dialAccountEndpoint(ctx, acct.ImapHost, acct.ImapPort, acct.UseTLS)
	if err == nil {
		return c, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if !shouldTryStartTLSFallback(acct, err) {
		return nil, err
	}
	fallback, fallbackErr := dialAccountStartTLS(ctx, acct.ImapHost, 143)
	if fallbackErr == nil {
		return fallback, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	return nil, errtrace.Wrapf(fallbackErr, "imap STARTTLS fallback after %s timed out", imapAddr(acct.ImapHost, acct.ImapPort))
}

func dialAccountEndpoint(ctx context.Context, host string, port int, useTLS bool) (*client.Client, error) {
	addr := imapAddr(host, port)
	dialer := &contextDialer{ctx: ctx, timeout: DefaultDialTimeout}
	stopAbort := dialer.abortOnCancel()
	defer stopAbort()
	var (
		c   *client.Client
		err error
	)
	if useTLS {
		c, err = client.DialWithDialerTLS(dialer, addr, &tls.Config{ServerName: host})
	} else {
		c, err = client.DialWithDialer(dialer, addr)
	}
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, errtrace.Wrapf(err, "imap dial %s", addr)
	}
	c.Timeout = DefaultDialTimeout
	return c, nil
}

func dialAccountStartTLS(ctx context.Context, host string, port int) (*client.Client, error) {
	c, err := dialAccountEndpoint(ctx, host, port, false)
	if err != nil {
		return nil, err
	}
	if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
		_ = c.Logout()
		return nil, errtrace.Wrapf(err, "imap starttls %s", imapAddr(host, port))
	}
	return c, nil
}

func shouldTryStartTLSFallback(acct config.Account, err error) bool {
	return acct.UseTLS && acct.ImapPort == 993 && isNetTimeout(err)
}

func imapAddr(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

type contextDialer struct {
	ctx     context.Context
	timeout time.Duration
	mu      sync.Mutex
	conn    net.Conn
	aborted bool
}

func (d *contextDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: d.timeout}).DialContext(d.ctx, network, addr)
	if err != nil {
		return nil, err
	}
	if d.timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(d.timeout)); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	d.mu.Lock()
	aborted := d.aborted
	d.conn = conn
	d.mu.Unlock()
	if aborted || d.ctx.Err() != nil {
		_ = conn.Close()
		return nil, d.ctx.Err()
	}
	return conn, nil
}

func (d *contextDialer) abortOnCancel() func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-d.ctx.Done():
			d.closeConn()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func (d *contextDialer) closeConn() {
	d.mu.Lock()
	d.aborted = true
	conn := d.conn
	d.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Close logs out and closes the connection.
func (c *Client) Close() error {
	if c == nil || c.c == nil {
		return nil
	}
	return c.c.Logout()
}

// MailboxStats summarises the selected mailbox so callers can log it.
type MailboxStats struct {
	Name        string // e.g. "INBOX"
	Messages    uint32 // total messages in the mailbox
	Recent      uint32 // \Recent count
	Unseen      uint32 // \Seen=false count (0 if server didn't report)
	UidNext     uint32 // next UID the server will assign
	UidValidity uint32 // changes when UID space is reset
}

// SelectInbox selects the configured mailbox (defaults to INBOX) and returns
// detailed mailbox statistics. UidNext-1 is the highest UID currently present
// — useful for first-run baselines and per-poll diagnostics.
func (c *Client) SelectInbox() (MailboxStats, error) {
	box := c.acct.Mailbox
	if box == "" {
		box = "INBOX"
	}
	return c.SelectMailbox(box)
}

// SelectMailbox selects a specific mailbox/folder and returns detailed stats.
func (c *Client) SelectMailbox(box string) (MailboxStats, error) {
	if box == "" {
		box = "INBOX"
	}
	mbox, err := c.c.Select(box, false)
	if err != nil {
		return MailboxStats{Name: box}, errtrace.Wrapf(err, "select %s", box)
	}
	return MailboxStats{
		Name:        box,
		Messages:    mbox.Messages,
		Recent:      mbox.Recent,
		Unseen:      mbox.Unseen,
		UidNext:     mbox.UidNext,
		UidValidity: mbox.UidValidity,
	}, nil
}

// HeaderSummary is a lightweight view of a mailbox message for diagnostics.
type HeaderSummary struct {
	Uid        uint32
	From       string
	To         string
	Subject    string
	ReceivedAt time.Time
}

// MailboxName describes one selectable mailbox/folder exposed by the server.
type MailboxName struct {
	Name       string
	Delimiter  string
	Attributes []string
}

// ListMailboxes returns the folders visible to the account. This helps diagnose
// delivery into Spam/Junk/All Mail instead of the configured INBOX.
func (c *Client) ListMailboxes() ([]MailboxName, error) {
	ch := make(chan *imap.MailboxInfo, 32)
	done := make(chan error, 1)
	go func() {
		done <- c.c.List("", "*", ch)
	}()

	var out []MailboxName
	for info := range ch {
		if info == nil {
			continue
		}
		out = append(out, MailboxName{
			Name:       info.Name,
			Delimiter:  string(info.Delimiter),
			Attributes: info.Attributes,
		})
	}
	if err := <-done; err != nil {
		return nil, errtrace.Wrap(err, "list mailboxes")
	}
	return out, nil
}

// FetchRecentHeaders returns up to limit latest message headers from the
// selected mailbox. Call SelectInbox first so UidNext reflects that mailbox.
func (c *Client) FetchRecentHeaders(stats MailboxStats, limit uint32) ([]HeaderSummary, error) {
	if limit == 0 || stats.Messages == 0 || stats.UidNext <= 1 {
		return nil, nil
	}

	lastUID := stats.UidNext - 1
	firstUID := uint32(1)
	if lastUID >= limit {
		firstUID = lastUID - limit + 1
	}

	seq := new(imap.SeqSet)
	seq.AddRange(firstUID, lastUID)

	items := []imap.FetchItem{
		imap.FetchUid,
		imap.FetchEnvelope,
		imap.FetchInternalDate,
	}
	ch := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() {
		done <- c.c.UidFetch(seq, items, ch)
	}()

	var out []HeaderSummary
	for msg := range ch {
		if msg == nil {
			continue
		}
		summary := HeaderSummary{Uid: msg.Uid, ReceivedAt: msg.InternalDate}
		if msg.Envelope != nil {
			summary.Subject = msg.Envelope.Subject
			if summary.ReceivedAt.IsZero() {
				summary.ReceivedAt = msg.Envelope.Date
			}
			if len(msg.Envelope.From) > 0 {
				summary.From = formatAddr(msg.Envelope.From[0])
			}
			if len(msg.Envelope.To) > 0 {
				summary.To = formatAddr(msg.Envelope.To[0])
			}
		}
		out = append(out, summary)
	}
	if err := <-done; err != nil {
		return nil, errtrace.Wrap(err, "fetch recent headers")
	}
	return out, nil
}

// FetchSince returns all messages with UID strictly greater than lastUid.
// Pass 0 on the first run to fetch nothing-but-baseline (callers should typically
// snapshot UIDNEXT before the first poll instead of replaying history).
func (c *Client) FetchSince(lastUid uint32) ([]*Message, error) {
	from := lastUid + 1
	if lastUid == 0 {
		from = 1
	}
	seq := new(imap.SeqSet)
	seq.AddRange(from, 0) // 0 == "*"

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{
		imap.FetchUid,
		imap.FetchEnvelope,
		imap.FetchInternalDate,
		section.FetchItem(),
	}

	ch := make(chan *imap.Message, 16)
	done := make(chan error, 1)
	go func() {
		done <- c.c.UidFetch(seq, items, ch)
	}()

	var out []*Message
	for msg := range ch {
		if msg == nil {
			continue
		}
		// The server sometimes returns messages we already have when range is "1:*".
		if msg.Uid <= lastUid && lastUid != 0 {
			continue
		}
		body := msg.GetBody(section)
		if body == nil {
			continue
		}
		raw, err := io.ReadAll(body)
		if err != nil {
			continue
		}
		parsed, err := parseRaw(raw)
		if err != nil {
			// Fall back to envelope-only data so we still record something.
			parsed = &Message{}
		}
		parsed.Uid = msg.Uid
		if parsed.MessageId == "" && msg.Envelope != nil {
			parsed.MessageId = msg.Envelope.MessageId
		}
		if parsed.Subject == "" && msg.Envelope != nil {
			parsed.Subject = msg.Envelope.Subject
		}
		if parsed.From == "" && msg.Envelope != nil && len(msg.Envelope.From) > 0 {
			parsed.From = formatAddr(msg.Envelope.From[0])
		}
		if parsed.ReceivedAt.IsZero() {
			if msg.InternalDate.IsZero() && msg.Envelope != nil {
				parsed.ReceivedAt = msg.Envelope.Date
			} else {
				parsed.ReceivedAt = msg.InternalDate
			}
		}
		if parsed.MessageId == "" {
			parsed.MessageId = fmt.Sprintf("<no-id-%s-uid-%d@email-read>", c.acct.Alias, msg.Uid)
		}
		parsed.Raw = raw
		out = append(out, parsed)
	}
	if err := <-done; err != nil {
		return nil, errtrace.Wrap(err, "uid fetch")
	}
	return out, nil
}

// SaveRaw writes the raw .eml file under
//
//	email/<alias>/<YYYY-MM-DD>/HH.MM.SS__<from>__<subject>__uid<N>.eml
//
// and returns the absolute path written.
//
// The filename format is human-readable and time-ordered:
//   - HH.MM.SS prefix (from the message's ReceivedAt) so files sort
//     chronologically inside the day folder.
//   - sanitized sender address (e.g. "abdullah-mahin-rasia-gmail-com").
//   - sanitized subject (trimmed to keep total filename reasonable).
//   - "__uidN" suffix as the dedup key — guarantees uniqueness even when
//     two emails share second + sender + subject.
func SaveRaw(alias string, m *Message) (string, error) {
	dir, when, err := resolveSaveDir(alias, m)
	if err != nil {
		return "", err
	}
	name := buildRawFilename(when, m)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, m.Raw, 0o600); err != nil {
		return "", errtrace.Wrapf(err, "write eml %s", path)
	}
	return path, nil
}

// resolveSaveDir returns the day-folder under email/<alias>/YYYY-MM-DD/,
// creating it if needed, plus the resolved timestamp used for the filename.
func resolveSaveDir(alias string, m *Message) (string, time.Time, error) {
	root, err := config.EmailDir()
	if err != nil {
		return "", time.Time{}, errtrace.Wrap(err, "email dir")
	}
	when := m.ReceivedAt
	if when.IsZero() {
		when = time.Now()
	}
	dir := filepath.Join(root, sanitize(alias), when.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", time.Time{}, errtrace.Wrap(err, "mkdir email dir")
	}
	return dir, when, nil
}

// buildRawFilename composes the readable .eml filename:
// "HH.MM.SS__from__subject__uidN.eml" with sender/subject sanitized & truncated.
func buildRawFilename(when time.Time, m *Message) string {
	timePrefix := when.Format("15.04.05")
	fromPart := sanitizeReadable(extractEmailAddr(m.From))
	if fromPart == "" {
		fromPart = "unknown-sender"
	}
	if len(fromPart) > 60 {
		fromPart = fromPart[:60]
	}
	subjPart := sanitizeReadable(m.Subject)
	if subjPart == "" {
		subjPart = "no-subject"
	}
	if len(subjPart) > 80 {
		subjPart = subjPart[:80]
	}
	return fmt.Sprintf("%s__%s__%s__uid%d.eml", timePrefix, fromPart, subjPart, m.Uid)
}

// --- helpers ---------------------------------------------------------------

var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._@-]+`)

// sanitize is the strict path-safe sanitizer used for alias/folder names.
func sanitize(s string) string {
	s = unsafePathChars.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// readableUnsafeChars matches anything that's not a lowercase letter, digit,
// or hyphen. Used to convert "Re: Check!" -> "re-check" for filenames.
var readableUnsafeChars = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeReadable produces a lowercase, hyphen-separated, filesystem-safe
// fragment suitable for human-readable filenames. Examples:
//
//	"Re: Check"                              -> "re-check"
//	"abdullah.mahin.rasia@gmail.com"         -> "abdullah-mahin-rasia-gmail-com"
//	`"Abdullah Al Mahin" <a.b@c.com>`        -> "abdullah-al-mahin-a-b-c-com"
func sanitizeReadable(s string) string {
	s = strings.ToLower(s)
	s = readableUnsafeChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// extractEmailAddr pulls "addr@host" out of `"Display" <addr@host>` if
// present, otherwise returns the input unchanged. Keeps filenames focused on
// the actual mailbox rather than the (often long) display name.
func extractEmailAddr(s string) string {
	if i := strings.IndexByte(s, '<'); i >= 0 {
		if j := strings.IndexByte(s[i:], '>'); j > 0 {
			return s[i+1 : i+j]
		}
	}
	return s
}

func formatAddr(a *imap.Address) string {
	if a == nil {
		return ""
	}
	if a.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", a.PersonalName, a.MailboxName, a.HostName)
	}
	return fmt.Sprintf("%s@%s", a.MailboxName, a.HostName)
}

// parseRaw walks the MIME tree and extracts text/html bodies plus headers.
func parseRaw(raw []byte) (*Message, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		// Not a multipart MIME message — try as a flat message/rfc822.
		ent, err2 := message.Read(bytes.NewReader(raw))
		if err2 != nil {
			return nil, err
		}
		out := &Message{}
		fillHeaderFlat(ent, out)
		readEntityBody(ent, out)
		return out, nil
	}
	defer mr.Close()

	out := &Message{}
	fillHeader(mr.Header, out)

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)
			if strings.EqualFold(ct, "text/html") {
				if out.BodyHtml == "" {
					out.BodyHtml = string(b)
				}
			} else {
				if out.BodyText == "" {
					out.BodyText = string(b)
				}
			}
		case *mail.AttachmentHeader:
			// Drain & ignore attachments.
			_, _ = io.Copy(io.Discard, p.Body)
		}
	}
	if out.BodyText == "" && out.BodyHtml != "" {
		out.BodyText = stripHtml(out.BodyHtml)
	}
	return out, nil
}

func fillHeader(h mail.Header, out *Message) {
	if v, err := h.MessageID(); err == nil && v != "" {
		out.MessageId = "<" + v + ">"
	}
	if v, err := h.Subject(); err == nil {
		out.Subject = v
	}
	if v, err := h.Date(); err == nil {
		out.ReceivedAt = v
	}
	if al, err := h.AddressList("From"); err == nil && len(al) > 0 {
		out.From = al[0].String()
	}
	if al, err := h.AddressList("To"); err == nil {
		out.To = joinAddrs(al)
	}
	if al, err := h.AddressList("Cc"); err == nil {
		out.Cc = joinAddrs(al)
	}
}

func fillHeaderFlat(ent *message.Entity, out *Message) {
	h := ent.Header
	out.MessageId = h.Get("Message-Id")
	out.Subject = h.Get("Subject")
	out.From = h.Get("From")
	out.To = h.Get("To")
	out.Cc = h.Get("Cc")
	if d := h.Get("Date"); d != "" {
		if t, err := netmail.ParseDate(d); err == nil {
			out.ReceivedAt = t
		}
	}
}

func readEntityBody(ent *message.Entity, out *Message) {
	ct, _, _ := ent.Header.ContentType()
	b, _ := io.ReadAll(ent.Body)
	if strings.EqualFold(ct, "text/html") {
		out.BodyHtml = string(b)
		out.BodyText = stripHtml(out.BodyHtml)
	} else {
		out.BodyText = string(b)
	}
}

func joinAddrs(al []*mail.Address) string {
	parts := make([]string, 0, len(al))
	for _, a := range al {
		parts = append(parts, a.String())
	}
	return strings.Join(parts, ", ")
}

var htmlTagRe = regexp.MustCompile(`(?s)<[^>]+>`)
var htmlSpaceRe = regexp.MustCompile(`\s+`)

func stripHtml(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = htmlSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
