// account_form_displayname_test.go covers the DisplayName plumbing added
// in slice 1 of the Add Account form extension: input is trimmed, blank
// is allowed (it's optional), and the cleaned value flows to the result.
package views

import "testing"

func TestValidateAccountForm_DisplayNameTrimmed(t *testing.T) {
	got := ValidateAccountForm(AccountFormInput{
		Alias:       "a",
		Email:       "u@x.com",
		Password:    "p",
		DisplayName: "  Work — Sales  ",
	})
	if !got.Valid {
		t.Fatalf("expected valid, got %v", got.Errors)
	}
	if got.DisplayName != "Work — Sales" {
		t.Errorf("DisplayName trim failed: %q", got.DisplayName)
	}
}

func TestValidateAccountForm_DisplayNameOptional(t *testing.T) {
	got := ValidateAccountForm(AccountFormInput{
		Alias: "a", Email: "u@x.com", Password: "p",
	})
	if !got.Valid {
		t.Fatalf("blank DisplayName should be allowed, got errors: %v", got.Errors)
	}
	if got.DisplayName != "" {
		t.Errorf("blank DisplayName should stay empty, got %q", got.DisplayName)
	}
}
