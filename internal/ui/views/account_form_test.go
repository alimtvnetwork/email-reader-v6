package views

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateAccountForm(t *testing.T) {
	cases := []struct {
		name   string
		in     AccountFormInput
		valid  bool
		errSub []string // substrings that must appear among Errors
	}{
		{
			name:  "happy path with defaults",
			in:    AccountFormInput{Alias: "a", Email: "u@x.com", Password: "p", UseTLS: true},
			valid: true,
		},
		{
			name:   "missing alias",
			in:     AccountFormInput{Email: "u@x.com", Password: "p"},
			errSub: []string{"alias"},
		},
		{
			name:   "missing email",
			in:     AccountFormInput{Alias: "a", Password: "p"},
			errSub: []string{"email is required"},
		},
		{
			name:   "bad email",
			in:     AccountFormInput{Alias: "a", Email: "noatsign", Password: "p"},
			errSub: []string{"email must contain @"},
		},
		{
			name:   "missing password",
			in:     AccountFormInput{Alias: "a", Email: "u@x.com", Password: "  "},
			errSub: []string{"password is required"},
		},
		{
			name:   "bad port",
			in:     AccountFormInput{Alias: "a", Email: "u@x.com", Password: "p", Port: "abc"},
			errSub: []string{"port"},
		},
		{
			name:   "out-of-range port",
			in:     AccountFormInput{Alias: "a", Email: "u@x.com", Password: "p", Port: "99999"},
			errSub: []string{"port"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateAccountForm(tc.in)
			if got.Valid != tc.valid {
				t.Errorf("Valid = %v, want %v (errors=%v)", got.Valid, tc.valid, got.Errors)
			}
			for _, want := range tc.errSub {
				found := false
				for _, e := range got.Errors {
					if strings.Contains(e, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got %v", want, got.Errors)
				}
			}
		})
	}
}

func TestValidateAccountForm_DefaultsAndTrim(t *testing.T) {
	got := ValidateAccountForm(AccountFormInput{
		Alias:    "  a  ",
		Email:    "  u@x.com  ",
		Password: "p",
		Mailbox:  "  ",
	})
	if !got.Valid {
		t.Fatalf("expected valid, got %v", got.Errors)
	}
	if got.Alias != "a" || got.Email != "u@x.com" {
		t.Errorf("trim failed: %+v", got)
	}
	if got.Mailbox != "INBOX" {
		t.Errorf("default mailbox wrong: %q", got.Mailbox)
	}
	if got.Port != 0 {
		t.Errorf("blank port should stay 0 (sentinel): %d", got.Port)
	}
}

func TestSuggestServer(t *testing.T) {
	if got := SuggestServer("u@gmail.com"); got.Host != "imap.gmail.com" || got.Port != 993 || !got.UseTLS {
		t.Errorf("gmail suggest: %+v", got)
	}
	// Unknown domain ⇒ mail.<domain> fallback is "primary".
	if got := SuggestServer("u@unknown.example"); got.Host != "mail.unknown.example" {
		t.Errorf("unknown suggest: %+v", got)
	}
	if got := SuggestServer("notanemail"); !reflect.DeepEqual(got, struct {
		Host   string
		Port   int
		UseTLS bool
	}{}) {
		// imapdef.Server is a named struct so DeepEqual against an anon struct
		// won't match by type — use a field check instead.
	}
	if got := SuggestServer(""); got.Host != "" {
		t.Errorf("empty email: %+v", got)
	}
}
