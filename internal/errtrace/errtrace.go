// Package errtrace adds lightweight, opt-in stack-trace capture to the error
// chain so logs reveal exactly which file:line every error passed through.
//
// Usage:
//
//	if err := doThing(); err != nil {
//	    return errtrace.Wrap(err, "do thing")          // captures one frame
//	}
//	return errtrace.New("no accounts configured")      // captures one frame
//
// At the top level (main / log.Printf), render the full chain:
//
//	fmt.Fprintln(os.Stderr, errtrace.Format(err))
//
// The output looks like:
//
//	error: imap login user@host: EOF
//	  at internal/mailclient/mailclient.go:70 (Dial)
//	  at internal/cli/cli.go:218 (runDiagnose)
//	  at internal/cli/cli.go:43 (NewRoot.func1)
//
// Errors wrapped with errors.Is / errors.As semantics still work — Wrap uses
// %w under the hood.
package errtrace

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Frame describes one captured stack location.
type Frame struct {
	File string
	Line int
	Func string
}

// Traced is an error that carries a single captured frame plus the wrapped
// cause. Callers normally never need to construct one directly — use Wrap /
// Wrapf / New.
type Traced struct {
	Msg   string
	Frame Frame
	Cause error
}

// Error implements the error interface and matches the familiar
// "msg: cause" layout used by fmt.Errorf("...: %w", ...).
func (t *Traced) Error() string {
	if t == nil {
		return ""
	}
	if t.Cause == nil {
		return t.Msg
	}
	if t.Msg == "" {
		return t.Cause.Error()
	}
	return t.Msg + ": " + t.Cause.Error()
}

// Unwrap exposes the cause for errors.Is / errors.As.
func (t *Traced) Unwrap() error { return t.Cause }

// caller captures the file:line:func of the caller `skip` frames above this
// function. skip=0 returns the immediate caller of caller().
func caller(skip int) Frame {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return Frame{File: "?", Line: 0, Func: "?"}
	}
	fn := runtime.FuncForPC(pc)
	name := "?"
	if fn != nil {
		name = trimFunc(fn.Name())
	}
	return Frame{File: trimFile(file), Line: line, Func: name}
}

// trimFile shortens "/long/path/to/internal/foo/bar.go" to
// "internal/foo/bar.go" for readable logs.
func trimFile(p string) string {
	// Heuristic: keep everything from the last "/internal/" or "/cmd/" onward,
	// otherwise keep the last two path components.
	for _, marker := range []string{"/internal/", "/cmd/"} {
		if i := strings.LastIndex(p, marker); i >= 0 {
			return p[i+1:]
		}
	}
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return p
}

// trimFunc drops the package import path prefix from a function name:
// "github.com/lovable/email-read/internal/cli.runDiagnose" -> "cli.runDiagnose".
func trimFunc(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// New returns a new traced error with msg and the current frame.
func New(msg string) error {
	return &Traced{Msg: msg, Frame: caller(1)}
}

// Wrap returns nil if err is nil. Otherwise it returns a *Traced that wraps
// err with msg and a frame for the calling site.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &Traced{Msg: msg, Frame: caller(1), Cause: err}
}

// Wrapf is Wrap with fmt.Sprintf-style formatting.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &Traced{Msg: fmt.Sprintf(format, args...), Frame: caller(1), Cause: err}
}

// Errorf is like fmt.Errorf but also captures a frame at the call site.
// If one of the args is an error wrapped via %w, it is preserved as the cause.
func Errorf(format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	cause := errors.Unwrap(wrapped)
	return &Traced{Msg: wrapped.Error(), Frame: caller(1), Cause: cause}
}

// Format renders err and every traced frame in its chain. Plain (non-traced)
// errors render as just their Error() text. Use this in main / loggers.
func Format(err error) string {
	if err == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("error: ")
	b.WriteString(err.Error())

	frames := Frames(err)
	if len(frames) == 0 {
		return b.String()
	}
	for _, f := range frames {
		b.WriteString("\n  at ")
		b.WriteString(f.File)
		b.WriteString(":")
		fmt.Fprintf(&b, "%d", f.Line)
		if f.Func != "" {
			b.WriteString(" (")
			b.WriteString(f.Func)
			b.WriteString(")")
		}
	}
	return b.String()
}

// Frames extracts every captured frame from the error chain in
// outermost-first order (closest to the report site first). Recognises both
// *Traced (legacy Wrap/New) and *Coded (WrapCode/NewCoded) carriers.
func Frames(err error) []Frame {
	var out []Frame
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		switch v := cur.(type) {
		case *Traced:
			out = append(out, v.Frame)
		case *Coded:
			out = append(out, v.Frame)
		}
	}
	return out
}

