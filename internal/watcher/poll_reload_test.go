package watcher

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

// Test_ApplyPollReload_ClampsAndLogs covers the live-reload helper that
// receives PollSeconds updates from the Settings event channel. It must:
//   - clamp values < 1 to 1 and > 60 to 60 (matches ER-SET-21771 server-side
//     validation, but the watcher is defensive in case a buggy producer
//     forwards an out-of-range value).
//   - update the *time.Duration in place so the next tick.Reset picks it up.
//   - log a single "poll cadence reloaded" line on actual change.
//   - be a no-op (no log) when the new cadence equals the current one.
//
// Spec: spec/21-app/02-features/07-settings/01-backend.md §8 (CF-W1).
func Test_ApplyPollReload_ClampsAndLogs(t *testing.T) {
	cases := []struct {
		name         string
		startSecs    int
		incoming     int
		wantSecs     int
		wantLogMatch string
	}{
		{"normal change", 3, 10, 10, "3s → 10s"},
		{"clamp low", 5, 0, 1, "5s → 1s"},
		{"clamp negative", 5, -100, 1, "5s → 1s"},
		{"clamp high", 5, 999, 60, "5s → 1m0s"},
		{"no-op same value", 7, 7, 7, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			lg := log.New(&buf, "", 0)
			poll := time.Duration(tc.startSecs) * time.Second
			applyPollReload(lg, "alias-1", &poll, tc.incoming)
			if got := int(poll / time.Second); got != tc.wantSecs {
				t.Fatalf("poll = %ds, want %ds", got, tc.wantSecs)
			}
			out := buf.String()
			if tc.wantLogMatch == "" {
				if out != "" {
					t.Fatalf("expected no log, got %q", out)
				}
				return
			}
			if !strings.Contains(out, "poll cadence reloaded") || !strings.Contains(out, tc.wantLogMatch) {
				t.Fatalf("log %q missing %q", out, tc.wantLogMatch)
			}
		})
	}
}
