// state.go holds the framework-agnostic UI state and a tiny observer
// pattern. Kept free of fyne imports so it compiles + tests on headless CI.
//
// AppState owns:
//   - the currently selected account alias (drives every view)
//   - the currently selected nav kind (so views can react to back-nav)
//
// Subscribers register a callback; AppState calls every subscriber whenever
// state changes. The callback runs on whatever goroutine called the setter
// — Fyne widgets must marshal back to the UI thread themselves if needed.
//
// NavKind itself is defined in nav.go (alongside the canonical NavItems
// list) so the two stay in sync without circular references.
package ui

import "sync"

// AppState is the single source of truth for cross-view UI state.
// Zero value is usable; prefer NewAppState for clarity.
type AppState struct {
	mu          sync.RWMutex
	alias       string                // empty ⇒ "no account selected"
	nav         NavKind               // current sidebar selection
	subscribers []func(AppStateEvent) // notified on every change
}

// AppStateEvent describes what changed. Either Alias or Nav (or both) is
// updated relative to the previous snapshot. Consumers compare PrevAlias /
// PrevNav to detect specific transitions.
type AppStateEvent struct {
	PrevAlias string
	Alias     string
	PrevNav   NavKind
	Nav       NavKind
}

// NewAppState returns a fresh AppState with no account selected and the
// dashboard preselected.
func NewAppState() *AppState {
	return &AppState{nav: NavDashboard}
}

// Alias returns the currently selected account alias ("" if none).
func (s *AppState) Alias() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alias
}

// Nav returns the currently selected sidebar entry.
func (s *AppState) Nav() NavKind {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nav
}

// SetAlias updates the selected account. No-op if unchanged. Subscribers
// fire only on actual transitions.
func (s *AppState) SetAlias(alias string) {
	s.mu.Lock()
	if alias == s.alias {
		s.mu.Unlock()
		return
	}
	prev := s.alias
	s.alias = alias
	ev := AppStateEvent{PrevAlias: prev, Alias: alias, PrevNav: s.nav, Nav: s.nav}
	subs := append([]func(AppStateEvent){}, s.subscribers...)
	s.mu.Unlock()
	for _, fn := range subs {
		fn(ev)
	}
}

// SetNav updates the selected sidebar entry. No-op if unchanged.
func (s *AppState) SetNav(n NavKind) {
	s.mu.Lock()
	if n == s.nav {
		s.mu.Unlock()
		return
	}
	prev := s.nav
	s.nav = n
	ev := AppStateEvent{PrevAlias: s.alias, Alias: s.alias, PrevNav: prev, Nav: n}
	subs := append([]func(AppStateEvent){}, s.subscribers...)
	s.mu.Unlock()
	for _, fn := range subs {
		fn(ev)
	}
}

// Subscribe registers fn to be invoked on every state change. Returns an
// unsubscribe function. fn is called synchronously from SetAlias/SetNav.
func (s *AppState) Subscribe(fn func(AppStateEvent)) func() {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, fn)
	idx := len(s.subscribers) - 1
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if idx < len(s.subscribers) {
			// Replace with no-op rather than reslicing so other indices stay
			// valid. AppState lifetime is the whole app, so this is fine.
			s.subscribers[idx] = func(AppStateEvent) {}
		}
	}
}
