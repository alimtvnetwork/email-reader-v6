// account_presets.go — pure-Go list of IMAP provider presets shown in the
// Add Account form's "Provider" dropdown. Picking a preset auto-fills the
// host / port / TLS fields so users skip the Autodiscover step entirely.
//
// No fyne imports here on purpose — the preset table and ApplyPreset are
// unit-tested headlessly in account_presets_test.go.
package views

// AccountPreset is one entry in the Provider dropdown.
type AccountPreset struct {
	Label  string // human-readable name shown in the Select widget
	Host   string // IMAP host; empty for "Custom" (user fills manually)
	Port   int    // IMAP port; 0 for "Custom"
	UseTLS bool
}

// AccountPresets is the canonical, ordered list of providers the form
// offers. "Custom" is sentinel: selecting it leaves the host/port/TLS
// fields untouched so the user can type their own values.
var AccountPresets = []AccountPreset{
	{Label: "Custom (manual)", Host: "", Port: 0, UseTLS: false},
	{Label: "Gmail", Host: "imap.gmail.com", Port: 993, UseTLS: true},
	{Label: "Outlook / Office 365", Host: "outlook.office365.com", Port: 993, UseTLS: true},
	{Label: "Yahoo Mail", Host: "imap.mail.yahoo.com", Port: 993, UseTLS: true},
	{Label: "iCloud Mail", Host: "imap.mail.me.com", Port: 993, UseTLS: true},
	{Label: "FastMail", Host: "imap.fastmail.com", Port: 993, UseTLS: true},
	{Label: "Zoho Mail", Host: "imap.zoho.com", Port: 993, UseTLS: true},
}

// PresetLabels returns just the labels in their canonical order — handy
// for feeding widget.NewSelect without exposing the table directly.
func PresetLabels() []string {
	out := make([]string, len(AccountPresets))
	for i, p := range AccountPresets {
		out[i] = p.Label
	}
	return out
}

// FindPreset returns the preset with the given label, or (zero, false)
// if no match. Case-sensitive — labels come from our own table.
func FindPreset(label string) (AccountPreset, bool) {
	for _, p := range AccountPresets {
		if p.Label == label {
			return p, true
		}
	}
	return AccountPreset{}, false
}

// IsCustomPreset reports whether the preset is the "Custom (manual)"
// sentinel — used by the form to skip auto-filling host/port/TLS.
func IsCustomPreset(p AccountPreset) bool {
	return p.Host == "" && p.Port == 0
}
