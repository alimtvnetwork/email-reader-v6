// emails_deletedat_roundtrip_test.go — confirms the M0012 `DeletedAt`
// column round-trips through `Store.SetEmailDeletedAt`.
//
// Coverage:
//   - Default insert → DeletedAt IS NULL (no DEFAULT, NULL = sentinel).
//   - SetEmailDeletedAt(non-nil) → DB row has the int64 stamp.
//   - SetEmailDeletedAt(nil) → DB row resets to NULL.
//   - Empty UIDs → no SQL issued, RowsAffected = 0.
//   - Multi-UID single call → all rows updated atomically.
package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestSetEmailDeletedAt_RoundTrip(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	for _, uid := range []uint32{1, 2, 3} {
		if _, _, err := st.UpsertEmail(ctx, &Email{
			Alias:      "user@x",
			MessageId:  "<msg-" + string(rune('0'+uid)) + "@y>",
			Uid:        uid,
			Subject:    "s",
			ReceivedAt: now,
		}); err != nil {
			t.Fatalf("seed uid=%d: %v", uid, err)
		}
	}

	// Default: DeletedAt is NULL.
	for _, uid := range []uint32{1, 2, 3} {
		var d sql.NullInt64
		if err := st.DB.QueryRowContext(ctx,
			`SELECT DeletedAt FROM Emails WHERE Alias = ? AND Uid = ?`,
			"user@x", uid).Scan(&d); err != nil {
			t.Fatalf("read default uid=%d: %v", uid, err)
		}
		if d.Valid {
			t.Errorf("uid=%d: default DeletedAt = %d, want NULL", uid, d.Int64)
		}
	}

	// Stamp uids 1 & 2.
	stamp := now.Unix()
	rows, err := st.SetEmailDeletedAt(ctx, "user@x", []uint32{1, 2}, &stamp)
	if err != nil {
		t.Fatalf("SetEmailDeletedAt(non-nil): %v", err)
	}
	if rows != 2 {
		t.Errorf("RowsAffected = %d, want 2", rows)
	}

	for _, uid := range []uint32{1, 2} {
		var d sql.NullInt64
		if err := st.DB.QueryRowContext(ctx,
			`SELECT DeletedAt FROM Emails WHERE Alias = ? AND Uid = ?`,
			"user@x", uid).Scan(&d); err != nil {
			t.Fatalf("read uid=%d: %v", uid, err)
		}
		if !d.Valid || d.Int64 != stamp {
			t.Errorf("uid=%d: DeletedAt = (valid=%v val=%d), want %d", uid, d.Valid, d.Int64, stamp)
		}
	}

	// uid 3 still NULL — non-targeted rows untouched.
	var d sql.NullInt64
	_ = st.DB.QueryRowContext(ctx,
		`SELECT DeletedAt FROM Emails WHERE Alias = ? AND Uid = ?`,
		"user@x", uint32(3)).Scan(&d)
	if d.Valid {
		t.Errorf("uid=3 untargeted but DeletedAt set to %d", d.Int64)
	}

	// Undelete uid 1 (nil → NULL).
	rows, err = st.SetEmailDeletedAt(ctx, "user@x", []uint32{1}, nil)
	if err != nil {
		t.Fatalf("SetEmailDeletedAt(nil): %v", err)
	}
	if rows != 1 {
		t.Errorf("undelete RowsAffected = %d, want 1", rows)
	}
	_ = st.DB.QueryRowContext(ctx,
		`SELECT DeletedAt FROM Emails WHERE Alias = ? AND Uid = ?`,
		"user@x", uint32(1)).Scan(&d)
	if d.Valid {
		t.Errorf("after undelete, uid=1 DeletedAt = %d, want NULL", d.Int64)
	}
}

func TestSetEmailDeletedAt_EmptyUids_NoSQL(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	stamp := int64(123)
	rows, err := st.SetEmailDeletedAt(ctx, "user@x", nil, &stamp)
	if err != nil {
		t.Fatalf("empty uids: %v", err)
	}
	if rows != 0 {
		t.Errorf("RowsAffected = %d, want 0 (empty uids = no-op)", rows)
	}
	rows, err = st.SetEmailDeletedAt(ctx, "user@x", []uint32{}, nil)
	if err != nil {
		t.Fatalf("empty uids slice: %v", err)
	}
	if rows != 0 {
		t.Errorf("empty slice RowsAffected = %d, want 0", rows)
	}
}
