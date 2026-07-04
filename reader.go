// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"context"
	"os"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	gruby "github.com/go-ruby-arrow/arrow"
)

// Seams over the arrow-go Parquet reader so the not-otherwise-reachable error
// branches (a corrupt Arrow schema in a structurally valid footer, a mid-stream
// column decode failure) can be fault-injected in tests, keeping coverage at
// 100%.
var (
	newParquetFileReader = pqarrow.NewFileReader
	readArrowTable       = func(fr *pqarrow.FileReader, ctx context.Context) (xarrow.Table, error) {
		return fr.ReadTable(ctx)
	}
	readArrowRowGroup = func(fr *pqarrow.FileReader, ctx context.Context, idx int) (xarrow.Table, error) {
		// A nil column selection reads *no* columns; pass the full leaf-column
		// index list so the whole row group is materialized, matching the
		// canonical ReadTable path.
		n := fr.ParquetReader().MetaData().Schema.NumColumns()
		cols := make([]int, n)
		for i := range cols {
			cols[i] = i
		}
		return fr.RowGroup(idx).ReadTable(ctx, cols)
	}
	readerSchema = func(fr *pqarrow.FileReader) (*xarrow.Schema, error) { return fr.Schema() }
)

// ArrowFileReader is the pure-Go counterpart of Parquet::ArrowFileReader — a
// random-access Parquet reader that yields go-ruby-arrow tables. Construct one
// from an in-memory reader with [NewArrowFileReader] or from a path with
// [OpenArrowFileReader], read with [ArrowFileReader.ReadTable] /
// [ArrowFileReader.ReadRowGroup], and [ArrowFileReader.Close] it when done.
type ArrowFileReader struct {
	pf *file.Reader
	fr *pqarrow.FileReader
}

// NewArrowFileReader opens a Parquet reader over an in-memory random-access
// source (Parquet::ArrowFileReader.new with an IO). A *bytes.Reader or *os.File
// satisfies [parquet.ReaderAtSeeker].
func NewArrowFileReader(r parquet.ReaderAtSeeker) (*ArrowFileReader, error) {
	if r == nil {
		return nil, newError(KindArgument, "nil reader")
	}
	return newArrowFileReader(r)
}

// ReadTableBytes reads a whole Parquet file held in memory into a go-ruby-arrow
// table, a convenience over [NewArrowFileReader] + [ArrowFileReader.ReadTable].
func ReadTableBytes(data []byte) (*gruby.Table, error) {
	rd, err := NewArrowFileReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	return rd.ReadTable()
}

// OpenArrowFileReader opens the Parquet file at path
// (Parquet::ArrowFileReader.new with a path). The whole file is buffered into
// memory and the OS file handle is released before returning, so no handle
// outlives the call — on Windows the file can then always be deleted, even
// while the reader is still in use.
func OpenArrowFileReader(path string) (*ArrowFileReader, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, wrapError(KindIO, err, "open %s", path)
	}
	return newArrowFileReader(bytes.NewReader(b))
}

// newArrowFileReader builds the reader over the in-memory random-access source
// r. The source carries no OS file handle, so [ArrowFileReader.Close] has none
// to release.
func newArrowFileReader(r parquet.ReaderAtSeeker) (*ArrowFileReader, error) {
	pf, err := file.NewParquetReader(r)
	if err != nil {
		return nil, wrapError(KindIO, err, "open Parquet file")
	}
	fr, err := newParquetFileReader(pf, pqarrow.ArrowReadProperties{}, alloc)
	if err != nil {
		_ = pf.Close()
		return nil, wrapError(KindIO, err, "open Parquet Arrow reader")
	}
	return &ArrowFileReader{pf: pf, fr: fr}, nil
}

// ReadTable reads every row group into one go-ruby-arrow table
// (Parquet::ArrowFileReader#read_table).
func (r *ArrowFileReader) ReadTable() (*gruby.Table, error) {
	xt, err := readArrowTable(r.fr, context.Background())
	if err != nil {
		return nil, wrapError(KindIO, err, "read Parquet table")
	}
	defer xt.Release()
	return toGrubyTable(xt)
}

// ReadRowGroup reads a single row group by index into a go-ruby-arrow table
// (Parquet::ArrowFileReader#read_row_group). An out-of-range index yields an
// [*Error] of [KindIndex].
func (r *ArrowFileReader) ReadRowGroup(idx int) (*gruby.Table, error) {
	n := r.NumRowGroups()
	if idx < 0 || idx >= n {
		return nil, newError(KindIndex, "row group %d out of range (%d row groups)", idx, n)
	}
	xt, err := readArrowRowGroup(r.fr, context.Background(), idx)
	if err != nil {
		return nil, wrapError(KindIO, err, "read Parquet row group %d", idx)
	}
	defer xt.Release()
	return toGrubyTable(xt)
}

// NumRows returns the total number of rows across all row groups
// (Parquet::ArrowFileReader#n_rows).
func (r *ArrowFileReader) NumRows() int64 { return r.pf.NumRows() }

// NumRowGroups returns the number of row groups
// (Parquet::ArrowFileReader#n_row_groups).
func (r *ArrowFileReader) NumRowGroups() int { return r.pf.NumRowGroups() }

// Schema returns the file's schema as a go-ruby-arrow schema
// (Parquet::ArrowFileReader#schema).
func (r *ArrowFileReader) Schema() (*gruby.Schema, error) {
	xs, err := readerSchema(r.fr)
	if err != nil {
		return nil, wrapError(KindIO, err, "read Parquet schema")
	}
	return schemaToGruby(xs), nil
}

// Close releases the reader (Parquet::ArrowFileReader#close). The source is
// always in memory, so there is no OS file handle to release. It is idempotent.
func (r *ArrowFileReader) Close() error {
	if r.pf == nil {
		return nil
	}
	err := r.pf.Close()
	r.pf = nil
	if err != nil {
		return wrapError(KindIO, err, "close Parquet reader")
	}
	return nil
}

// Load reads a Parquet file at path into a go-ruby-arrow table, mirroring
// red-arrow's Arrow::Table.load format dispatch for the ".parquet" extension.
func Load(path string) (*gruby.Table, error) {
	rd, err := OpenArrowFileReader(path)
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	return rd.ReadTable()
}
