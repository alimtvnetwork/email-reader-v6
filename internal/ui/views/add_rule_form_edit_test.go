package views

import "testing"

func TestRuleSubmitLabel(t *testing.T) {
	if got := ruleSubmitLabel(false); got != "Save rule" {
		t.Fatalf("add mode label = %q, want %q", got, "Save rule")
	}
	if got := ruleSubmitLabel(true); got != "Update rule" {
		t.Fatalf("edit mode label = %q, want %q", got, "Update rule")
	}
}

func TestRuleClearLabel(t *testing.T) {
	if got := ruleClearLabel(false); got != "Clear" {
		t.Fatalf("add mode label = %q, want %q", got, "Clear")
	}
	if got := ruleClearLabel(true); got != "Reset" {
		t.Fatalf("edit mode label = %q, want %q", got, "Reset")
	}
}

// TestValidateRuleForm_EditModeShape verifies that values matching what
// applyInitialRuleToEntries would push (existing rule data) round-trip
// through validation cleanly — i.e. an Edit submit on an unmodified form
// does not synthesise spurious errors.
func TestValidateRuleForm_EditModeShape(t *testing.T) {
	in := RuleFormInput{
		Name:         "magic-link",
		UrlRegex:     `https://app\.example\.com/auth\?token=\S+`,
		FromRegex:    `noreply@example\.com`,
		SubjectRegex: `(?i)sign.?in`,
		BodyRegex:    "",
		Enabled:      true,
	}
	v := ValidateRuleForm(in)
	if !v.Valid {
		t.Fatalf("expected valid, got errors: %v", v.Errors)
	}
	if v.Name != in.Name || v.UrlRegex != in.UrlRegex {
		t.Fatalf("round-trip mismatch: %+v", v)
	}
	if !v.Enabled {
		t.Fatal("Enabled should round-trip true")
	}
}
