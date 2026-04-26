// account_form_edit_test.go covers AllowBlankPassword behaviour added
// for Edit Account mode: blank password should validate, non-blank
// should still be sanitized into the result.
package views

import "testing"

func TestValidateAccountForm_AllowBlankPasswordInEditMode(t *testing.T) {
	got := ValidateAccountForm(AccountFormInput{
		Alias:              "a",
		Email:              "u@x.com",
		AllowBlankPassword: true,
	})
	if !got.Valid {
		t.Fatalf("blank password should be allowed in edit mode, got errors: %v", got.Errors)
	}
	if got.Password != "" {
		t.Errorf("blank password should stay blank, got %q", got.Password)
	}
}

func TestValidateAccountForm_BlankPasswordStillRejectedInAddMode(t *testing.T) {
	got := ValidateAccountForm(AccountFormInput{
		Alias: "a", Email: "u@x.com",
		// AllowBlankPassword left false (Add mode default)
	})
	if got.Valid {
		t.Fatal("Add mode must require a password")
	}
	found := false
	for _, e := range got.Errors {
		if e == "password is required" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'password is required' error, got %v", got.Errors)
	}
}
