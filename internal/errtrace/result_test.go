package errtrace

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapCodeAndContext(t *testing.T) {
	cause := errors.New("disk full")
	err := WrapCode(cause, ErrExportWriteRow, "export csv").
		WithContext("OutPath", "/tmp/x.csv").
		WithContext("RowsWritten", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is should find cause; got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "[ER-EXP-21602]") {
		t.Fatalf("missing code in %q", msg)
	}
	if !strings.Contains(msg, "OutPath=/tmp/x.csv") || !strings.Contains(msg, "RowsWritten=42") {
		t.Fatalf("missing context in %q", msg)
	}
}

func TestWrapCodeNilReturnsNil(t *testing.T) {
	if got := WrapCode(nil, ErrUnknown, "noop"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestResultOkAndValue(t *testing.T) {
	r := Ok[string]("hello")
	if r.HasError() {
		t.Fatal("Ok should not have error")
	}
	if r.Value() != "hello" {
		t.Fatalf("expected hello, got %q", r.Value())
	}
	if r.Error() != nil {
		t.Fatalf("expected nil error, got %v", r.Error())
	}
}

func TestResultErrAndPropagate(t *testing.T) {
	cause := WrapCode(errors.New("boom"), ErrDbOpen, "open db")
	r := Err[int](cause)
	if !r.HasError() {
		t.Fatal("Err should report HasError")
	}
	if r.Value() != 0 {
		t.Fatalf("zero value expected, got %d", r.Value())
	}
	prop := r.PropagateError()
	if prop == nil {
		t.Fatal("PropagateError on failed result should be non-nil")
	}
	if !errors.Is(prop, cause) {
		t.Fatal("propagated error should still unwrap to original cause")
	}
	// Frame chain should grow by 1 (the propagation frame).
	if len(Frames(prop)) < 2 {
		t.Fatalf("expected ≥2 frames after propagate, got %d", len(Frames(prop)))
	}
}

func TestResultErrNilDefensive(t *testing.T) {
	r := Err[string](nil)
	if !r.HasError() {
		t.Fatal("Err(nil) should still report HasError to avoid silent success")
	}
}
