// tools_diagnose.go adds two services on top of the existing
// `core.Diagnose` backend:
//
//  1. `Tools.Diagnose(ctx, spec, emit)` — wraps the global Diagnose with
//     a 60 s in-memory cache (per `Tools` instance, per alias). On cache
//     hit, the captured event trail is replayed to `emit` so the UI
//     renders identically to a live run. `Force: true` bypasses + evicts.
//
//  2. `Tools.RecentOpenedUrls(ctx, spec)` — read-only audit accessor
//     backed by the OpenedUrls table. Delta #1 (PascalCase migration)
//     activated the rich schema (Alias / Origin / OriginalUrl /
//     IsDeduped / IsIncognito / TraceId); the Alias and Origin filter
//     args are now honoured by `buildOpenedUrlsQuery`.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.3 + §2.5.
package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// DiagnoseSpec controls the cached diagnose run.
type DiagnoseSpec struct {
	Alias string
	Force bool // bypass + evict cache entry
}

// DiagnosticsReport summarises a cached diagnose run.
type DiagnosticsReport struct {
	Alias    string
	Cached   bool
	StoredAt time.Time
	Events   []DiagnoseEvent
}

// diagCacheEntry is one cached run pinned to its capture time.
type diagCacheEntry struct {
	report DiagnosticsReport
}

// diagnoseCache is the per-Tools 60 s cache. Keys are aliases (empty
// string is a valid key for "first configured account" runs).
type diagnoseCache struct {
	mu      sync.Mutex
	entries map[string]diagCacheEntry
	ttl     time.Duration
}

func newDiagnoseCache() *diagnoseCache {
	return &diagnoseCache{entries: map[string]diagCacheEntry{}, ttl: 60 * time.Second}
}

func (c *diagnoseCache) get(key string, now time.Time) (DiagnosticsReport, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return DiagnosticsReport{}, false
	}
	if now.Sub(e.report.StoredAt) > c.ttl {
		delete(c.entries, key)
		return DiagnosticsReport{}, false
	}
	return e.report, true
}

func (c *diagnoseCache) put(key string, r DiagnosticsReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = diagCacheEntry{report: r}
}

func (c *diagnoseCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// diagnoseFn is the seam for tests. Production = `core.Diagnose`.
type diagnoseFn func(alias string, emit func(DiagnoseEvent)) errtrace.Result[struct{}]

// CachedDiagnose runs Diagnose with the 60 s cache. emit may be nil.
// On cache hit, every captured event is replayed to emit before return.
func (t *Tools) CachedDiagnose(ctx context.Context, spec DiagnoseSpec, emit func(DiagnoseEvent)) errtrace.Result[DiagnosticsReport] {
	return cachedDiagnoseWith(ctx, t.diagCache(), spec, emit, Diagnose, time.Now)
}

// diagCache lazily allocates the cache on first use so existing callers
// of NewTools don't need to thread the cache field through construction.
func (t *Tools) diagCache() *diagnoseCache {
	t.diagCacheOnce.Do(func() { t.diagCachePtr = newDiagnoseCache() })
	return t.diagCachePtr
}

// cachedDiagnoseWith is the testable inner — caller injects diagnoseFn + clock.
func cachedDiagnoseWith(ctx context.Context, cache *diagnoseCache, spec DiagnoseSpec,
	emit func(DiagnoseEvent), diagnose diagnoseFn, now func() time.Time,
) errtrace.Result[DiagnosticsReport] {
	if err := ctxCheck(ctx); err != nil {
		return errtrace.Err[DiagnosticsReport](err)
	}
	if !spec.Force {
		if r, hit := cache.get(spec.Alias, now()); hit {
			replayEvents(r.Events, emit)
			r.Cached = true
			return errtrace.Ok(r)
		}
	}
	cache.invalidate(spec.Alias)
	return runAndCacheDiagnose(spec, emit, diagnose, cache, now)
}

func runAndCacheDiagnose(spec DiagnoseSpec, emit func(DiagnoseEvent),
	diagnose diagnoseFn, cache *diagnoseCache, now func() time.Time,
) errtrace.Result[DiagnosticsReport] {
	captured := make([]DiagnoseEvent, 0, 8)
	wrap := func(ev DiagnoseEvent) {
		captured = append(captured, ev)
		if emit != nil {
			emit(ev)
		}
	}
	res := diagnose(spec.Alias, wrap)
	if res.HasError() {
		return errtrace.Err[DiagnosticsReport](errtrace.WrapCode(res.Error(), errtrace.ErrToolsDiagnoseAborted, "Diagnose"))
	}
	rep := DiagnosticsReport{Alias: spec.Alias, StoredAt: now(), Events: captured}
	cache.put(spec.Alias, rep)
	return errtrace.Ok(rep)
}

func replayEvents(evs []DiagnoseEvent, emit func(DiagnoseEvent)) {
	if emit == nil {
		return
	}
	for _, ev := range evs {
		emit(ev)
	}
}

// ----- RecentOpenedUrls ---------------------------------------------------

// OpenedUrlListSpec filters the audit-list query. Alias / Origin became
// active filters in Delta #1 (PascalCase OpenedUrls migration).
type OpenedUrlListSpec struct {
	Alias  string        // empty → all aliases
	Origin OpenUrlOrigin // empty → all origins
	Limit  int           // 1..1000; default 100
	Before time.Time     // pagination cursor; zero = now
}

// OpenedUrlRow is one historical launch. Fields populated by Delta #1
// are zero-valued for legacy rows that predate the migration.
type OpenedUrlRow struct {
	Id          int64
	EmailId     int64
	Alias       string
	RuleName    string
	Origin      OpenUrlOrigin
	Url         string
	OriginalUrl string
	IsDeduped   bool
	IsIncognito bool
	TraceId     string
	OpenedAt    time.Time
}

// RecentOpenedUrls returns the most-recent audit rows. Honours the
// Alias and Origin filters activated by Delta #1.
func (t *Tools) RecentOpenedUrls(ctx context.Context, spec OpenedUrlListSpec) errtrace.Result[[]OpenedUrlRow] {
	if err := validateOpenedUrlListSpec(&spec); err != nil {
		return errtrace.Err[[]OpenedUrlRow](err)
	}
	st, err := openExportStore() // shared helper from tools_export.go
	if err != nil {
		return errtrace.Err[[]OpenedUrlRow](err)
	}
	defer st.Close()
	return queryOpenedUrls(ctx, st, spec)
}

func validateOpenedUrlListSpec(spec *OpenedUrlListSpec) error {
	if spec.Limit == 0 {
		spec.Limit = 100
	}
	if spec.Limit < 1 || spec.Limit > 1000 {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "Limit out of [1,1000]")
	}
	if spec.Before.IsZero() {
		spec.Before = time.Now()
	}
	if spec.Origin != "" && spec.Origin != OriginManual && spec.Origin != OriginRule && spec.Origin != OriginCli {
		return errtrace.NewCoded(errtrace.ErrToolsInvalidArgument, "Origin must be empty or one of {manual,rule,cli}")
	}
	return nil
}

func queryOpenedUrls(ctx context.Context, st *store.Store, spec OpenedUrlListSpec) errtrace.Result[[]OpenedUrlRow] {
	rows, err := st.QueryOpenedUrls(ctx, openedUrlFilterFromSpec(spec))
	if err != nil {
		return errtrace.Err[[]OpenedUrlRow](errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "QueryOpenedUrls"))
	}
	defer rows.Close()
	out, err := scanOpenedUrlRows(rows)
	if err != nil {
		return errtrace.Err[[]OpenedUrlRow](errtrace.WrapCode(err, errtrace.ErrToolsInvalidArgument, "scan"))
	}
	return errtrace.Ok(out)
}

// openedUrlFilterFromSpec translates the core-side spec (which carries
// the typed `OpenUrlOrigin` enum) into the primitive store-side filter.
// Keeps the import direction one-way: core → store.
func openedUrlFilterFromSpec(spec OpenedUrlListSpec) store.OpenedUrlListFilter {
	return store.OpenedUrlListFilter{
		Before: spec.Before,
		Alias:  spec.Alias,
		Origin: string(spec.Origin),
		Limit:  spec.Limit,
	}
}

func scanOpenedUrlRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]OpenedUrlRow, error) {
	var out []OpenedUrlRow
	for rows.Next() {
		var (
			r                                  OpenedUrlRow
			origin                             string
			isDeduped, isIncognito             int
		)
		if err := rows.Scan(&r.Id, &r.EmailId, &r.Alias, &r.RuleName, &origin,
			&r.Url, &r.OriginalUrl, &isDeduped, &isIncognito, &r.TraceId, &r.OpenedAt); err != nil {
			return nil, err
		}
		r.Origin = OpenUrlOrigin(origin)
		r.IsDeduped = isDeduped != 0
		r.IsIncognito = isIncognito != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
