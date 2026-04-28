// rules_preview.go — F-23 contract surface for the rule editor's
// "Test rule → open in browser" preview button.
//
// Why a dedicated helper (vs. building `OpenUrlSpec` inline at the
// future button's click handler):
//
//  1. **Calling-convention pin.** The Tools spec
//     (`spec/21-app/02-features/06-tools/99-consistency-report.md`
//     §13, OI-3) requires that previews fired from the rule editor
//     route through `core.Tools.OpenUrl(ctx, OpenUrlSpec{
//     Origin: OriginManual, RuleName: <currentRuleName>})` — i.e.
//     the launch reads as "manual" in audit logs (the user clicked
//     a preview button, not an automated rule match) yet still
//     carries the rule name so `OpenedUrls.RuleName` is populated.
//     The combination is non-obvious — `Origin: OriginRule` would
//     be the natural reading — so encoding it in a helper with a
//     single test is cheaper than re-deriving it at every future
//     callsite.
//
//  2. **Headless-testable.** The button itself is Fyne-canvas
//     bound (deferred to Slice #118e per the Phase 1 plan), but
//     the *shape* of the spec it must build is a pure data
//     contract with zero UI dependency. Pulling the contract into
//     `internal/core/` lets us pin it today with a 🟢 sandbox-
//     safe test (`rules_preview_test.go`) instead of waiting for
//     the canvas harness.
//
//  3. **Audit-trail invariant.** A future refactor that "tidies
//     up" the manual-with-rule-name combination — e.g. by
//     collapsing it into a new `OriginManualRule` enum value, or
//     by stripping the rule name when origin is manual — must
//     change this helper, which forces an explicit diff to the
//     Tools spec instead of silent drift.
//
// Spec:
//   - spec/21-app/02-features/03-rules/97-acceptance-criteria.md (F-23)
//   - spec/21-app/02-features/06-tools/99-consistency-report.md §13 (OI-3 closure)
package core

// OpenUrlSpecForTestRulePreview builds the canonical OpenUrlSpec
// that the rule-editor "Test rule → open in browser" preview button
// must hand to `Tools.OpenUrl`.
//
// The combination is intentional and locked by `F-23`:
//
//   - Origin = OriginManual — the launch is user-initiated from the
//     rule editor (a manual "test this rule" click), not an
//     automated rule match firing on incoming mail. Audit logs and
//     the Recent Opens list filter on origin; mis-tagging this as
//     OriginRule would falsify the "rule matched a real email" count.
//
//   - RuleName = ruleName — the preview is *about* a specific rule,
//     so the audit row's `RuleName` column should still resolve to
//     the rule under test. Leaving it empty would orphan the
//     OpenedUrls row from the rule that triggered it.
//
// `Alias` and `EmailId` are deliberately left zero — the preview is
// not bound to any specific account or message. The existing
// `Tools.OpenUrl` validation accepts both as zero-value (see
// `internal/core/tools.go` validation chain).
//
// Callers must NOT post-process the returned struct beyond the
// documented zero-value fields; doing so defeats the contract pin
// this helper exists to enforce.
func OpenUrlSpecForTestRulePreview(ruleName, url string) OpenUrlSpec {
	return OpenUrlSpec{
		Url:      url,
		Origin:   OriginManual,
		RuleName: ruleName,
	}
}
