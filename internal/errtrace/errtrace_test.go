package errtrace

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapCapturesFrameAndChain(t *testing.T) {
	cause := errors.New("boom")
	err := Wrap(cause, "outer")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is should walk to cause, got %v", err)
	}
	if got := err.Error(); got != "outer: boom" {
		t.Fatalf("error text mismatch: %q", got)
	}
	frames := Frames(err)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d (%+v)", len(frames), frames)
	}
	if !strings.HasSuffix(frames[0].File, "errtrace_test.go") {
		t.Fatalf("unexpected file: %q", frames[0].File)
	}
	if frames[0].Line == 0 {
		t.Fatalf("frame line should be non-zero")
	}
}

func TestWrapNilReturnsNil(t *testing.T) {
	if err := Wrap(nil, "noop"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNestedFramesPreserveOrder(t *testing.T) {
	inner := func() error { return New("inner") }
	middle := func() error { return Wrap(inner(), "middle") }
	outer := func() error { return Wrap(middle(), "outer") }

	err := outer()
	frames := Frames(err)
	if len(frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(frames))
	}
	if !strings.Contains(Format(err), "outer: middle: inner") {
		t.Fatalf("format missing chained message: %s", Format(err))
	}
}

func TestErrorfPreservesUnwrap(t *testing.T) {
	cause := errors.New("disk full")
	err := Errorf("write %s: %w", "log", cause)
	if !errors.Is(err, cause) {
		t.Fatalf("Errorf should preserve %%w cause for errors.Is")
	}
	if !strings.HasPrefix(err.Error(), "write log: disk full") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
	if len(Frames(err)) != 1 {
		t.Fatalf("Errorf should capture exactly one frame")
	}
}
