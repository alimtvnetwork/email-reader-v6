// links.go — pure-Go URL extraction used by the Emails view to render
// clickable buttons. Lives outside emails.go (which is fyne-tagged) so the
// regex + dedupe logic stays unit-testable on headless CI.
package views

import "regexp"

// urlPattern matches http(s) URLs in plain text or HTML bodies. Trailing
// punctuation common in prose (`.`, `,`, `)`, `]`, `>`, `"`, `'`) is
// trimmed by ExtractUrls so the link the user clicks is canonical.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// ExtractUrls returns unique http(s) URLs found in the given text in
// first-seen order. Trailing punctuation is stripped. Empty input ⇒ nil.
//
// This is intentionally simpler than the rules-engine matcher: the Emails
// view only needs a "links found in this email" preview, not full rule
// evaluation. A user who wants rule-driven actions runs `read` instead.
func ExtractUrls(text string) []string {
	if text == "" {
		return nil
	}
	raw := urlPattern.FindAllString(text, -1)
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, u := range raw {
		u = trimTrailingPunct(u)
		if u == "" {
			continue
		}
		if _, dup := seen[u]; dup {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

// trimTrailingPunct strips characters that are almost never part of a real
// URL when the URL appears at the end of a sentence or inside parentheses.
func trimTrailingPunct(u string) string {
	for len(u) > 0 {
		switch u[len(u)-1] {
		case '.', ',', ';', ':', ')', ']', '}', '>', '"', '\'', '!', '?':
			u = u[:len(u)-1]
		default:
			return u
		}
	}
	return u
}
