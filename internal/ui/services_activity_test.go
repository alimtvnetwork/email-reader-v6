// services_activity_test.go — Slice #105: AttachActivitySource wire-up.
// Twin of services_health_test.go.
//
//go:build !nofyne

package ui

import (
	"testing"
)

func TestServices_AttachActivitySource_NilReceiver_NoPanic(t *testing.T) {
	var s *Services
	s.AttachActivitySource(openTestStore(t)) // must not panic
}

func TestServices_AttachActivitySource_NilStore_LeavesFieldNil(t *testing.T) {
	s := &Services{}
	s.AttachActivitySource(nil)
	if s.ActivitySource != nil {
		t.Fatalf("ActivitySource: expected nil, got non-nil")
	}
}

func TestServices_AttachActivitySource_RealStore_PopulatesField(t *testing.T) {
	s := &Services{}
	s.AttachActivitySource(openTestStore(t))
	if s.ActivitySource == nil {
		t.Fatalf("ActivitySource: expected non-nil after attach with real store")
	}
}

func TestServices_AttachActivitySource_Idempotent(t *testing.T) {
	s := &Services{}
	st := openTestStore(t)
	s.AttachActivitySource(st)
	if s.ActivitySource == nil {
		t.Fatal("ActivitySource: nil after first attach")
	}
	s.AttachActivitySource(st)
	if s.ActivitySource == nil {
		t.Fatal("ActivitySource: nil after second attach")
	}
}
