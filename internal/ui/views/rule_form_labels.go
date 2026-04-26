// rule_form_labels.go — pure-Go (no build tag) so the tag-free test
// suite can verify Add vs Edit button labels without a Fyne toolchain.
package views

// ruleSubmitLabel returns the primary button label for the Add/Edit
// rule form.
func ruleSubmitLabel(editing bool) string {
	if editing {
		return "Update rule"
	}
	return "Save rule"
}

// ruleClearLabel returns the secondary button label — "Reset" in edit
// mode (restores the original values), "Clear" in add mode.
func ruleClearLabel(editing bool) string {
	if editing {
		return "Reset"
	}
	return "Clear"
}
