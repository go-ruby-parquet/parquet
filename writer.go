// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"io"
	"os"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	gruby "github.com/go-ruby-arrow/arrow"
)

// newFileWriter is a seam over pqarrow.NewFileWriter so the (schema-dependent,
// not otherwise reachable) construction error can be fault-injected in tests.
var newFileWriter = pqarrow.NewFileWriter

// createFile is a seam over os.Create so the file-create and file-close error
// branches of [WriteTable] can be fault-injected in tests.
var createFile = func(path string) (io.WriteCloser, error) { return os.Create(path) }

// writeOnly hides the io.Closer of an underlying writer so the Parquet writer
// does not close the caller's file handle out from under it.
type writeOnly struct{ w io.Writer }

func (o writeOnly) Write(p []byte) (int, error) { return o.w.Write(p) }

// ArrowFileWriter is the pure-Go counterpart of Parquet::ArrowFileWriter — a
// streaming Parquet writer bound to an [io.Writer] and an Arrow schema. Write
// one or more go-ruby-arrow tables, then [ArrowFileWriter.Close] to flush the
// footer.
type ArrowFileWriter struct {
	fw     *pqarrow.FileWriter
	schema *xarrow.Schema
	chunk  int64
	closed bool
}

// NewArrowFileWriter opens a Parquet writer over w for tables matching schema
// (Parquet::ArrowFileWriter.new). The write options select compression,
// row-group size and dictionary encoding.
func NewArrowFileWriter(w io.Writer, schema *gruby.Schema, opts ...WriteOption) (*ArrowFileWriter, error) {
	if w == nil {
		return nil, newError(KindArgument, "nil writer")
	}
	if schema == nil {
		return nil, newError(KindArgument, "nil schema")
	}
	cfg := buildWriteConfig(opts)
	props, err := cfg.resolve()
	if err != nil {
		return nil, err
	}
	xschema := schema.Unwrap()
	fw, err := newFileWriter(xschema, w, props, pqarrow.DefaultWriterProps())
	if err != nil {
		return nil, wrapError(KindIO, err, "open Parquet writer")
	}
	return &ArrowFileWriter{fw: fw, schema: xschema, chunk: cfg.rowGroupSize}, nil
}

// Write appends a go-ruby-arrow table to the file, chunked into row groups of
// the configured size (Parquet::ArrowFileWriter#write_table). The table's schema
// must match the writer's schema.
func (w *ArrowFileWriter) Write(t *gruby.Table) error {
	if t == nil {
		return newError(KindArgument, "nil table")
	}
	if !t.Schema().Unwrap().Equal(w.schema) {
		return newError(KindType, "table schema does not match writer schema")
	}
	rec := grubyRecord(t)
	xt := array.NewTableFromRecords(w.schema, []xarrow.RecordBatch{rec})
	defer xt.Release()
	if err := w.fw.WriteTable(xt, w.chunk); err != nil {
		return wrapError(KindIO, err, "write Parquet table")
	}
	return nil
}

// Close flushes the Parquet footer and releases the writer
// (Parquet::ArrowFileWriter#close). It is idempotent.
func (w *ArrowFileWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if err := w.fw.Close(); err != nil {
		return wrapError(KindIO, err, "close Parquet writer")
	}
	return nil
}

// WriteTableTo writes a single go-ruby-arrow table to w as a complete Parquet
// file (open, write, close), mirroring Parquet::Writer.write to an IO.
func WriteTableTo(w io.Writer, t *gruby.Table, opts ...WriteOption) error {
	if t == nil {
		return newError(KindArgument, "nil table")
	}
	fw, err := NewArrowFileWriter(w, t.Schema(), opts...)
	if err != nil {
		return err
	}
	if err := fw.Write(t); err != nil {
		_ = fw.Close()
		return err
	}
	return fw.Close()
}

// WriteTable writes a go-ruby-arrow table to a Parquet file at path
// (Parquet::Writer.write(table, path)).
func WriteTable(t *gruby.Table, path string, opts ...WriteOption) error {
	f, err := createFile(path)
	if err != nil {
		return wrapError(KindIO, err, "create %s", path)
	}
	// The Parquet writer closes any io.Closer sink on its own Close, so hand it
	// a write-only view and keep ownership of the file handle here.
	if werr := WriteTableTo(writeOnly{f}, t, opts...); werr != nil {
		_ = f.Close()
		return werr
	}
	if err := f.Close(); err != nil {
		return wrapError(KindIO, err, "close %s", path)
	}
	return nil
}

// Save writes a go-ruby-arrow table to a Parquet file at path, mirroring
// red-arrow's Arrow::Table#save format dispatch for the ".parquet" extension.
func Save(t *gruby.Table, path string, opts ...WriteOption) error {
	return WriteTable(t, path, opts...)
}
