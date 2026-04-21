package browser

import (
	"path/filepath"
	"testing"
)

func TestPickIncognitoArg(t *testing.T) {
	cases := map[string]string{
		filepath.FromSlash("/usr/bin/google-chrome"):  "--incognito",
		filepath.FromSlash("/usr/bin/chromium"):       "--incognito",
		filepath.FromSlash("/Applications/Brave.app"): "--incognito",
		filepath.FromSlash("/usr/bin/firefox"):        "-private-window",
	}
	for path, want := range cases {
		if got := pickIncognitoArg(path, ""); got != want {
			t.Errorf("path=%s got=%q want=%q", path, got, want)
		}
	}
	if got := pickIncognitoArg("/usr/bin/chrome", "--inprivate"); got != "--inprivate" {
		t.Errorf("override ignored: %q", got)
	}
}

func TestFileExists(t *testing.T) {
	if fileExists("") {
		t.Error("empty path should be false")
	}
	if fileExists("/this/path/does/not/exist/zzzzzzz") {
		t.Error("nonexistent path should be false")
	}
}
