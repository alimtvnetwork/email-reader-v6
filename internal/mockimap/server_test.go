package mockimap

import (
	"testing"
	"time"

	"github.com/emersion/go-imap/client"
)

// Test_Server_LoginSelectLogout is the E.1 acceptance smoke: a real
// go-imap client can complete CAPABILITY → LOGIN → SELECT → LOGOUT
// against the mock server, end-to-end, in <1s.
func Test_Server_LoginSelectLogout(t *testing.T) {
	srv := New("alice", "s3cret", []Message{
		{From: "boss@x", To: "alice@x", Subject: "hello", ReceivedAt: "Mon, 1 Jan 2024 00:00:00 +0000"},
	})
	addr, err := srv.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.Timeout = 2 * time.Second
	defer c.Logout()

	if err := c.Login("alice", "s3cret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	mb, err := c.Select("INBOX", false)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if mb.Messages != 1 {
		t.Fatalf("EXISTS = %d, want 1", mb.Messages)
	}
	if mb.UidNext != 2 {
		t.Fatalf("UIDNEXT = %d, want 2", mb.UidNext)
	}
}

// Test_Server_LoginRejectsBadPassword verifies the negative path used by
// AC-PROJ tests covering ER-MAIL-21201 (auth failure).
func Test_Server_LoginRejectsBadPassword(t *testing.T) {
	srv := New("alice", "s3cret", nil)
	addr, _ := srv.Start()
	defer srv.Stop()

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.Timeout = 2 * time.Second
	defer c.Logout()

	if err := c.Login("alice", "wrong"); err == nil {
		t.Fatal("login with wrong password unexpectedly succeeded")
	}
}

// Test_Server_FailNextLoginInjectsAuthError lets a test simulate a server
// that's currently rejecting valid credentials (e.g. rate-limited).
func Test_Server_FailNextLoginInjectsAuthError(t *testing.T) {
	srv := New("alice", "s3cret", nil)
	addr, _ := srv.Start()
	defer srv.Stop()
	srv.FailNextLogin("rate limited")

	c, _ := client.Dial(addr)
	c.Timeout = 2 * time.Second
	if err := c.Login("alice", "s3cret"); err == nil {
		t.Fatal("FailNextLogin did not inject an error")
	}

	// After the injected failure clears, the next attempt should succeed.
	c2, _ := client.Dial(addr)
	c2.Timeout = 2 * time.Second
	defer c2.Logout()
	if err := c2.Login("alice", "s3cret"); err != nil {
		t.Fatalf("post-injection login failed: %v", err)
	}
}

// Test_Server_DeliverGrowsMailbox is an E.1 sanity check that Deliver()
// actually mutates the mailbox the server serves; subsequent SELECT sees
// the new EXISTS count.
func Test_Server_DeliverGrowsMailbox(t *testing.T) {
	srv := New("alice", "s3cret", nil)
	addr, _ := srv.Start()
	defer srv.Stop()

	srv.Deliver(Message{From: "a", To: "b", Subject: "first"})
	srv.Deliver(Message{From: "a", To: "b", Subject: "second"})

	if got := srv.MessageCount(); got != 2 {
		t.Fatalf("MessageCount = %d, want 2", got)
	}

	c, _ := client.Dial(addr)
	c.Timeout = 2 * time.Second
	if err := c.Login("alice", "s3cret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	defer c.Logout()
	mb, err := c.Select("INBOX", false)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if mb.Messages != 2 {
		t.Fatalf("EXISTS = %d, want 2", mb.Messages)
	}
}
