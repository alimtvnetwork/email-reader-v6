package core

import (
	"context"
	"strings"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// EmailSummary is a UI-friendly projection of a stored email row. The body
// fields are kept separate so the UI can render a list cheaply (subject + from
// + date) and only render the body when the user opens an email.
type EmailSummary struct {
	Id         int64
	Alias      string
	Uid        uint32
	From       string
	Subject    string
	ReceivedAt string // formatted RFC3339 in UTC; empty when not stored
	Snippet    string // first ~140 chars of BodyText (or BodyHtml fallback)
	FilePath   string
}

// EmailDetail extends EmailSummary with the full body fields needed by the
// detail pane.
type EmailDetail struct {
	EmailSummary
	To       string
	Cc       string
	BodyText string
	BodyHtml string
}

// ListEmailsOptions mirrors store.EmailQuery but is exported for UI use
// without leaking the store package into views.
type ListEmailsOptions struct {
	Alias  string
	Search string
	Limit  int
	Offset int
}

// ListEmails returns email summaries from the store. Opens and closes the
// store on each call — fine for UI list refreshes which are infrequent.
func ListEmails(ctx context.Context, opts ListEmailsOptions) ([]EmailSummary, error) {
	st, err := store.Open()
	if err != nil {
		return nil, errtrace.Wrap(err, "open store")
	}
	defer st.Close()
	rows, err := st.ListEmails(ctx, store.EmailQuery{
		Alias:  opts.Alias,
		Search: opts.Search,
		Limit:  opts.Limit,
		Offset: opts.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]EmailSummary, 0, len(rows))
	for _, e := range rows {
		out = append(out, toSummary(e))
	}
	return out, nil
}

// GetEmail returns the full detail for one stored email identified by
// (alias, uid). Returns an error if no such row exists.
func GetEmail(ctx context.Context, alias string, uid uint32) (*EmailDetail, error) {
	st, err := store.Open()
	if err != nil {
		return nil, errtrace.Wrap(err, "open store")
	}
	defer st.Close()
	e, err := st.GetEmailByUid(ctx, alias, uid)
	if err != nil {
		return nil, err
	}
	d := EmailDetail{
		EmailSummary: toSummary(*e),
		To:           e.ToAddr,
		Cc:           e.CcAddr,
		BodyText:     e.BodyText,
		BodyHtml:     e.BodyHtml,
	}
	return &d, nil
}

// CountEmails returns how many emails are stored for the given alias
// (empty alias = total across all accounts).
func CountEmails(ctx context.Context, alias string) (int, error) {
	st, err := store.Open()
	if err != nil {
		return 0, errtrace.Wrap(err, "open store")
	}
	defer st.Close()
	return st.CountEmails(ctx, alias)
}

func toSummary(e store.Email) EmailSummary {
	s := EmailSummary{
		Id:       e.Id,
		Alias:    e.Alias,
		Uid:      e.Uid,
		From:     e.FromAddr,
		Subject:  e.Subject,
		FilePath: e.FilePath,
		Snippet:  snippet(e.BodyText, e.BodyHtml),
	}
	if !e.ReceivedAt.IsZero() {
		s.ReceivedAt = e.ReceivedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return s
}

// snippet returns a short single-line preview of the body. Prefers plain text;
// falls back to HTML with tags stripped naively. Capped at ~140 chars.
func snippet(text, html string) string {
	src := text
	if strings.TrimSpace(src) == "" {
		src = stripTags(html)
	}
	src = strings.Join(strings.Fields(src), " ") // collapse whitespace
	if len(src) > 140 {
		src = src[:139] + "…"
	}
	return src
}

func stripTags(s string) string {
	var b strings.Builder
	skip := false
	for _, r := range s {
		switch {
		case r == '<':
			skip = true
		case r == '>':
			skip = false
		case !skip:
			b.WriteRune(r)
		}
	}
	return b.String()
}
