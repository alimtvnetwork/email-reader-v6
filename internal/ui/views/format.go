// format.go holds tiny pure-Go helpers used by the views. No fyne imports
// here so they can be unit-tested on headless CI without cgo / OpenGL.
package views

import (
	"fmt"

	"github.com/lovable/email-read/internal/core"
)

// FormatEmailsValue picks the best label for the "Emails stored" stat card.
// When an alias is selected we surface the alias-scoped count first so the
// dashboard reflects the user's current focus.
func FormatEmailsValue(s core.DashboardStats) string {
	if s.Alias != "" {
		return fmt.Sprintf("%d (%d total)", s.EmailsForAlias, s.EmailsTotal)
	}
	return fmt.Sprintf("%d", s.EmailsTotal)
}
