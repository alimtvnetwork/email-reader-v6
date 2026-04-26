// cf_d1_dashboard_test.go locks CF-D1: the Dashboard's "Auto-start
// watcher" indicator must reflect the latest `Settings.AutoStartWatch`
// within ≤1s of a Save. The indicator widget itself lives behind
// !nofyne in views/dashboard.go (formatAutoStart + forwardAutoStartEvents),
// but its CONTRACT is the Settings publish — if the Snapshot in a
// SettingsEvent carries the new AutoStartWatch value within the spec
// budget, the indicator will too (Fyne SetText is synchronous).
//
// We therefore test the publish contract here:
//   - Subscribe to Settings.
//   - Save with AutoStartWatch flipped.
//   - Assert the SettingsEvent arrives within 1s and Snapshot.AutoStartWatch
//     equals the new value.
//
// Spec: spec/21-app/02-features/01-dashboard/99-consistency-report.md CF-D1.
package core

import (
	"context"
	"testing"
	"time"
)

func TestCF_D1_Dashboard_AutoStartIndicator_Live(t *testing.T) {
	withIsolatedConfig(t, func() {
		s := newTestSettings(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events, sub := s.Subscribe(ctx)
		defer sub()

		// Baseline: AutoStartWatch defaults to false.
		base := s.Get(context.Background())
		if base.HasError() {
			t.Fatalf("Get baseline: %v", base.Error())
		}
		if base.Value().AutoStartWatch {
			t.Fatalf("baseline AutoStartWatch = true; expected false")
		}

		in := DefaultSettingsInput()
		in.AutoStartWatch = true
		if r := s.Save(context.Background(), in); r.HasError() {
			t.Fatalf("Save: %v", r.Error())
		}

		select {
		case ev := <-events:
			if !ev.Snapshot.AutoStartWatch {
				t.Fatalf("event Snapshot.AutoStartWatch = false; want true (CF-D1)")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("no Settings event within 1s of Save — CF-D1 budget breached")
		}
	})
}
