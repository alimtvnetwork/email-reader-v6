// rules_preview_test.go — F-23 contract pin for
// `OpenUrlSpecForTestRulePreview`.
//
// This is the headless half of `RulesVM_TestRule_OpenUrl_OriginManualWithRuleName`
// (the row-name in `spec/21-app/02-features/03-rules/97-acceptance-criteria.md`
// F-23). The Fyne-bound half — wiring an actual button into the
// rule editor and asserting the click-handler hands the produced
// spec to `Tools.OpenUrl` — lives behind Slice #118e's deferred
// canvas harness. Until then, this test asserts the only invariant
// that does not need a canvas: the spec shape is exactly
// `(Origin=OriginManual, RuleName=<input>)`.
//
// The four sub-tests cover, in order, the spec § noted on each:
//
//   1. Happy-path — Origin must be OriginManual (NOT OriginRule),
//      RuleName must round-trip the input. The headline assertion
//      from F-23 itself.
//   2. Empty rule name — passes through unchanged. The helper does
//      not invent a placeholder; an empty rule name signals a UI
//      bug that the caller must surface, not paper over.
//   3. Empty URL — passes through unchanged. Validation belongs to
//      `Tools.OpenUrl.validateUrl`, not to this shape helper. Two
//      validators in two layers would be a maintenance trap.
//   4. Alias / EmailId — must be zero. The preview is account-
//      and message-agnostic; if a future caller starts populating
//      these fields, the contract pin breaks loudly.
package core

import "testing"

func TestOpenUrlSpecForTestRulePreview_OriginManualWithRuleName(t *testing.T) {
	got := OpenUrlSpecForTestRulePreview("lovable-magic-link",
		"https://app.example.com/auth?token=abc")

	if got.Origin != OriginManual {
		t.Errorf("F-23: Origin = %q, want %q (OriginManual). "+
			"A preview click is user-initiated; tagging it as "+
			"OriginRule would falsify the audit log's "+
			"rule-match count.", got.Origin, OriginManual)
	}
	if got.RuleName != "lovable-magic-link" {
		t.Errorf("F-23: RuleName = %q, want %q. The OpenedUrls "+
			"audit row must still resolve to the rule under test.",
			got.RuleName, "lovable-magic-link")
	}
	if got.Url != "https://app.example.com/auth?token=abc" {
		t.Errorf("F-23: Url not round-tripped: got %q", got.Url)
	}
}

func TestOpenUrlSpecForTestRulePreview_EmptyRuleName_PassesThrough(t *testing.T) {
	got := OpenUrlSpecForTestRulePreview("", "https://x.test/p")
	if got.RuleName != "" {
		t.Errorf("RuleName = %q, want empty. The helper must not "+
			"invent a placeholder — an empty name signals an "+
			"upstream UI bug the caller must surface.", got.RuleName)
	}
	if got.Origin != OriginManual {
		t.Errorf("Origin = %q, want OriginManual even when rule "+
			"name is empty.", got.Origin)
	}
}

func TestOpenUrlSpecForTestRulePreview_EmptyUrl_PassesThrough(t *testing.T) {
	got := OpenUrlSpecForTestRulePreview("rule-x", "")
	if got.Url != "" {
		t.Errorf("Url = %q, want empty. URL validation belongs to "+
			"Tools.OpenUrl.validateUrl — duplicating it here "+
			"would split the validation contract.", got.Url)
	}
	if got.RuleName != "rule-x" {
		t.Errorf("RuleName not round-tripped: got %q", got.RuleName)
	}
}

func TestOpenUrlSpecForTestRulePreview_AliasAndEmailId_AreZero(t *testing.T) {
	got := OpenUrlSpecForTestRulePreview("rule-y", "https://y.test/")
	if got.Alias != "" {
		t.Errorf("Alias = %q, want empty. Preview is account-"+
			"agnostic.", got.Alias)
	}
	if got.EmailId != 0 {
		t.Errorf("EmailId = %d, want 0. Preview is message-"+
			"agnostic.", got.EmailId)
	}
}
