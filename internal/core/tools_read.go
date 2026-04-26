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

	"github.com/lovable/email-read/internal/config"
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

// readDialer is the slim seam for tests; *mailclient.Client satisfies it.
type readDialer func(config.Account) (readClient, error)

type readClient interface {
	SelectInbox() (mailclient.MailboxStats, error)
	FetchRecentHeaders(stats mailclient.MailboxStats, limit uint32) ([]mailclient.HeaderSummary, error)
	Close() error
}

// defaultReadDialer wraps mailclient.Dial into the readDialer signature.
func defaultReadDialer(a config.Account) (readClient, error) { return mailclient.Dial(a) }

// ReadOnce dials, logs in, selects INBOX, fetches `Limit` headers, and
// closes `progress` on return. Watcher cursor is never written.
func (t *Tools) ReadOnce(ctx context.Context, spec ReadSpec, progress chan<- string) errtrace.Result[ReadResult] {
	return readOnceWith(ctx, spec, progress, defaultReadDialer, resolveAccountByAlias)
}

// readOnceWith is the testable inner that takes injected seams.
func readOnceWith(ctx context.Context, spec ReadSpec, progress chan<- string,
	dial readDialer, resolve func(string) (config.Account, string, error),
) errtrace.Result[ReadResult] {
	defer closeProgress(progress)
	limit, err := normalizeReadSpec(spec)
	if err != nil {
		return errtrace.Err[ReadResult](err)
	}
	acct, alias, err := resolve(spec.Alias)
	if err != nil {
		return errtrace.Err[ReadResult](err)
	}
	send(progress, fmt.Sprintf("dialing %s:%d as %s…", acct.ImapHost, acct.ImapPort, acct.Email))
	if err := ctxCheck(ctx); err != nil {
		return errtrace.Err[ReadResult](err)
	}
	return runReadOnce(ctx, alias, acct, uint32(limit), progress, dial)
}

// runReadOnce performs the IMAP-touching half — split so the public
// method body stays ≤ 15 LOC per coding-standards §3.
func runReadOnce(ctx context.Context, alias string, acct config.Account, limit uint32,
	progress chan<- string, dial readDialer,
) errtrace.Result[ReadResult] {
	started := time.Now()
	cli, err := dial(acct)
	if err != nil {
		return errtrace.Err[ReadResult](errtrace.WrapCode(err, errtrace.ErrToolsReadFetchFailed, "dial"))
	}
	defer cli.Close()
	send(progress, "dial OK; login OK")
	if err := ctxCheck(ctx); err != nil {
		return errtrace.Err[ReadResult](err)
	}
	hdrs, err := selectAndFetch(cli, limit, progress)
	if err != nil {
		return errtrace.Err[ReadResult](errtrace.WrapCode(err, errtrace.ErrToolsReadFetchFailed, "select/fetch"))
	}
	dur := time.Since(started)
	send(progress, fmt.Sprintf("fetched %d header(s) in %s", len(hdrs), dur.Round(time.Millisecond)))
	return errtrace.Ok(ReadResult{Alias: alias, Headers: hdrs, Duration: dur})
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

// resolveAccountByAlias is the production resolver: empty alias → first
// configured account; non-empty → looked up via core.GetAccount.
func resolveAccountByAlias(alias string) (config.Account, string, error) {
	if alias == "" {
		r := ListAccounts()
		if r.HasError() {
			return config.Account{}, "", r.Error()
		}
		if len(r.Value()) == 0 {
			return config.Account{}, "", errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "no accounts configured")
		}
		first := r.Value()[0]
		return first, first.Alias, nil
	}
	r := GetAccount(alias)
	if r.HasError() {
		return config.Account{}, "", r.Error()
	}
	return r.Value(), alias, nil
}

func selectAndFetch(cli readClient, limit uint32, progress chan<- string) ([]mailclient.HeaderSummary, error) {
	stats, err := cli.SelectInbox()
	if err != nil {
		return nil, err
	}
	send(progress, fmt.Sprintf("INBOX selected: %d messages, UidNext=%d", stats.Messages, stats.UidNext))
	return cli.FetchRecentHeaders(stats, limit)
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
