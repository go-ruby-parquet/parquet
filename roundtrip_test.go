// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	gruby "github.com/go-ruby-arrow/arrow"
)

// TestRoundTripAllTypesAllCodecs writes the kitchen-sink table with every
// compression codec, reads it back and checks values + schema are preserved.
func TestRoundTripAllTypesAllCodecs(t *testing.T) {
	codecs := []Compression{Uncompressed, Snappy, Gzip, Zstd}
	for _, codec := range codecs {
		t.Run(codec.String(), func(t *testing.T) {
			tbl := buildSampleTable(t)
			var buf bytes.Buffer
			if err := WriteTableTo(&buf, tbl,
				WithCompression(codec),
				WithRowGroupSize(2),
				WithDictionary(true)); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, err := ReadTableBytes(buf.Bytes())
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			assertSameTable(t, got)
		})
	}
}

// TestReadRowGroups checks per-row-group reads reassemble the full table and
// that NumRows/NumRowGroups report the chunking.
func TestReadRowGroups(t *testing.T) {
	tbl := buildSampleTable(t)
	var buf bytes.Buffer
	if err := WriteTableTo(&buf, tbl, WithRowGroupSize(2)); err != nil {
		t.Fatalf("write: %v", err)
	}
	rd, err := NewArrowFileReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()

	if rd.NumRows() != 3 {
		t.Errorf("NumRows = %d, want 3", rd.NumRows())
	}
	if rd.NumRowGroups() != 2 { // 3 rows / 2 per group => 2 groups
		t.Fatalf("NumRowGroups = %d, want 2", rd.NumRowGroups())
	}

	var total int64
	for i := 0; i < rd.NumRowGroups(); i++ {
		rg, err := rd.ReadRowGroup(i)
		if err != nil {
			t.Fatalf("row group %d: %v", i, err)
		}
		total += rg.NumRows()
	}
	if total != 3 {
		t.Errorf("row-group rows sum = %d, want 3", total)
	}

	// The reader schema mirrors the table schema (names + types).
	sc, err := rd.Schema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if sc.NumFields() != len(sampleColumns) {
		t.Errorf("schema fields = %d, want %d", sc.NumFields(), len(sampleColumns))
	}
}

// TestSaveLoadFile exercises the Table.save / Table.load convenience over a
// temp file (no network).
func TestSaveLoadFile(t *testing.T) {
	tbl := buildSampleTable(t)
	path := filepath.Join(t.TempDir(), "kitchen.parquet")
	if err := Save(tbl, path, WithCompression(Zstd)); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	assertSameTable(t, got)
}

// TestOpenArrowFileReaderPath reads through the path-based constructor.
func TestOpenArrowFileReaderPath(t *testing.T) {
	tbl := buildSampleTable(t)
	path := filepath.Join(t.TempDir(), "p.parquet")
	if err := WriteTable(tbl, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	rd, err := OpenArrowFileReader(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()
	got, err := rd.ReadTable()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	assertSameTable(t, got)
}

// TestWireCompatWeToCanonical writes here, reads with arrow-go's canonical
// pqarrow.ReadTable, and checks the row/column counts survive.
func TestWireCompatWeToCanonical(t *testing.T) {
	tbl := buildSampleTable(t)
	var buf bytes.Buffer
	if err := WriteTableTo(&buf, tbl, WithCompression(Snappy)); err != nil {
		t.Fatalf("write: %v", err)
	}
	xt, err := pqarrow.ReadTable(context.Background(), bytes.NewReader(buf.Bytes()),
		nil, pqarrow.ArrowReadProperties{}, alloc)
	if err != nil {
		t.Fatalf("canonical read: %v", err)
	}
	defer xt.Release()
	if xt.NumRows() != 3 {
		t.Errorf("canonical NumRows = %d, want 3", xt.NumRows())
	}
	if int(xt.NumCols()) != len(sampleColumns) {
		t.Errorf("canonical NumCols = %d, want %d", xt.NumCols(), len(sampleColumns))
	}
}

// TestWireCompatCanonicalToUs writes with arrow-go's canonical writer and reads
// the file back through this package, checking values + schema.
func TestWireCompatCanonicalToUs(t *testing.T) {
	tbl := buildSampleTable(t)
	xt := grubyToArrowTable(t, tbl)
	defer xt.Release()

	var buf bytes.Buffer
	if err := pqarrow.WriteTable(xt, &buf, 2, nil, pqarrow.DefaultWriterProps()); err != nil {
		t.Fatalf("canonical write: %v", err)
	}
	got, err := ReadTableBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	assertSameTable(t, got)
}

// grubyToArrowTable lifts a go-ruby-arrow table into an arrow-go table for the
// canonical-writer direction of the wire-compat test.
func grubyToArrowTable(t *testing.T, tbl *gruby.Table) xarrow.Table {
	t.Helper()
	rec := grubyRecord(tbl)
	return array.NewTableFromRecords(rec.Schema(), []xarrow.RecordBatch{rec})
}
