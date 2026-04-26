// tools_test.go covers OpenUrl validation, redaction, dedup, and the
// audit/launch ordering. Uses in-memory stubs for browser + store.
package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

type fakeBrowser struct {
	mu       sync.Mutex
	opened   []string
	openErr  error
	pathErr  error
	pathStub string
}

func (f *fakeBrowser) Open(u string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.openErr != nil {
		return f.openErr
	}
	f.opened = append(f.opened, u)
	return nil
}

func (f *fakeBrowser) Path() (string, error) {
	if f.pathErr != nil {
		return "", f.pathErr
	}
	if f.pathStub == "" {
		return "/usr/bin/chromium", nil
	}
	return f.pathStub, nil
}

type fakeStore struct {
	mu          sync.Mutex
	hasHits     map[string]bool
	records     []string
	extInserts  []store.OpenedUrlInsert
}

func newFakeStore() *fakeStore { return &fakeStore{hasHits: map[string]bool{}} }

func (s *fakeStore) HasOpenedUrl(_ context.Context, id int64, u string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasHits[u], nil
}

func (s *fakeStore) RecordOpenedUrl(_ context.Context, id int64, rule, u string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, u)
	return true, nil
}

func (s *fakeStore) RecordOpenedUrlExt(_ context.Context, in store.OpenedUrlInsert) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, in.Url)
	s.extInserts = append(s.extInserts, in)
	return true, nil
}

func mustTools(t *testing.T, b urlLauncher, s openedUrlRecorder) *Tools {
	t.Helper()
	r := NewTools(b, s, DefaultToolsConfig())
	if r.HasError() {
		t.Fatalf("NewTools: %v", r.Error())
	}
	return r.Value()
}

func TestOpenUrl_HappyPath(t *testing.T) {
	b := &fakeBrowser{}
	s := newFakeStore()
	tools := mustTools(t, b, s)
	r := tools.OpenUrl(context.Background(), OpenUrlSpec{Url: "https://example.com/page", Origin: OriginManual})
	if r.HasError() {
		t.Fatalf("OpenUrl: %v", r.Error())
	}
	if rep := r.Value(); rep.Deduped || !rep.IsIncognito || rep.BrowserBinary == "" {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if len(b.opened) != 1 {
		t.Fatalf("expected 1 launch, got %d", len(b.opened))
	}
}

func TestOpenUrl_ValidationFailures(t *testing.T) {
	b := &fakeBrowser{}
	s := newFakeStore()
	tools := mustTools(t, b, s)
	cases := []struct {
		name string
		url  string
		code errtrace.Code
	}{
		{"empty", "", errtrace.ErrToolsOpenUrlEmpty},
		{"too-long", "https://example.com/" + strings.Repeat("a", 9000), errtrace.ErrToolsOpenUrlTooLong},
		{"scheme-javascript", "javascript:alert(1)", errtrace.ErrToolsOpenUrlScheme},
		{"scheme-file", "file:///etc/passwd", errtrace.ErrToolsOpenUrlScheme},
		{"loopback", "http://127.0.0.1/admin", errtrace.ErrToolsOpenUrlLocalhost},
		{"localhost", "http://localhost:8080/", errtrace.ErrToolsOpenUrlLocalhost},
		{"private-ip", "http://10.0.0.5/", errtrace.ErrToolsOpenUrlPrivateIp},
		{"no-host", "https://", errtrace.ErrToolsOpenUrlMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tools.OpenUrl(context.Background(), OpenUrlSpec{Url: tc.url})
			if !r.HasError() {
				t.Fatalf("expected error for %q", tc.url)
			}
			var coded *errtrace.Coded
			if !errors.As(r.Error(), &coded) || coded.Code != tc.code {
				t.Fatalf("expected code %s, got %v", tc.code, r.Error())
			}
			if len(b.opened) != 0 {
				t.Fatalf("browser must not launch on validation failure (%q)", tc.url)
			}
		})
	}
}

func TestOpenUrl_DedupInMemory(t *testing.T) {
	b := &fakeBrowser{}
	s := newFakeStore()
	tools := mustTools(t, b, s)
	spec := OpenUrlSpec{Url: "https://x.test/p", Alias: "work", Origin: OriginManual}
	_ = tools.OpenUrl(context.Background(), spec)
	r2 := tools.OpenUrl(context.Background(), spec)
	if r2.HasError() || !r2.Value().Deduped {
		t.Fatalf("second call must be deduped, got %+v err=%v", r2.Value(), r2.Error())
	}
	if len(b.opened) != 1 {
		t.Fatalf("dedup must skip browser launch; got %d launches", len(b.opened))
	}
}

func TestOpenUrl_RedactsUserinfoAndSecrets(t *testing.T) {
	b := &fakeBrowser{}
	s := newFakeStore()
	tools := mustTools(t, b, s)
	r := tools.OpenUrl(context.Background(), OpenUrlSpec{Url: "https://user:pw@example.com/?token=abc&keep=1"})
	if r.HasError() {
		t.Fatalf("OpenUrl: %v", r.Error())
	}
	got := r.Value()
	if strings.Contains(got.Url, "user:pw") || strings.Contains(got.Url, "token=abc") {
		t.Fatalf("canonical url not redacted: %s", got.Url)
	}
	if !strings.Contains(got.OriginalUrl, "user:pw") {
		t.Fatalf("original must preserve userinfo: %s", got.OriginalUrl)
	}
	if !strings.Contains(b.opened[0], "token=%2A%2A%2A") && !strings.Contains(b.opened[0], "token=***") {
		t.Fatalf("launched url must use redacted token, got %s", b.opened[0])
	}
}

func TestNewTools_ConfigValidation(t *testing.T) {
	bad := DefaultToolsConfig()
	bad.OpenUrlAllowedSchemes = nil
	if r := NewTools(&fakeBrowser{}, newFakeStore(), bad); !r.HasError() {
		t.Fatal("expected error for empty schemes")
	}
	bad2 := DefaultToolsConfig()
	bad2.OpenUrlMaxLengthBytes = 10
	if r := NewTools(&fakeBrowser{}, newFakeStore(), bad2); !r.HasError() {
		t.Fatal("expected error for tiny max-length")
	}
}
