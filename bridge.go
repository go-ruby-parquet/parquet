// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	gruby "github.com/go-ruby-arrow/arrow"
)

// alloc is the shared, CGO-free Go allocator. Using memory.NewGoAllocator (not
// arrow-go's cgo mallocator) keeps the whole Parquet path pure-Go (CGO=0), the
// same choice go-ruby-arrow makes.
var alloc memory.Allocator = memory.NewGoAllocator()

// Seams over the arrow-go IPC bridge used to move a decoded arrow-go table into
// a go-ruby-arrow [gruby.Table]. Encoding a valid, already-materialized table to
// an in-memory buffer does not fail in practice; these indirections exist so the
// unreachable-in-normal-operation error branches can be fault-injected in tests
// and the package can hold 100% coverage.
var (
	ipcWriteRecord   = func(w *ipc.Writer, r xarrow.RecordBatch) error { return w.Write(r) }
	ipcCloseWriter   = func(w *ipc.Writer) error { return w.Close() }
	decodeGrubyTable = func(data []byte) (*gruby.Table, error) { return gruby.DecodeTable(data) }
)

// toGrubyTable converts an arrow-go table (as produced by pqarrow) into a
// go-ruby-arrow [gruby.Table] by streaming it through the Arrow IPC format that
// both libraries share. The result is a single-batch go-ruby-arrow Table, so the
// two libraries interoperate at the Ruby level. The caller retains ownership of
// xt and must release it.
func toGrubyTable(xt xarrow.Table) (*gruby.Table, error) {
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(xt.Schema()), ipc.WithAllocator(alloc))

	tr := array.NewTableReader(xt, 0)
	defer tr.Release()
	for tr.Next() {
		if err := ipcWriteRecord(w, tr.RecordBatch()); err != nil {
			_ = w.Close()
			return nil, wrapError(KindIO, err, "encode Arrow table")
		}
	}
	if err := ipcCloseWriter(w); err != nil {
		return nil, wrapError(KindIO, err, "finalize Arrow table")
	}
	return decodeGrubyTable(buf.Bytes())
}

// grubyRecord extracts the underlying arrow-go record batch backing a
// go-ruby-arrow [gruby.Table], via the library's public Unwrap accessors.
func grubyRecord(t *gruby.Table) xarrow.RecordBatch {
	return t.RecordBatch().Unwrap()
}

// schemaToGruby re-presents an arrow-go schema as a go-ruby-arrow [gruby.Schema]
// through go-ruby-arrow's public field constructors, preserving field names,
// types and nullability.
func schemaToGruby(s *xarrow.Schema) *gruby.Schema {
	xf := s.Fields()
	fields := make([]*gruby.Field, len(xf))
	for i := range xf {
		dt := gruby.FromArrowType(xf[i].Type)
		if xf[i].Nullable {
			fields[i] = gruby.NewField(xf[i].Name, dt)
		} else {
			fields[i] = gruby.NewFieldNonNull(xf[i].Name, dt)
		}
	}
	return gruby.NewSchema(fields...)
}
