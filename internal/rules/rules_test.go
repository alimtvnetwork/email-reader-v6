package rules

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/mailclient"
)

func TestEvaluateMatchesAndDedupes(t *testing.T) {
	e, err := New([]config.Rule{
		{
			Name: "magic", Enabled: true,
			FromRegex:    "noreply@.*",
			SubjectRegex: "(?i)sign.?in",
			UrlRegex:     `https://app\.example\.com/auth\?token=[A-Za-z0-9_-]+`,
		},
		{Name: "disabled", Enabled: false, UrlRegex: ".*"},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m := &mailclient.Message{
		From:     "noreply@example.com",
		Subject:  "Sign in to your account",
		BodyText: "Click https://app.example.com/auth?token=ABC and again https://app.example.com/auth?token=ABC and https://app.example.com/auth?token=XYZ",
	}
	got := e.Evaluate(m)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(got), got)
	}
	for _, g := range got {
		if g.RuleName != "magic" {
			t.Errorf("unexpected rule: %s", g.RuleName)
		}
	}
}

func TestEvaluateNoFromMatch(t *testing.T) {
	e, _ := New([]config.Rule{{
		Name: "r", Enabled: true,
		FromRegex: "boss@.*", UrlRegex: `https?://\S+`,
	}})
	m := &mailclient.Message{From: "spam@x.com", BodyText: "https://x.com/a"}
	if got := e.Evaluate(m); len(got) != 0 {
		t.Errorf("expected no matches, got %+v", got)
	}
}

func TestEmptyPatternMatchesAny(t *testing.T) {
	e, _ := New([]config.Rule{{
		Name: "r", Enabled: true, UrlRegex: `https?://\S+`,
	}})
	m := &mailclient.Message{BodyText: "go https://x.com now"}
	if got := e.Evaluate(m); len(got) != 1 || got[0].Url != "https://x.com" {
		t.Errorf("got %+v", got)
	}
}
