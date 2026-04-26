// emails_isread_roundtrip_test.go — confirms the M0010 `IsRead`
// column round-trips into `store.Email.IsRead` through both
// `ListEmails` and `GetEmailByUid`.
//
// Why a dedicated file?
//   - This test is the unit-level guarantee that the P4.6 follow-up's
//     `core.EmailQuery.OnlyUnread` filter has correct upstream data.
//     If the column ever drops out of `emailColumns` again, this test
//     fails before any core-level test would.
//
// Coverage:
//   - Default insert → IsRead=false (DB DEFAULT 0 carries through).
//   - SetEmailRead(...,true) → next ListEmails / GetEmailByUid sees
//     IsRead=true.
//   - Toggle back to false → likewise.

package store

import (
	"context"
	"testing"
	"time"
)

func TestEmail_IsRead_RoundTripsThroughListAndGet(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	if _, _, err := st.UpsertEmail(ctx, &Email{
		Alias: "a", MessageId: "m1@x", Uid: 1,
		FromAddr: "f@x", Subject: "s",
		ReceivedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertEmail: %v", err)
	}

	// Initial read: IsRead must be false (DB default).
	rows, err := st.ListEmails(ctx, EmailQuery{Alias: "a"})
	if err != nil {
		t.Fatalf("ListEmails: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListEmails returned %d rows, want 1", len(rows))
	}
	if rows[0].IsRead {
		t.Errorf("default IsRead = true, want false")
	}
	got, err := st.GetEmailByUid(ctx, "a", 1)
	if err != nil {
		t.Fatalf("GetEmailByUid: %v", err)
	}
	if got.IsRead {
		t.Errorf("GetEmailByUid default IsRead = true, want false")
	}

	// Flip to true.
	if _, err := st.SetEmailRead(ctx, "a", []uint32{1}, true); err != nil {
		t.Fatalf("SetEmailRead true: %v", err)
	}
	rows, _ = st.ListEmails(ctx, EmailQuery{Alias: "a"})
	if !rows[0].IsRead {
		t.Errorf("after SetEmailRead(true): ListEmails IsRead = false, want true")
	}
	got, _ = st.GetEmailByUid(ctx, "a", 1)
	if !got.IsRead {
		t.Errorf("after SetEmailRead(true): GetEmailByUid IsRead = false, want true")
	}

	// Flip back to false.
	if _, err := st.SetEmailRead(ctx, "a", []uint32{1}, false); err != nil {
		t.Fatalf("SetEmailRead false: %v", err)
	}
	rows, _ = st.ListEmails(ctx, EmailQuery{Alias: "a"})
	if rows[0].IsRead {
		t.Errorf("after SetEmailRead(false): IsRead stuck at true")
	}
}
