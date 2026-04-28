// rules_rename.go — Phase 5.3: atomic `(*RulesService).Rename`.
//
// What it does
//   Renames an existing rule from `oldName` to `newName` in a single
//   load → mutate → save round-trip. "Atomic" here means: the read,
//   the mutation, and the write all happen against one in-memory
//   `*config.Config` snapshot, then `saveCfg` persists the whole file.
//   No partial state is ever observable on disk — either both the
//   old name disappears AND the new name appears, or neither does
//   (saveCfg failure leaves the prior file intact, since
//   `config.Save` writes to a temp file + rename).
//
// Why a dedicated method (vs. Remove + Add)
//   Doing it as Remove + Add would:
//     1. Lose every field on the rule (caller would need to read it
//        first, replay all fields through RuleInput, then re-add).
//     2. Briefly persist a config with the old rule deleted but the
//        new one not yet inserted — observable via concurrent UI
//        reads and irreversible if the second save fails.
//     3. Reset position in the rules slice (Add appends; rename
//        should preserve user-curated ordering).
//   In-place rename solves all three: copy struct, change Name field,
//   persist.
//
// Why a dedicated ER-RUL-21306 for same-name no-op (vs. silent OK)
//   A "rename A → A" call indicates a caller bug (probably a UI
//   click handler that didn't diff old/new before submitting). Silent
//   success would mask the bug; a coded error surfaces it during
//   testing without crashing production paths. Caller can ignore the
//   code with a one-line check if they want lenient semantics.
//
// Spec source: roadmap-phases.md §PHASE 5 — P5.3.

package core

import (
	"fmt"
	"strings"

	"github.com/lovable/email-read/internal/errtrace"
)

// Rename atomically changes the rule name from oldName to newName.
// Both names are trimmed before validation; whitespace-only inputs
// are rejected as invalid. Error envelope:
//
//   - empty oldName / newName    → ErrCoreInvalidArgument
//   - oldName == newName (after trim) → ErrRuleRenameNoop
//     (caller bug — UI should diff
//     before submitting)
//   - no rule with oldName       → ErrRuleNotFound + name context
//   - newName already taken      → ErrRuleDuplicate + both names ctx
//   - load/save IO failure       → ErrConfigOpen / ErrConfigEncode
//     (existing pattern from Add/Remove)
//
// On success returns Unit{}; the rule's other fields (Enabled,
// FromRegex, SubjectRegex, BodyRegex, UrlRegex) and its position in
// the rules slice are preserved verbatim.
func (s *RulesService) Rename(oldName, newName string) errtrace.Result[Unit] {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.RulesService.Rename: oldName is required"))
	}
	if newName == "" {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"core.RulesService.Rename: newName is required").
			WithContext("oldName", oldName))
	}
	if oldName == newName {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrRuleRenameNoop,
			fmt.Sprintf("core.RulesService.Rename: oldName == newName (%q)", oldName)).
			WithContext("name", oldName))
	}
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[Unit](errtrace.WrapCode(err,
			errtrace.ErrConfigOpen, "load config").
			WithContext("oldName", oldName).
			WithContext("newName", newName))
	}
	// Single pass: locate oldName's index AND check newName collision
	// in one walk so a 1000-rule config still costs O(n), not O(2n).
	oldIdx := -1
	for i := range cfg.Rules {
		switch cfg.Rules[i].Name {
		case oldName:
			oldIdx = i
		case newName:
			return errtrace.Err[Unit](errtrace.NewCoded(
				errtrace.ErrRuleDuplicate,
				fmt.Sprintf("core.RulesService.Rename: a rule named %q already exists", newName)).
				WithContext("oldName", oldName).
				WithContext("newName", newName))
		}
	}
	if oldIdx < 0 {
		return errtrace.Err[Unit](errtrace.NewCoded(
			errtrace.ErrRuleNotFound,
			fmt.Sprintf("core.RulesService.Rename: no rule with name %q", oldName)).
			WithContext("oldName", oldName).
			WithContext("newName", newName))
	}
	cfg.Rules[oldIdx].Name = newName
	if err := s.saveCfg(cfg); err != nil {
		return errtrace.Err[Unit](errtrace.WrapCode(err,
			errtrace.ErrConfigEncode, "save config").
			WithContext("oldName", oldName).
			WithContext("newName", newName))
	}
	return errtrace.Ok(Unit{})
}
