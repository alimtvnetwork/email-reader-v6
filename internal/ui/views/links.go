// links.go — pure-Go URL extraction used by the Emails view. Lives outside
// emails.go (which is fyne-tagged) so it stays unit-testable on headless CI.
package views

import "regexp"

var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// ExtractUrls returns unique http(s) URLs found in the given text in
// first-seen order. Trailing punctuation is stripped. Empty input ⇒ nil.
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
