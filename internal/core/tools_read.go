// tools_read.go implements the ReadOnce sub-tool: a one-shot IMAP probe
// that fetches the most-recent N headers without persisting anything or
// advancing the watcher cursor. Streams human-readable progress lines on
// the caller-supplied channel; closes it exactly once on return.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.1.
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
)

// ReadSpec is the request shape for ReadOnce.
type ReadSpec struct {
	Alias string // empty → first configured account
	Limit int    // 1..500; default 10
}

// ReadResult summarises the fetch — Headers is exposed for callers that
// want to render a table; Duration powers the progress trail.
type ReadResult struct {
	Alias    string
	Headers  []mailclient.HeaderSummary
	Duration time.Duration
}

// ReadOnce dials, logs in, selects INBOX, fetches `Limit` headers, and
// closes `progress` on return. Watcher cursor is never written.
func (t *Tools) ReadOnce(ctx context.Context, spec ReadSpec, progress chan<- string) errtrace.Result[ReadResult] {
	defer closeProgress(progress)
	limit, err := normalizeReadSpec(spec)
	if err != nil {
		return errtrace.Err[ReadResult](err)
	}
	acct, err := resolveAccount(spec.Alias)
	if err != nil {
		return errtrace.Err[ReadResult](err)
	}
	send(progress, fmt.Sprintf("dialing %s:%d as %s…", acct.Server, acct.Port, acct.User))
	if err := ctxCheck(ctx); err != nil {
		return errtrace.Err[ReadResult](err)
	}
	return runReadOnce(ctx, acct, limit, progress)
}

// runReadOnce is the IMAP-touching half — split from OpenUrl so the
// public method body stays ≤ 15 LOC per coding-standards §3.
func runReadOnce(ctx context.Context, acct any, limit int, progress chan<- string) errtrace.Result[ReadResult] {
	started := time.Now()
	cli, err := dialAndLogin(acct, progress)
	if err != nil {
		return errtrace.Err[ReadResult](errtrace.WrapCode(err, errtrace.ErrToolsReadFetchFailed, "dial/login"))
	}
	defer cli.Close()
	if err := ctxCheck(ctx); err != nil {
		return errtrace.Err[ReadResult](err)
	}
	hdrs, err := selectAndFetch(cli, uint32(limit), progress)
	if err != nil {
		return errtrace.Err[ReadResult](errtrace.WrapCode(err, errtrace.ErrToolsReadFetchFailed, "select/fetch"))
	}
	dur := time.Since(started)
	send(progress, fmt.Sprintf("fetched %d header(s) in %s", len(hdrs), dur.Round(time.Millisecond)))
	return errtrace.Ok(ReadResult{Headers: hdrs, Duration: dur})
}

func normalizeReadSpec(spec ReadSpec) (int, error) {
	limit := spec.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 500 {
		return 0, errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "Limit out of [1,500]")
	}
	return limit, nil
}

func resolveAccount(alias string) (mailclient.Account, error) {
	if alias == "" {
		r := ListAccounts()
		if r.HasError() {
			return mailclient.Account{}, r.Error()
		}
		if len(r.Value()) == 0 {
			return mailclient.Account{}, errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "no accounts configured")
		}
		return toMailAccount(r.Value()[0]), nil
	}
	r := GetAccount(alias)
	if r.HasError() {
		return mailclient.Account{}, r.Error()
	}
	return toMailAccount(r.Value()), nil
}

func dialAndLogin(acct any, progress chan<- string) (readCloser, error) {
	a, ok := acct.(mailclient.Account)
	if !ok {
		return nil, errtrace.New("internal: account type mismatch")
	}
	cli, err := mailclient.Dial(a.toConfig())
	if err != nil {
		return nil, err
	}
	send(progress, "dial OK; login OK")
	return cli, nil
}

func selectAndFetch(cli readCloser, limit uint32, progress chan<- string) ([]mailclient.HeaderSummary, error) {
	stats, err := cli.SelectInbox()
	if err != nil {
		return nil, err
	}
	send(progress, fmt.Sprintf("INBOX selected: %d messages, UidNext=%d", stats.Messages, stats.UidNext))
	hdrs, err := cli.FetchRecentHeaders(stats, limit)
	if err != nil {
		return nil, err
	}
	return hdrs, nil
}

// readCloser is the slim interface the read flow needs from mailclient.
type readCloser interface {
	SelectInbox() (mailclient.MailboxStats, error)
	FetchRecentHeaders(stats mailclient.MailboxStats, limit uint32) ([]mailclient.HeaderSummary, error)
	Close() error
}

func ctxCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errtrace.WrapCode(ctx.Err(), errtrace.ErrCoreContextCancelled, "ReadOnce cancelled")
	default:
		return nil
	}
}

func send(ch chan<- string, line string) {
	if ch == nil {
		return
	}
	select {
	case ch <- line:
	default:
	}
}

func closeProgress(ch chan<- string) {
	defer func() { _ = recover() }() // tolerate caller-closed channel
	if ch != nil {
		close(ch)
	}
}
