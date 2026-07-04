// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"errors"
	"io"
)

// errFault is the canonical injected failure used across the fault tests.
var errFault = errors.New("injected fault")

// budgetWriter lets the first budget bytes through, then fails every Write. A
// budget of zero fails on the first non-empty write.
type budgetWriter struct {
	n      int
	budget int
}

func (b *budgetWriter) Write(p []byte) (int, error) {
	if b.n+len(p) > b.budget {
		b.n = b.budget
		return 0, errFault
	}
	b.n += len(p)
	return len(p), nil
}

// failWriteCloser is a WriteCloser that writes fine but fails on Close.
type failWriteCloser struct{ w io.Writer }

func (f failWriteCloser) Write(p []byte) (int, error) { return f.w.Write(p) }
func (f failWriteCloser) Close() error                { return errFault }

// failReadSeekCloser is a parquet.ReaderAtSeeker (backed by a bytes.Reader) whose
// Close fails, so the Parquet reader's Close error branch is reachable.
type failReadSeekCloser struct{ r *bytes.Reader }

func newFailReadSeekCloser(data []byte) *failReadSeekCloser {
	return &failReadSeekCloser{r: bytes.NewReader(data)}
}
func (f *failReadSeekCloser) Read(p []byte) (int, error)            { return f.r.Read(p) }
func (f *failReadSeekCloser) ReadAt(p []byte, o int64) (int, error) { return f.r.ReadAt(p, o) }
func (f *failReadSeekCloser) Seek(o int64, w int) (int64, error)    { return f.r.Seek(o, w) }
func (f *failReadSeekCloser) Close() error                          { return errFault }
