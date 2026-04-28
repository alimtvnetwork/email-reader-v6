// Package benchgate implements ADVISORY p95 latency gates for the
// AC-DBP and AC-SP acceptance rows. "Advisory" means: gate output is
// recorded and rendered, but a budget breach NEVER fails the test —
// the sandbox cannot produce stable timing, so an enforcing gate would
// flap. On real hardware (workstation / dedicated CI runner) flip
// `Mode = ModeEnforcing` to promote breaches to test failures.
//
// Why a separate package: each Benchmark* lives in its own file and
// reports a single op latency. benchgate collects N samples per gate,
// computes p95 against a declared budget, and renders a single roll-up
// table so the user sees ALL gates in one block at the end of `go test`.
//
// Out of scope today: cross-process IMAP latency (covered by the E2E
// harness scaffolding in internal/mockimap), GUI redraw timing (cgo+GL
// blocked), and any hardware-specific tuning. Add new gates by appending
// to DefaultGates with the spec ID, the function under measure, and the
// budget.
package benchgate

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Mode controls whether budget breaches fail the test.
type Mode int

const (
	// ModeAdvisory records breaches but never fails the test. Sandbox default.
	ModeAdvisory Mode = iota
	// ModeEnforcing promotes a breach to t.Fatal. Use on stable hardware only.
	ModeEnforcing
)

// Gate is one measurable workload.
type Gate struct {
	// SpecID is the AC row this gate satisfies (e.g. "AC-DBP-01").
	SpecID string
	// Name is a human-readable label rendered in the report.
	Name string
	// Budget is the p95 latency budget. Breach == p95 > Budget.
	Budget time.Duration
	// Run executes ONE iteration. Called Samples times per Measure().
	Run func() error
}

// Result is one Measure() output.
type Result struct {
	Gate    Gate
	Samples int
	P50     time.Duration
	P95     time.Duration
	Max     time.Duration
	Err     error // first non-nil Run error, if any
	Breach  bool  // P95 > Gate.Budget
}

// Measure runs the gate `samples` times and computes p50/p95/max. The
// caller picks the sample count — small (50) for fast PR gates, larger
// (500) for nightly precision. Returns the result whether or not the
// budget was breached; advisory mode means the caller decides.
func Measure(g Gate, samples int) Result {
	if samples <= 0 {
		samples = 50
	}
	durs := make([]time.Duration, 0, samples)
	res := Result{Gate: g, Samples: samples}
	for i := 0; i < samples; i++ {
		t0 := time.Now()
		if err := g.Run(); err != nil {
			if res.Err == nil {
				res.Err = err
			}
			// Continue measuring — one error shouldn't drop the whole gate;
			// the report shows both the error and what timing we did capture.
			continue
		}
		durs = append(durs, time.Since(t0))
	}
	if len(durs) == 0 {
		return res
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	res.P50 = durs[len(durs)/2]
	res.P95 = durs[(len(durs)*95)/100]
	if res.P95 < res.P50 {
		res.P95 = durs[len(durs)-1] // tiny sample set; fall back to max
	}
	res.Max = durs[len(durs)-1]
	res.Breach = res.P95 > g.Budget
	return res
}

// Report renders multiple Results as a fixed-column table. Mode is
// rendered in the header so a reader of CI logs can tell at a glance
// whether breaches were enforced or merely advised.
func Report(mode Mode, results []Result) string {
	var b strings.Builder
	modeLabel := "ADVISORY (breaches do NOT fail the test)"
	if mode == ModeEnforcing {
		modeLabel = "ENFORCING (breaches FAIL the test)"
	}
	fmt.Fprintf(&b, "Bench gate roll-up — %s\n", modeLabel)
	fmt.Fprintf(&b, "  %-14s %-32s %10s %10s %10s %10s %s\n",
		"SpecID", "Name", "Samples", "P50", "P95", "Budget", "Result")
	for _, r := range results {
		mark := "✓"
		if r.Breach {
			mark = "✗ BREACH"
		}
		if r.Err != nil {
			mark = "! ERR"
		}
		fmt.Fprintf(&b, "  %-14s %-32s %10d %10s %10s %10s %s\n",
			r.Gate.SpecID, truncate(r.Gate.Name, 32), r.Samples,
			fmtDur(r.P50), fmtDur(r.P95), fmtDur(r.Gate.Budget), mark)
	}
	return b.String()
}

func fmtDur(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
