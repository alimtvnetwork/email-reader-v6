package config

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	plain := "ZPb*sz=d!cEE_Wgc"
	enc := EncodePassword(plain)
	if enc == plain {
		t.Fatalf("password should be encoded, got plaintext")
	}
	dec, err := DecodePassword(enc)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if dec != plain {
		t.Fatalf("round trip mismatch: got %q want %q", dec, plain)
	}
}

func TestUpsertAndRemoveAccount(t *testing.T) {
	c := Default()
	c.UpsertAccount(Account{Alias: "a", Email: "a@x"})
	c.UpsertAccount(Account{Alias: "b", Email: "b@x"})
	if len(c.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(c.Accounts))
	}
	c.UpsertAccount(Account{Alias: "a", Email: "a2@x"})
	if got := c.FindAccount("a").Email; got != "a2@x" {
		t.Fatalf("upsert did not replace: %s", got)
	}
	if !c.RemoveAccount("b") {
		t.Fatal("remove returned false")
	}
	if c.FindAccount("b") != nil {
		t.Fatal("account still present after remove")
	}
}
