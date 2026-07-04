// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorString(t *testing.T) {
	base := errors.New("boom")
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{"msg only", newError(KindIO, "nope"), "nope"},
		{"msg + cause", wrapError(KindIO, base, "context"), "context: boom"},
		{"cause only", &Error{Kind: KindIO, Err: base}, "boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestErrorUnwrapAndIs(t *testing.T) {
	base := errors.New("root")
	err := wrapError(KindIO, base, "io failed")
	if !errors.Is(err, base) {
		t.Error("Unwrap should expose the cause")
	}
	if !errors.Is(err, ErrIO) {
		t.Error("Is should match by kind")
	}
	if errors.Is(err, ErrType) {
		t.Error("Is should not match a different kind")
	}
	// Is against a non-*Error target returns false.
	if err.Is(errors.New("plain")) {
		t.Error("Is should be false for a non-*Error target")
	}
}

func TestErrorRubyClass(t *testing.T) {
	cases := map[ErrorKind]string{
		KindError:          "Parquet::Error",
		KindType:           "TypeError",
		KindIndex:          "IndexError",
		KindArgument:       "ArgumentError",
		KindIO:             "Parquet::Error::Io",
		KindNotImplemented: "NotImplementedError",
	}
	for kind, want := range cases {
		if got := (&Error{Kind: kind}).RubyClass(); got != want {
			t.Errorf("kind %d RubyClass = %q, want %q", kind, got, want)
		}
	}
}

func TestErrorSentinels(t *testing.T) {
	for _, s := range []*Error{ErrType, ErrIndex, ErrArgument, ErrIO, ErrNotImplemented} {
		if s.Error() == "" && s.Kind == KindError {
			t.Errorf("unexpected sentinel %v", s)
		}
	}
	// wrapError formats its message.
	e := wrapError(KindArgument, fmt.Errorf("x"), "bad %d", 3)
	if e.Msg != "bad 3" {
		t.Errorf("Msg = %q", e.Msg)
	}
}
