package benchgate

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Test_Measure_HappyPath verifies the basic shape: N samples, sorted
// p50/p95, no breach when Run is fast.
func Test_Measure_HappyPath(t *testing.T) {
	g := Gate{
		SpecID: "AC-TEST-01",
		Name:   "noop",
		Budget: 10 * time.Millisecond,
		Run:    func() error { return nil },
	}
	r := Measure(g, 50)
	if r.Err != nil {
		t.Fatalf("unexpected Err: %v", r.Err)
	}
	if r.Samples != 50 {
		t.Errorf("Samples = %d, want 50", r.Samples)
	}
	if r.Breach {
		t.Errorf("noop should not breach 10ms budget; p95=%s", r.P95)
	}
}

// Test_Measure_RecordsBreachWithoutPanicking proves a breach is observed
// and rendered but never crashes the runner.
func Test_Measure_RecordsBreachWithoutPanicking(t *testing.T) {
	g := Gate{
		SpecID: "AC-TEST-02",
		Name:   "slow",
		Budget: 1 * time.Microsecond, // intentionally unreachable
		Run:    func() error { time.Sleep(2 * time.Millisecond); return nil },
	}
	r := Measure(g, 10)
	if !r.Breach {
		t.Fatal("expected Breach=true for sub-µs budget vs ms workload")
	}
	out := Report(ModeAdvisory, []Result{r})
	if !strings.Contains(out, "BREACH") {
		t.Errorf("Report missing BREACH marker:\n%s", out)
	}
	if !strings.Contains(out, "ADVISORY") {
		t.Errorf("Report missing ADVISORY mode label:\n%s", out)
	}
}

// Test_Measure_PreservesFirstRunError keeps the gate going when one
// iteration errors; both timing and error are surfaced in the report.
func Test_Measure_PreservesFirstRunError(t *testing.T) {
	calls := 0
	g := Gate{
		SpecID: "AC-TEST-03",
		Name:   "flaky",
		Budget: time.Second,
		Run: func() error {
			calls++
			if calls == 1 {
				return errors.New("boom")
			}
			return nil
		},
	}
	r := Measure(g, 5)
	if r.Err == nil {
		t.Fatal("Err nil; expected first iteration error to be captured")
	}
	if calls != 5 {
		t.Errorf("calls = %d, want 5 (must keep measuring after err)", calls)
	}
}
