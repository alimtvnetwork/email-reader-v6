// codes_yaml_ranges_test.go — structural integrity of the YAML
// registry's *block* metadata.
//
// The existing P1.4 registry tests (`codes_registry_test.go`) prove
// that every CONST in `codes.go` has a row in `codes.yaml` and vice
// versa. They do NOT prove that the *block-level* metadata makes
// sense — a developer could:
//
//   - Reuse a prefix across two blocks (ER-EML in two places)
//   - Declare overlapping numeric ranges ([22000,22099] and
//     [22050,22150]) so two unrelated blocks stomp on each other's
//     reservation
//   - Declare a code OUTSIDE its own block's range (ER-EML-22001
//     under a block whose range is [21000,21099]) — the symptom is
//     "code looks orphaned" months later when readers grep by range
//
// All three failure modes survived P1.4 because the registry test
// only checks the (const, code) tuple — not the block envelope around
// it. This file closes the gap. It runs at `go test` time alongside
// the existing registry guards so any drift is caught BEFORE the
// merge that introduces it.
//
// We re-parse `codes.yaml` rather than re-using the codegen's
// `flatten()` output because:
//
//   - flatten() discards the per-block range (it's "informational"
//     per the codegen header). To assert on it we need the raw struct.
//   - Keeping the range-validation logic in the test (not the
//     codegen) means a range bug never silently lands in
//     `codes_gen.go` — the failure surface is the test, where it
//     belongs, not a freshly-regenerated file.
//
// Schema: mirrors codegen/main.go's `codesYAML` minus the
// description fields (we never assert on prose).
package errtrace

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type yamlBlock struct {
	Name   string     `yaml:"name"`
	Prefix string     `yaml:"prefix"`
	Range  [2]int     `yaml:"range"`
	Codes  []yamlCode `yaml:"codes"`
}

type yamlCode struct {
	Const string `yaml:"const"`
	Code  string `yaml:"code"`
}

type yamlDoc struct {
	Blocks []yamlBlock `yaml:"blocks"`
}

// loadYAML reads codes.yaml from disk. Path is package-relative
// because `go test` runs with cwd set to the package directory.
func loadYAML(t *testing.T) yamlDoc {
	t.Helper()
	raw, err := os.ReadFile("codes.yaml")
	if err != nil {
		t.Fatalf("read codes.yaml: %v", err)
	}
	var doc yamlDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse codes.yaml: %v", err)
	}
	if len(doc.Blocks) == 0 {
		t.Fatal("codes.yaml has zero blocks — parse likely silently failed")
	}
	return doc
}

// TestYAML_PrefixesUnique catches "ER-EML defined in two blocks"
// drift. Two blocks sharing a prefix would let a code visually
// belong to the wrong group when grouped by prefix in UI / docs.
func TestYAML_PrefixesUnique(t *testing.T) {
	doc := loadYAML(t)
	seen := map[string]string{} // prefix → first block name
	for _, b := range doc.Blocks {
		if prev, dup := seen[b.Prefix]; dup {
			t.Errorf("prefix %q is shared by blocks %q and %q — every prefix must own exactly one block",
				b.Prefix, prev, b.Name)
		}
		seen[b.Prefix] = b.Name
	}
}

// TestYAML_BlockNamesUnique mirrors the prefix check for human-
// readable block names (e.g. two "Emails" blocks would confuse the
// error-registry doc reader even if their prefixes differed).
func TestYAML_BlockNamesUnique(t *testing.T) {
	doc := loadYAML(t)
	seen := map[string]bool{}
	for _, b := range doc.Blocks {
		if seen[b.Name] {
			t.Errorf("block name %q appears more than once — must be unique", b.Name)
		}
		seen[b.Name] = true
	}
}

// TestYAML_RangesValid catches degenerate / inverted ranges
// ([22099, 22000]) before they propagate into the spec doc and
// confuse readers searching by numeric range.
func TestYAML_RangesValid(t *testing.T) {
	doc := loadYAML(t)
	for _, b := range doc.Blocks {
		lo, hi := b.Range[0], b.Range[1]
		if lo == 0 && hi == 0 {
			t.Errorf("block %q (%s) is missing `range:` (got [0, 0])", b.Name, b.Prefix)
		}
		if lo > hi {
			t.Errorf("block %q (%s) range inverted: [%d, %d]", b.Name, b.Prefix, lo, hi)
		}
	}
}

// TestYAML_CodesWithinDeclaredRange catches "ER-EML-22001 sitting in
// a block whose range is [21000, 21099]" drift. The numeric tail of
// every code MUST fall within its block's declared range — otherwise
// the range is a lie and grep-by-range loses meaning.
func TestYAML_CodesWithinDeclaredRange(t *testing.T) {
	doc := loadYAML(t)
	for _, b := range doc.Blocks {
		lo, hi := b.Range[0], b.Range[1]
		for _, c := range b.Codes {
			n, ok := numericTail(c.Code)
			if !ok {
				t.Errorf("block %q: code %q has no numeric tail (expected ER-XXX-NNNNN)",
					b.Name, c.Code)
				continue
			}
			if n < lo || n > hi {
				t.Errorf("block %q: code %s = %d falls outside declared range [%d, %d] — "+
					"either move the code or expand the range",
					b.Name, c.Const, n, lo, hi)
			}
		}
	}
}

// TestYAML_RangesNonOverlapping catches "[22000, 22099] vs [22050,
// 22150]" drift. Two blocks reserving the same number is a recipe
// for a future merge conflict where two PRs claim the same code.
//
// EXCEPTIONS — historical sub-block carve-outs. The Settings (21770–
// 21789) and Tools (21750–21769) blocks were carved INSIDE the Core
// (21700–21799) range before this guard existed (per codes.yaml
// header note). They are intentional sub-blocks, not collisions.
// Flagging them as overlaps would force a churn-rename of every
// ER-COR / ER-SET / ER-TLS const in the codebase. The exception list
// is closed — new blocks must use a fresh range.
//
// The Unknown sentinel (21999, 21999) sits inside ER-UI's range
// (21900–21999) by design; it's a single-code reserved fallback.
func TestYAML_RangesNonOverlapping(t *testing.T) {
	doc := loadYAML(t)
	type rng struct {
		name   string
		prefix string
		lo, hi int
	}
	ranges := make([]rng, 0, len(doc.Blocks))
	for _, b := range doc.Blocks {
		ranges = append(ranges, rng{b.Name, b.Prefix, b.Range[0], b.Range[1]})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })

	allowed := map[string]bool{
		// Core ⊃ Tools (carve-out).
		"ER-COR|ER-TLS": true,
		// Core ⊃ Settings (carve-out).
		"ER-COR|ER-SET": true,
		// UI ⊃ Unknown (single-code sentinel).
		"ER-UI|ER-UNKNOWN": true,
	}
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			a, b := ranges[i], ranges[j]
			if a.hi < b.lo {
				break // sorted by lo — no later block can overlap a
			}
			key := a.prefix + "|" + b.prefix
			if allowed[key] {
				continue
			}
			t.Errorf("range overlap: %q (%s) [%d,%d] vs %q (%s) [%d,%d] — "+
				"new blocks must use a fresh non-overlapping range; "+
				"if this is an intentional carve-out, add it to the allow-list",
				a.name, a.prefix, a.lo, a.hi, b.name, b.prefix, b.lo, b.hi)
		}
	}
}

// numericTail extracts the trailing integer from "ER-XXX-NNNNN".
// Returns (0, false) when the format doesn't match — caller treats
// that as a structural error.
func numericTail(code string) (int, bool) {
	idx := strings.LastIndex(code, "-")
	if idx <= 0 || idx == len(code)-1 {
		return 0, false
	}
	n, err := strconv.Atoi(code[idx+1:])
	if err != nil {
		return 0, false
	}
	return n, true
}
