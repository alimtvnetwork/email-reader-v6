package ui

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestAppState_SetAliasFiresOnChange(t *testing.T) {
	s := NewAppState()
	var got AppStateEvent
	var calls int32
	s.Subscribe(func(ev AppStateEvent) {
		got = ev
		atomic.AddInt32(&calls, 1)
	})

	s.SetAlias("a")
	if got.Alias != "a" || got.PrevAlias != "" {
		t.Errorf("first set: %+v", got)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	// No-op set should not fire.
	s.SetAlias("a")
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("no-op fired: calls = %d", calls)
	}
	s.SetAlias("b")
	if got.PrevAlias != "a" || got.Alias != "b" {
		t.Errorf("transition: %+v", got)
	}
}

func TestAppState_SetNavFiresOnChange(t *testing.T) {
	s := NewAppState()
	var seen []NavKind
	s.Subscribe(func(ev AppStateEvent) { seen = append(seen, ev.Nav) })
	s.SetNav(NavEmails)
	s.SetNav(NavEmails) // dedupe
	s.SetNav(NavRules)
	if len(seen) != 2 || seen[0] != NavEmails || seen[1] != NavRules {
		t.Errorf("seen = %v", seen)
	}
}

// TestAppState_ConcurrentAccess stresses the lock to catch races. Run with
// -race; failures show up as data-race reports, not test assertions.
func TestAppState_ConcurrentAccess(t *testing.T) {
	s := NewAppState()
	s.Subscribe(func(AppStateEvent) {})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				s.SetAlias("a")
			} else {
				s.SetAlias("b")
			}
		}(i)
		go func() {
			defer wg.Done()
			_ = s.Alias()
			_ = s.Nav()
		}()
	}
	wg.Wait()
}
