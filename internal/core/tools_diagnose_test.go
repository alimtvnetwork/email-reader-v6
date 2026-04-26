// tools_diagnose_test.go covers the 60 s diagnose cache + the small
// helpers around RecentOpenedUrls validation.
package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// stubClock returns the configured time so cache TTL is deterministic.
type stubClock struct{ now time.Time }

func (c *stubClock) Now() time.Time { return c.now }

func TestCachedDiagnose_MissThenHitReplaysEvents(t *testing.T) {
	cache := newDiagnoseCache()
	clock := &stubClock{now: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)}
	calls := 0
	diag := func(_ string, emit func(DiagnoseEvent)) errtrace.Result[struct{}] {
		calls++
		emit(DiagnoseEvent{Kind: DiagnoseEventStart, Message: "start"})
		emit(DiagnoseEvent{Kind: DiagnoseEventLoginOK, Message: "login"})
		emit(DiagnoseEvent{Kind: DiagnoseEventSummary, Message: "ok"})
		return errtrace.Ok(struct{}{})
	}
	var live []DiagnoseEvent
	r1 := cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "work"},
		func(ev DiagnoseEvent) { live = append(live, ev) }, diag, clock.Now)
	if r1.HasError() || calls != 1 || len(live) != 3 || r1.Value().Cached {
		t.Fatalf("first run unexpected: err=%v calls=%d events=%d cached=%v", r1.Error(), calls, len(live), r1.Value().Cached)
	}
	clock.now = clock.now.Add(30 * time.Second) // within TTL
	var replayed []DiagnoseEvent
	r2 := cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "work"},
		func(ev DiagnoseEvent) { replayed = append(replayed, ev) }, diag, clock.Now)
	if r2.HasError() || calls != 1 || len(replayed) != 3 || !r2.Value().Cached {
		t.Fatalf("cache hit failed: calls=%d replayed=%d cached=%v err=%v", calls, len(replayed), r2.Value().Cached, r2.Error())
	}
}

func TestCachedDiagnose_TtlExpiryRefetches(t *testing.T) {
	cache := newDiagnoseCache()
	clock := &stubClock{now: time.Now()}
	calls := 0
	diag := func(_ string, _ func(DiagnoseEvent)) errtrace.Result[struct{}] {
		calls++
		return errtrace.Ok(struct{}{})
	}
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "x"}, nil, diag, clock.Now)
	clock.now = clock.now.Add(61 * time.Second) // past TTL
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "x"}, nil, diag, clock.Now)
	if calls != 2 {
		t.Fatalf("expected 2 live runs after TTL expiry, got %d", calls)
	}
}

func TestCachedDiagnose_ForceBypassesCache(t *testing.T) {
	cache := newDiagnoseCache()
	clock := &stubClock{now: time.Now()}
	calls := 0
	diag := func(_ string, _ func(DiagnoseEvent)) errtrace.Result[struct{}] {
		calls++
		return errtrace.Ok(struct{}{})
	}
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "y"}, nil, diag, clock.Now)
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "y", Force: true}, nil, diag, clock.Now)
	if calls != 2 {
		t.Fatalf("Force must bypass cache; calls=%d", calls)
	}
}

func TestCachedDiagnose_WrapsBackendError(t *testing.T) {
	cache := newDiagnoseCache()
	diag := func(_ string, _ func(DiagnoseEvent)) errtrace.Result[struct{}] {
		return errtrace.Err[struct{}](errors.New("imap blew up"))
	}
	r := cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "z"}, nil, diag, time.Now)
	if !r.HasError() {
		t.Fatal("expected error")
	}
	var coded *errtrace.Coded
	if !errors.As(r.Error(), &coded) || coded.Code != errtrace.ErrToolsDiagnoseAborted {
		t.Fatalf("expected ErrToolsDiagnoseAborted, got %v", r.Error())
	}
}

func TestCachedDiagnose_CtxCancelled(t *testing.T) {
	cache := newDiagnoseCache()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := cachedDiagnoseWith(ctx, cache, DiagnoseSpec{}, nil,
		func(_ string, _ func(DiagnoseEvent)) errtrace.Result[struct{}] { return errtrace.Ok(struct{}{}) },
		time.Now)
	if !r.HasError() {
		t.Fatal("expected ctx error")
	}
}

func TestCachedDiagnose_FailureNotCached(t *testing.T) {
	cache := newDiagnoseCache()
	calls := 0
	diag := func(_ string, _ func(DiagnoseEvent)) errtrace.Result[struct{}] {
		calls++
		if calls == 1 {
			return errtrace.Err[struct{}](errors.New("boom"))
		}
		return errtrace.Ok(struct{}{})
	}
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "f"}, nil, diag, time.Now)
	_ = cachedDiagnoseWith(context.Background(), cache, DiagnoseSpec{Alias: "f"}, nil, diag, time.Now)
	if calls != 2 {
		t.Fatalf("failures must NOT be cached; calls=%d", calls)
	}
}

func TestValidateOpenedUrlListSpec(t *testing.T) {
	spec := OpenedUrlListSpec{}
	if err := validateOpenedUrlListSpec(&spec); err != nil {
		t.Fatalf("defaults must pass: %v", err)
	}
	if spec.Limit != 100 || spec.Before.IsZero() {
		t.Fatalf("defaults not applied: %+v", spec)
	}
	for _, lim := range []int{-1, 0, 1001, 99999} {
		s := OpenedUrlListSpec{Limit: lim}
		if lim == 0 {
			continue // 0 is the default-trigger, not an error
		}
		err := validateOpenedUrlListSpec(&s)
		if err == nil {
			t.Fatalf("Limit=%d should fail", lim)
		}
		var coded *errtrace.Coded
		if !errors.As(err, &coded) || coded.Code != errtrace.ErrToolsInvalidArgument {
			t.Fatalf("Limit=%d: expected ErrToolsInvalidArgument, got %v", lim, err)
		}
	}
}

// fakeRows satisfies the inline interface scanOpenedUrlRows expects.
type fakeRows struct {
	rows  [][]any
	idx   int
	scan  func(dest []any, src []any) error
	final error
}

func (f *fakeRows) Next() bool { f.idx++; return f.idx <= len(f.rows) }
func (f *fakeRows) Scan(dest ...any) error {
	src := f.rows[f.idx-1]
	if f.scan != nil {
		return f.scan(dest, src)
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *int64:
			*d = src[i].(int64)
		case *string:
			*d = src[i].(string)
		case *time.Time:
			*d = src[i].(time.Time)
		}
	}
	return nil
}
func (f *fakeRows) Err() error { return f.final }

func TestScanOpenedUrlRows_HappyPath(t *testing.T) {
	now := time.Now()
	rows := &fakeRows{rows: [][]any{
		{int64(1), int64(10), "rule-A", "https://x.test/1", now},
		{int64(2), int64(11), "", "https://y.test/2", now.Add(-time.Hour)},
	}}
	out, err := scanOpenedUrlRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Url != "https://x.test/1" || out[1].RuleName != "" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestScanOpenedUrlRows_ScanError(t *testing.T) {
	rows := &fakeRows{
		rows: [][]any{{nil}},
		scan: func(_, _ []any) error { return errors.New("scan boom") },
	}
	if _, err := scanOpenedUrlRows(rows); err == nil {
		t.Fatal("expected scan error")
	}
}
