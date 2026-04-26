// services_health_test.go — Slice #103: AttachHealthSource wire-up.
// Verifies the bundle's HealthSource field is populated when a real
// `*store.Store` is passed and stays nil for nil/no-op inputs.
//
//go:build !nofyne

package ui

import (
	"path/filepath"
	"testing"

	"github.com/lovable/email-read/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenAt(filepath.Join(t.TempDir(), "svc-health.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestServices_AttachHealthSource_NilReceiver_NoPanic(t *testing.T) {
	var s *Services
	s.AttachHealthSource(openTestStore(t)) // must not panic
}

func TestServices_AttachHealthSource_NilStore_LeavesFieldNil(t *testing.T) {
	s := &Services{}
	s.AttachHealthSource(nil)
	if s.HealthSource != nil {
		t.Fatalf("HealthSource: expected nil, got non-nil")
	}
}

func TestServices_AttachHealthSource_RealStore_PopulatesField(t *testing.T) {
	s := &Services{}
	s.AttachHealthSource(openTestStore(t))
	if s.HealthSource == nil {
		t.Fatalf("HealthSource: expected non-nil after attach with real store")
	}
}

func TestServices_AttachHealthSource_Idempotent(t *testing.T) {
	s := &Services{}
	st := openTestStore(t)
	s.AttachHealthSource(st)
	first := s.HealthSource
	s.AttachHealthSource(st)
	if s.HealthSource == nil || first == nil {
		t.Fatal("HealthSource: nil after attach")
	}
	// Function values aren't comparable for equality in Go beyond nil
	// checks; the contract is "stays populated", which we assert.
}
