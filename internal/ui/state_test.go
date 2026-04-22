package ui

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestAppState_DefaultState verifies the zero-config starting point.
func TestAppState_DefaultState(t *testing.T) {
	s := NewAppState()
	if got := s.Alias(); got != "" {
		t.Errorf("default Alias = %q, want empty", got)
	}
	if got := s.Nav(); got != NavDashboard {
		t.Errorf("default Nav = %q, want %q", got, NavDashboard)
	}
}

// TestAppState_SetAliasNotifies confirms subscribers see the transition with
// both prev + new alias populated.
func TestAppState_SetAliasNotifies(t *testing.T) {
	s := NewAppState()
	var got AppStateEvent
	var calls int32
	s.Subscribe(func(ev AppStateEvent) {
		atomic.AddInt32(&calls, 1)
		got = ev
	})

	s.SetAlias("primary")
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("subscriber called %d times, want 1", calls)
	}
	if got.PrevAlias != "" || got.Alias != "primary" {
		t.Errorf("event = %+v, want PrevAlias=\"\" Alias=\"primary\"", got)
	}

	// No-op set must NOT trigger a second callback.
	s.SetAlias("primary")
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("no-op SetAlias triggered subscriber: calls=%d", calls)
	}
}

// TestAppState_SetNavNotifies mirrors the alias test for nav transitions.
func TestAppState_SetNavNotifies(t *testing.T) {
	s := NewAppState()
	var got AppStateEvent
	s.Subscribe(func(ev AppStateEvent) { got = ev })

	s.SetNav(NavWatch)
	if got.PrevNav != NavDashboard || got.Nav != NavWatch {
		t.Errorf("event = %+v, want PrevNav=dashboard Nav=watch", got)
	}
}

// TestAppState_Concurrent runs many setters + readers; with -race this
// guarantees the mutex protects shared state.
func TestAppState_Concurrent(t *testing.T) {
	s := NewAppState()
	s.Subscribe(func(AppStateEvent) {})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if n%2 == 0 {
					s.SetAlias("a")
					s.SetAlias("b")
				} else {
					_ = s.Alias()
					_ = s.Nav()
				}
			}
		}(i)
	}
	wg.Wait()
}
