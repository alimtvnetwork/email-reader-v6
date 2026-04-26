// tools_read_test.go covers ReadOnce: validation, dial seam, progress
// streaming, channel close, and context cancellation.
package core

import (
	"context"
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

type fakeReadClient struct {
	stats   mailclient.MailboxStats
	hdrs    []mailclient.HeaderSummary
	selErr  error
	fetchErr error
	closed  bool
}

func (f *fakeReadClient) SelectInbox() (mailclient.MailboxStats, error) {
	if f.selErr != nil {
		return mailclient.MailboxStats{}, f.selErr
	}
	return f.stats, nil
}

func (f *fakeReadClient) FetchRecentHeaders(_ mailclient.MailboxStats, _ uint32) ([]mailclient.HeaderSummary, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.hdrs, nil
}

func (f *fakeReadClient) Close() error { f.closed = true; return nil }

func mustToolsForRead(t *testing.T) *Tools {
	t.Helper()
	r := NewTools(&fakeBrowser{}, newFakeStore(), DefaultToolsConfig())
	if r.HasError() {
		t.Fatalf("NewTools: %v", r.Error())
	}
	return r.Value()
}

func okResolver(_ string) (config.Account, string, error) {
	return config.Account{Alias: "work", Email: "w@x.test", ImapHost: "imap.x.test", ImapPort: 993}, "work", nil
}

func TestReadOnce_HappyPath(t *testing.T) {
	cli := &fakeReadClient{
		stats: mailclient.MailboxStats{Messages: 42, UidNext: 100},
		hdrs:  []mailclient.HeaderSummary{{Uid: 99, Subject: "hi"}, {Uid: 98, Subject: "yo"}},
	}
	dial := func(_ config.Account) (readClient, error) { return cli, nil }
	progress := make(chan string, 8)
	r := readOnceWith(context.Background(), ReadSpec{Limit: 2}, progress, dial, okResolver)
	if r.HasError() {
		t.Fatalf("ReadOnce: %v", r.Error())
	}
	if got := r.Value(); len(got.Headers) != 2 || got.Alias != "work" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !cli.closed {
		t.Fatal("client must be closed")
	}
	// drain channel — should be closed
	count := 0
	for range progress {
		count++
	}
	if count < 3 { // dialing… + dial OK + INBOX selected + fetched
		t.Fatalf("expected ≥3 progress lines, got %d", count)
	}
}

func TestReadOnce_LimitValidation(t *testing.T) {
	dial := func(_ config.Account) (readClient, error) { return &fakeReadClient{}, nil }
	for _, lim := range []int{-1, 501, 9999} {
		r := readOnceWith(context.Background(), ReadSpec{Limit: lim}, nil, dial, okResolver)
		if !r.HasError() {
			t.Fatalf("Limit=%d should fail", lim)
		}
		var coded *errtrace.Coded
		if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrToolsInvalidArgument {
			t.Fatalf("Limit=%d: expected ErrToolsInvalidArgument, got %v", lim, r.Error())
		}
	}
}

func TestReadOnce_DialFailureWraps21751(t *testing.T) {
	dial := func(_ config.Account) (readClient, error) { return nil, errors.New("boom") }
	r := readOnceWith(context.Background(), ReadSpec{Limit: 5}, nil, dial, okResolver)
	if !r.HasError() {
		t.Fatal("expected error")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrToolsReadFetchFailed {
		t.Fatalf("expected ErrToolsReadFetchFailed, got %v", r.Error())
	}
}

func TestReadOnce_ContextCancelled(t *testing.T) {
	dial := func(_ config.Account) (readClient, error) { return &fakeReadClient{}, nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := readOnceWith(ctx, ReadSpec{Limit: 5}, nil, dial, okResolver)
	if !r.HasError() {
		t.Fatal("expected error on cancelled ctx")
	}
}

func TestReadOnce_ProgressChannelClosedOnError(t *testing.T) {
	progress := make(chan string, 4)
	r := readOnceWith(context.Background(), ReadSpec{Limit: -1}, progress, nil, okResolver)
	if !r.HasError() {
		t.Fatal("expected error")
	}
	// channel must be closed even on early-validation failure
	for range progress {
	}
}

func TestNormalizeReadSpec_DefaultsTo10(t *testing.T) {
	if n, err := normalizeReadSpec(ReadSpec{}); err != nil || n != 10 {
		t.Fatalf("expected 10, got %d / %v", n, err)
	}
}

// silence the unused-import linter in the rare configs without test usage.
var _ = mustToolsForRead
