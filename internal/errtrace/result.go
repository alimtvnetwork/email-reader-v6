// Package errtrace — result.go adds the Result[T] envelope and structured
// wrapping primitives (Code + WithContext) required by spec/21-app/04-coding-
// standards.md §4.2 and spec/12-consolidated-guidelines/03-error-management.md.
//
// The legacy Wrap(err, msg) / New(msg) helpers in errtrace.go remain
// supported for incremental adoption. New code in internal/core MUST use:
//
//	func DoThing(...) errtrace.Result[T] {
//	    if err := lower(); err != nil {
//	        return errtrace.Err[T](errtrace.WrapCode(err, ErrXxx, "do thing").
//	            WithContext("Alias", alias))
//	    }
//	    return errtrace.Ok(value)
//	}
//
// At call sites:
//
//	r := DoThing(...)
//	if r.HasError() {
//	    return r.PropagateError() // or convert to error via r.Error()
//	}
//	v := r.Value()
package errtrace

import (
	"fmt"
	"strings"
)

// Code is a stable, registry-backed error identifier (e.g. "ER-EXP-21601").
// All codes are declared in spec/21-app/06-error-registry.md and surfaced as
// constants in this package's codes.go.
type Code string

// String returns the raw code, e.g. "ER-EXP-21601".
func (c Code) String() string { return string(c) }

// Coded is an error annotated with a stable Code, an operation phrase, a
// captured frame, contextual key/value pairs, and (optionally) a wrapped
// cause. Produced by WrapCode / NewCoded.
type Coded struct {
	Code    Code
	Op      string
	Frame   Frame
	Context []ContextField
	Cause   error
}

// ContextField is a single key/value pair attached via WithContext. Order is
// preserved so logs and Format output read in insertion order.
type ContextField struct {
	Key   string
	Value any
}

// Error implements the error interface using the familiar
// "[CODE] op: cause (k=v ...)" layout.
func (c *Coded) Error() string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	if c.Code != "" {
		b.WriteString("[")
		b.WriteString(string(c.Code))
		b.WriteString("] ")
	}
	if c.Op != "" {
		b.WriteString(c.Op)
	}
	if c.Cause != nil {
		if c.Op != "" {
			b.WriteString(": ")
		}
		b.WriteString(c.Cause.Error())
	}
	if len(c.Context) > 0 {
		b.WriteString(" (")
		for i, f := range c.Context {
			if i > 0 {
				b.WriteString(" ")
			}
			fmt.Fprintf(&b, "%s=%v", f.Key, f.Value)
		}
		b.WriteString(")")
	}
	return b.String()
}

// Unwrap exposes the cause for errors.Is / errors.As.
func (c *Coded) Unwrap() error { return c.Cause }

// WithContext appends a key/value pair and returns the same *Coded for
// chaining. Safe on nil receiver — returns nil.
func (c *Coded) WithContext(key string, value any) *Coded {
	if c == nil {
		return nil
	}
	c.Context = append(c.Context, ContextField{Key: key, Value: value})
	return c
}

// WrapCode wraps cause with a Code, an operation phrase, and a captured
// frame. Returns nil if cause is nil.
func WrapCode(cause error, code Code, op string) *Coded {
	if cause == nil {
		return nil
	}
	return &Coded{Code: code, Op: op, Frame: caller(1), Cause: cause}
}

// NewCoded returns a new *Coded with no underlying cause. Use when the error
// originates here (e.g. validation failure).
func NewCoded(code Code, op string) *Coded {
	return &Coded{Code: code, Op: op, Frame: caller(1)}
}

// ---------------------------------------------------------------------------
// Result[T]
// ---------------------------------------------------------------------------

// Result[T] is the single-return envelope for core APIs. Either Val or Err is
// meaningful but never both: HasError reports which.
type Result[T any] struct {
	val T
	err error
}

// Ok wraps a successful value.
func Ok[T any](v T) Result[T] { return Result[T]{val: v} }

// Err wraps a failure. Accepts any error so callers can pass *Coded directly
// or pre-existing errors.
func Err[T any](err error) Result[T] {
	if err == nil {
		// Degenerate but defensive — never let an Err with nil sneak through.
		err = NewCoded("ER-UNKNOWN-21999", "nil error passed to errtrace.Err")
	}
	return Result[T]{err: err}
}

// HasError reports whether the result carries a failure.
func (r Result[T]) HasError() bool { return r.err != nil }

// Value returns the success value. Caller MUST check HasError first; the
// zero value of T is returned otherwise.
func (r Result[T]) Value() T { return r.val }

// Error returns the underlying error or nil.
func (r Result[T]) Error() error { return r.err }

// PropagateError re-wraps the carried error with a fresh frame at the call
// site so log output points at the propagation point, not the origin. Returns
// nil if there is no error. Use as:
//
//	if r.HasError() { return errtrace.Err[U](r.PropagateError()) }
func (r Result[T]) PropagateError() error {
	if r.err == nil {
		return nil
	}
	return &Traced{Msg: "propagate", Frame: caller(1), Cause: r.err}
}
