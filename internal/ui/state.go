// state.go — framework-agnostic UI state shared between the sidebar (which
// owns the account picker + nav list) and the detail views (which need the
// currently selected alias + nav). No fyne imports here on purpose so the
// observer pattern is unit-testable on headless CI.
package ui

import "sync"

// AppStateEvent is delivered to subscribers when state changes. PrevAlias
// / PrevNav let listeners diff cheaply without re-querying.
type AppStateEvent struct {
	PrevAlias string
	Alias     string
	PrevNav   NavKind
	Nav       NavKind
}

// AppState holds the selected account alias + active sidebar nav. Safe for
// concurrent use; the only writers are the UI goroutine and the watcher
// (Phase 5) reporting status, but the lock keeps stress tests honest.
type AppState struct {
	mu          sync.RWMutex
	alias       string
	nav         NavKind
	subscribers []func(AppStateEvent)
}

// NewAppState returns a zero-value state pointing at the first nav item.
func NewAppState() *AppState {
	return &AppState{nav: NavDashboard}
}

// Alias returns the selected account alias ("" ⇒ none).
func (s *AppState) Alias() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alias
}

// SetAlias updates the alias and fans out an event when it changes.
func (s *AppState) SetAlias(a string) {
	s.mu.Lock()
	if s.alias == a {
		s.mu.Unlock()
		return
	}
	prev := s.alias
	s.alias = a
	subs := append([]func(AppStateEvent){}, s.subscribers...)
	curNav := s.nav
	s.mu.Unlock()
	ev := AppStateEvent{PrevAlias: prev, Alias: a, PrevNav: curNav, Nav: curNav}
	for _, fn := range subs {
		fn(ev)
	}
}

// Nav returns the active sidebar destination.
func (s *AppState) Nav() NavKind {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nav
}

// SetNav updates the active nav and fans out an event when it changes.
func (s *AppState) SetNav(k NavKind) {
	s.mu.Lock()
	if s.nav == k {
		s.mu.Unlock()
		return
	}
	prev := s.nav
	s.nav = k
	subs := append([]func(AppStateEvent){}, s.subscribers...)
	curAlias := s.alias
	s.mu.Unlock()
	ev := AppStateEvent{PrevAlias: curAlias, Alias: curAlias, PrevNav: prev, Nav: k}
	for _, fn := range subs {
		fn(ev)
	}
}

// Subscribe registers fn to receive future state changes. There is no
// unsubscribe — the shell owns the AppState for its full lifetime so
// subscribers live as long as the process.
func (s *AppState) Subscribe(fn func(AppStateEvent)) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.subscribers = append(s.subscribers, fn)
	s.mu.Unlock()
}
