// Package mailclient connects to an IMAP server, fetches new messages by UID,
// parses them into a normalized struct, and saves the raw .eml to disk under
// email/<alias>/<YYYY-MM-DD>/<safe-message-id>.eml.
package mailclient

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // register extra charsets
	"github.com/emersion/go-message/mail"

	"github.com/lovable/email-read/internal/config"
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

// Dial opens an IMAP connection and logs in.
func Dial(acct config.Account) (*Client, error) {
	addr := net.JoinHostPort(acct.ImapHost, fmt.Sprintf("%d", acct.ImapPort))
	var (
		c   *client.Client
		err error
	)
	if acct.UseTLS {
		c, err = client.DialTLS(addr, &tls.Config{ServerName: acct.ImapHost})
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		return nil, fmt.Errorf("imap dial %s: %w", addr, err)
	}
	pwd, err := config.DecodePassword(acct.PasswordB64)
	if err != nil {
		_ = c.Logout()
		return nil, err
	}
	if err := c.Login(acct.Email, pwd); err != nil {
		_ = c.Logout()
		return nil, fmt.Errorf("imap login %s: %w", acct.Email, err)
	}
	return &Client{acct: acct, c: c}, nil
}

// Close logs out and closes the connection.
func (c *Client) Close() error {
	if c == nil || c.c == nil {
		return nil
	}
	return c.c.Logout()
}

// SelectInbox selects the configured mailbox (defaults to INBOX) and returns
// the highest UID currently present (UIDNEXT-1) — useful for first-run baselines.
func (c *Client) SelectInbox() (uidNext uint32, err error) {
	box := c.acct.Mailbox
	if box == "" {
		box = "INBOX"
	}
	mbox, err := c.c.Select(box, false)
	if err != nil {
		return 0, fmt.Errorf("select %s: %w", box, err)
	}
	return mbox.UidNext, nil
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
		return nil, fmt.Errorf("uid fetch: %w", err)
	}
	return out, nil
}

// SaveRaw writes the raw .eml file under email/<alias>/<YYYY-MM-DD>/<safe-id>.eml
// and returns the absolute path written.
func SaveRaw(alias string, m *Message) (string, error) {
	root, err := config.EmailDir()
	if err != nil {
		return "", err
	}
	when := m.ReceivedAt
	if when.IsZero() {
		when = time.Now()
	}
	dir := filepath.Join(root, sanitize(alias), when.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir email dir: %w", err)
	}
	name := sanitize(strings.Trim(m.MessageId, "<>"))
	if name == "" {
		name = fmt.Sprintf("uid-%d", m.Uid)
	}
	if len(name) > 120 {
		name = name[:120]
	}
	path := filepath.Join(dir, name+".eml")
	if err := os.WriteFile(path, m.Raw, 0o600); err != nil {
		return "", fmt.Errorf("write eml: %w", err)
	}
	return path, nil
}

// --- helpers ---------------------------------------------------------------

var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._@-]+`)

func sanitize(s string) string {
	s = unsafePathChars.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
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
		if t, err := mail.ParseDate(d); err == nil {
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
