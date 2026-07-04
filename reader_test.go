// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// sampleBytes returns a valid Parquet file for the kitchen-sink table.
func sampleBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteTableTo(&buf, buildSampleTable(t), WithRowGroupSize(2)); err != nil {
		t.Fatalf("write: %v", err)
	}
	return buf.Bytes()
}

func TestNewArrowFileReaderNil(t *testing.T) {
	if _, err := NewArrowFileReader(nil); !errKind(err, KindArgument) {
		t.Errorf("nil reader error = %v, want KindArgument", err)
	}
}

func TestReadTableBytesGarbage(t *testing.T) {
	if _, err := ReadTableBytes([]byte("not a parquet file at all")); !errKind(err, KindIO) {
		t.Errorf("garbage error = %v, want KindIO", err)
	}
}

func TestOpenArrowFileReaderMissing(t *testing.T) {
	if _, err := OpenArrowFileReader(filepath.Join(t.TempDir(), "nope.parquet")); !errKind(err, KindIO) {
		t.Errorf("missing path error = %v, want KindIO", err)
	}
}

func TestOpenArrowFileReaderNotParquet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(path, []byte("hello, not parquet"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenArrowFileReader(path); !errKind(err, KindIO) {
		t.Errorf("non-parquet error = %v, want KindIO", err)
	}
}

func TestNewParquetFileReaderSeamError(t *testing.T) {
	orig := newParquetFileReader
	newParquetFileReader = func(*file.Reader, pqarrow.ArrowReadProperties, memory.Allocator) (*pqarrow.FileReader, error) {
		return nil, errFault
	}
	defer func() { newParquetFileReader = orig }()
	if _, err := NewArrowFileReader(bytes.NewReader(sampleBytes(t))); !errKind(err, KindIO) {
		t.Errorf("seam error = %v, want KindIO", err)
	}
}

func TestReadTableSeamError(t *testing.T) {
	rd, err := NewArrowFileReader(bytes.NewReader(sampleBytes(t)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()
	orig := readArrowTable
	readArrowTable = func(*pqarrow.FileReader, context.Context) (xarrow.Table, error) {
		return nil, errFault
	}
	defer func() { readArrowTable = orig }()
	if _, err := rd.ReadTable(); !errKind(err, KindIO) {
		t.Errorf("read table seam error = %v, want KindIO", err)
	}
}

func TestReadRowGroupOutOfRange(t *testing.T) {
	rd, err := NewArrowFileReader(bytes.NewReader(sampleBytes(t)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()
	if _, err := rd.ReadRowGroup(-1); !errKind(err, KindIndex) {
		t.Errorf("ReadRowGroup(-1) error = %v, want KindIndex", err)
	}
	if _, err := rd.ReadRowGroup(999); !errKind(err, KindIndex) {
		t.Errorf("ReadRowGroup(999) error = %v, want KindIndex", err)
	}
}

func TestReadRowGroupSeamError(t *testing.T) {
	rd, err := NewArrowFileReader(bytes.NewReader(sampleBytes(t)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()
	orig := readArrowRowGroup
	readArrowRowGroup = func(*pqarrow.FileReader, context.Context, int) (xarrow.Table, error) {
		return nil, errFault
	}
	defer func() { readArrowRowGroup = orig }()
	if _, err := rd.ReadRowGroup(0); !errKind(err, KindIO) {
		t.Errorf("row group seam error = %v, want KindIO", err)
	}
}

func TestSchemaSeamError(t *testing.T) {
	rd, err := NewArrowFileReader(bytes.NewReader(sampleBytes(t)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rd.Close()
	orig := readerSchema
	readerSchema = func(*pqarrow.FileReader) (*xarrow.Schema, error) { return nil, errFault }
	defer func() { readerSchema = orig }()
	if _, err := rd.Schema(); !errKind(err, KindIO) {
		t.Errorf("schema seam error = %v, want KindIO", err)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.parquet")); !errKind(err, KindIO) {
		t.Errorf("Load(missing) error = %v, want KindIO", err)
	}
}

func TestReaderClosePfError(t *testing.T) {
	rd, err := NewArrowFileReader(newFailReadSeekCloser(sampleBytes(t)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := rd.Close(); !errKind(err, KindIO) {
		t.Errorf("pf close error = %v, want KindIO", err)
	}
	// Idempotent: a second close is a no-op.
	if err := rd.Close(); err != nil {
		t.Errorf("second close = %v, want nil", err)
	}
}
