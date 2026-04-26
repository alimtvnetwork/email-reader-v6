package views

import "testing"

func TestPresetLabels_OrderAndCount(t *testing.T) {
	labels := PresetLabels()
	if len(labels) != len(AccountPresets) {
		t.Fatalf("PresetLabels length = %d, want %d", len(labels), len(AccountPresets))
	}
	if labels[0] != "Custom (manual)" {
		t.Errorf("first preset must be Custom sentinel, got %q", labels[0])
	}
}

func TestFindPreset_Known(t *testing.T) {
	p, ok := FindPreset("Gmail")
	if !ok {
		t.Fatal("Gmail preset not found")
	}
	if p.Host != "imap.gmail.com" || p.Port != 993 || !p.UseTLS {
		t.Errorf("Gmail preset wrong: %+v", p)
	}
}

func TestFindPreset_Unknown(t *testing.T) {
	if _, ok := FindPreset("Nope"); ok {
		t.Error("expected unknown preset to miss")
	}
}

func TestIsCustomPreset(t *testing.T) {
	custom, _ := FindPreset("Custom (manual)")
	if !IsCustomPreset(custom) {
		t.Error("Custom (manual) must be the sentinel")
	}
	gmail, _ := FindPreset("Gmail")
	if IsCustomPreset(gmail) {
		t.Error("Gmail must not be the sentinel")
	}
}
