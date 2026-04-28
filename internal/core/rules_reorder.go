// rules_reorder.go — Phase 5.4: atomic `(*RulesService).Reorder`.
//
// What it does
//   Replaces the current rule ordering with the permutation given by
//   `names`. The input must be exactly a permutation of the existing
//   rule names — same multiset, no missing, no extra, no duplicates.
//   On success, `cfg.Rules` is rebuilt in the new order and persisted
//   in a single load → mutate → save round-trip.
//
// Why a dedicated method (vs. Remove + Add loop)
//   Doing it as N×Remove + N×Add would:
//     1. Lose every field on every rule (caller would have to read
//        each rule's struct first).
//     2. Persist N intermediate broken states observable to concurrent
//        readers (one save per Add/Remove).
//     3. Be O(n²) instead of O(n) due to slice splicing on each Remove.
//   In-place reorder solves all three: build name→rule map once, walk
//   `names` once writing the new slice, persist once.
//
// Why identity reorder (same order in, same order out) is allowed
//   Unlike Rename's same-name no-op (which strongly indicates a UI
//   click handler that didn't diff), an identity Reorder can legitimately
//   come from a drag-and-drop UI where the user dropped a rule back in
//   its original slot. We persist it (cheap) rather than reject; tests
//   assert exactly-one save still occurs.
//
// Why ErrRuleReorderMismatch (vs. reusing ErrRuleNotFound / ErrRuleDuplicate)
//   The "this list is not a permutation" failure is a single semantic
//   condition with several possible causes (missing/extra/dup). Caller
//   gets one code to handle, with the offending name(s) attached as
//   context. Reusing NotFound/Duplicate would force the caller to
//   distinguish "Reorder rejected" from "individual lookup failed".
//
// Spec source: roadmap-phases.md §PHASE 5 — P5.4.

package core

import (
	"fmt"
	"strings"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// Reorder atomically replaces the order of rules in `cfg.Rules` with
// the permutation specified by `names`. Each entry in `names` must
// match an existing rule's `Name` field exactly (no trimming applied
// — names are compared verbatim so a stray space is a real mismatch,
// not silently coerced). The input must contain every current rule
// exactly once.
//
// Error envelope:
//
//   - nil names slice on a non-empty config → ErrRuleReorderMismatch
//     (interpreted as "you forgot to pass the names")
//   - len(names) != len(cfg.Rules)          → ErrRuleReorderMismatch
//   - counts in context
//   - duplicate name in `names`             → ErrRuleReorderMismatch
//   - duplicateName ctx
//   - name in `names` not in current rules  → ErrRuleReorderMismatch
//   - missingName ctx
//   - load/save IO failure                  → ErrConfigOpen / ErrConfigEncode
//
// Empty `names` against an empty rules slice is a valid no-op success
// (still persisted once for consistency with identity-reorder semantics).
func (s *RulesService) Reorder(names []string) errtrace.Result[Unit] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[Unit](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config").
			WithContext("nameCount", fmt.Sprintf("%d", len(names))))
	}
	if len(names) != len(cfg.Rules) {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrRuleReorderMismatch,
			fmt.Sprintf("core.RulesService.Reorder: input has %d names, config has %d rules",
				len(names), len(cfg.Rules))).
			WithContext("inputCount", fmt.Sprintf("%d", len(names))).
			WithContext("ruleCount", fmt.Sprintf("%d", len(cfg.Rules))))
	}
	// Build name→rule map from current config (O(n)). Duplicate names
	// in storage shouldn't happen (Add enforces uniqueness) but if they
	// somehow did, the last one wins — we don't try to repair corrupt
	// state from a Reorder call.
	byName := make(map[string]config.Rule, len(cfg.Rules))
	for _, r := range cfg.Rules {
		byName[r.Name] = r
	}
	// Single pass over `names`: detect duplicates AND build new slice
	// AND verify each name exists in the current set.
	seen := make(map[string]struct{}, len(names))
	newRules := make([]config.Rule, 0, len(names))
	for i, n := range names {
		if _, dup := seen[n]; dup {
			return errtrace.Err[Unit](errtrace.NewCoded(
				errtrace.ErrRuleReorderMismatch,
				fmt.Sprintf("core.RulesService.Reorder: duplicate name %q at index %d", n, i)).
				WithContext("duplicateName", n).
				WithContext("index", fmt.Sprintf("%d", i)))
		}
		seen[n] = struct{}{}
		r, ok := byName[n]
		if !ok {
			return errtrace.Err[Unit](errtrace.NewCoded(
				errtrace.ErrRuleReorderMismatch,
				fmt.Sprintf("core.RulesService.Reorder: name %q at index %d does not match any current rule", n, i)).
				WithContext("missingName", n).
				WithContext("index", fmt.Sprintf("%d", i)))
		}
		newRules = append(newRules, r)
	}
	// Counts already match and `seen` has len(names) unique entries
	// that are all present in `byName` — by pigeonhole this is a
	// permutation, no extra check needed.
	cfg.Rules = newRules
	if err := s.saveCfg(cfg); err != nil {
		return errtrace.Err[Unit](errtrace.WrapCode(err,
			errtrace.ErrConfigEncode, "save config").
			WithContext("nameCount", fmt.Sprintf("%d", len(names))).
			WithContext("firstName", firstOrEmpty(names)))
	}
	return errtrace.Ok(Unit{})
}

// firstOrEmpty returns names[0] or "" — used in error context so a
// save-error trace always carries a sample of what we tried to write,
// without dumping the whole slice into the log.
func firstOrEmpty(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimSpace(names[0])
}
