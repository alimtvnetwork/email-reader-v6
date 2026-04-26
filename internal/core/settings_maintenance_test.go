// settings_maintenance_test.go pins the §5 maintenance-knob validation
// (spec/23-app-database/04). All four rows share ER-SET-21778 per the
// spec table; we verify each boundary + a happy path.
package core

import (
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

func baseValidInput() SettingsInput {
	in := DefaultSettingsInput()
	return in
}

func Test_ValidateMaintenanceKnobs_Defaults(t *testing.T) {
	in := baseValidInput()
	if err := validateInput(in); err != nil {
		t.Fatalf("DefaultSettingsInput should validate: %v", err)
	}
}

func Test_ValidateMaintenanceKnobs_Bounds(t *testing.T) {
	cases := []struct {
		name string
		mut  func(in *SettingsInput)
	}{
		{"weekday<Sunday", func(in *SettingsInput) { in.WeeklyVacuumOn = -1 }},
		{"weekday>Saturday", func(in *SettingsInput) { in.WeeklyVacuumOn = 7 }},
		{"hour>23", func(in *SettingsInput) { in.WeeklyVacuumHourLocal = 24 }},
		{"wal=0", func(in *SettingsInput) { in.WalCheckpointHours = 0 }},
		{"wal>168", func(in *SettingsInput) { in.WalCheckpointHours = 169 }},
		{"batch<100", func(in *SettingsInput) { in.PruneBatchSize = 99 }},
		{"batch>50000", func(in *SettingsInput) { in.PruneBatchSize = 50001 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseValidInput()
			tc.mut(&in)
			err := validateMaintenanceKnobs(in)
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
			var coded *errtrace.CodedError
			if !errors.As(err, &coded) || coded.Code() != errtrace.ErrSettingsPersist {
				t.Fatalf("%s: expected ER-SET-21778 (ErrSettingsPersist), got %v", tc.name, err)
			}
		})
	}
}

func Test_ValidateMaintenanceKnobs_HappyPath(t *testing.T) {
	in := baseValidInput()
	in.WeeklyVacuumOn = time.Wednesday
	in.WeeklyVacuumHourLocal = 4
	in.WalCheckpointHours = 12
	in.PruneBatchSize = 2000
	if err := validateMaintenanceKnobs(in); err != nil {
		t.Fatalf("happy path failed: %v", err)
	}
}

func Test_NormalizeInput_FillsZeroMaintenanceKnobs(t *testing.T) {
	in := SettingsInput{
		PollSeconds:           3,
		Theme:                 ThemeDark,
		OpenUrlAllowedSchemes: []string{"https"},
	}
	out := normalizeInput(in)
	if out.WalCheckpointHours == 0 {
		t.Errorf("WalCheckpointHours should be defaulted, got 0")
	}
	if out.PruneBatchSize == 0 {
		t.Errorf("PruneBatchSize should be defaulted, got 0")
	}
}

func Test_ParseWeekday_Roundtrip(t *testing.T) {
	for w := time.Sunday; w <= time.Saturday; w++ {
		got, ok := ParseWeekday(w.String())
		if !ok || got != w {
			t.Errorf("ParseWeekday(%q)=%v,%v want %v,true", w.String(), got, ok, w)
		}
	}
	if _, ok := ParseWeekday("nonsense"); ok {
		t.Errorf("ParseWeekday(nonsense) should return false")
	}
	if _, ok := ParseWeekday(""); ok {
		t.Errorf("ParseWeekday(\"\") should return false")
	}
}
