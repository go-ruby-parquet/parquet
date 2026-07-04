// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"testing"

	gruby "github.com/go-ruby-arrow/arrow"
)

// TestSmokeReadmeExample mirrors the README usage snippet end to end.
func TestSmokeReadmeExample(t *testing.T) {
	schema := gruby.NewSchema(
		gruby.NewField("id", gruby.Int64()),
		gruby.NewField("name", gruby.StringType()),
	)
	id, _ := gruby.NewArrayOf(gruby.Int64(), []any{int64(1), int64(2), nil})
	name, _ := gruby.NewArrayOf(gruby.StringType(), []any{"a", "b", "c"})
	table, err := gruby.NewTable(schema, []*gruby.Array{id, name})
	if err != nil {
		t.Fatalf("build table: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteTableTo(&buf, table,
		WithCompression(Zstd), WithRowGroupSize(1000)); err != nil {
		t.Fatalf("write: %v", err)
	}
	back, err := ReadTableBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if back.NumRows() != 3 || back.NumColumns() != 2 {
		t.Fatalf("shape = %dx%d, want 3x2", back.NumRows(), back.NumColumns())
	}
	col, _ := back.Column("name")
	v, _ := col.Get(2)
	if v != "c" {
		t.Errorf("name[2] = %v, want c", v)
	}
}
