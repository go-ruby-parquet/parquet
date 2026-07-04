// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	gruby "github.com/go-ruby-arrow/arrow"
)

func TestNewArrowFileWriterNilArgs(t *testing.T) {
	sch := gruby.NewSchema(gruby.NewField("a", gruby.Int64()))
	if _, err := NewArrowFileWriter(nil, sch); !errKind(err, KindArgument) {
		t.Errorf("nil writer error = %v, want KindArgument", err)
	}
	if _, err := NewArrowFileWriter(&bytes.Buffer{}, nil); !errKind(err, KindArgument) {
		t.Errorf("nil schema error = %v, want KindArgument", err)
	}
}

func TestNewArrowFileWriterBadCompression(t *testing.T) {
	sch := gruby.NewSchema(gruby.NewField("a", gruby.Int64()))
	_, err := NewArrowFileWriter(&bytes.Buffer{}, sch, WithCompression(Compression(99)))
	if !errKind(err, KindArgument) {
		t.Errorf("bad compression error = %v, want KindArgument", err)
	}
}

func TestNewArrowFileWriterConstructError(t *testing.T) {
	orig := newFileWriter
	newFileWriter = func(*xarrow.Schema, io.Writer, *parquet.WriterProperties, pqarrow.ArrowWriterProperties) (*pqarrow.FileWriter, error) {
		return nil, errFault
	}
	defer func() { newFileWriter = orig }()

	sch := gruby.NewSchema(gruby.NewField("a", gruby.Int64()))
	if _, err := NewArrowFileWriter(&bytes.Buffer{}, sch); !errKind(err, KindIO) {
		t.Errorf("construct error = %v, want KindIO", err)
	}
}

func TestWriteNilTable(t *testing.T) {
	sch := gruby.NewSchema(gruby.NewField("a", gruby.Int64()))
	w, err := NewArrowFileWriter(&bytes.Buffer{}, sch)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Close()
	if err := w.Write(nil); !errKind(err, KindArgument) {
		t.Errorf("Write(nil) error = %v, want KindArgument", err)
	}
}

func TestWriteSchemaMismatch(t *testing.T) {
	w, err := NewArrowFileWriter(&bytes.Buffer{},
		gruby.NewSchema(gruby.NewField("a", gruby.Int64())))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Close()

	other, _ := gruby.NewTable(
		gruby.NewSchema(gruby.NewField("b", gruby.StringType())),
		[]*gruby.Array{mustArray(t, gruby.StringType(), []any{"x"})})
	if err := w.Write(other); !errKind(err, KindType) {
		t.Errorf("mismatch error = %v, want KindType", err)
	}
}

func TestWriteSinkError(t *testing.T) {
	tbl := buildSampleTable(t)
	w, err := NewArrowFileWriter(&budgetWriter{budget: 8}, tbl.Schema())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := w.Write(tbl); !errKind(err, KindIO) {
		t.Errorf("write sink error = %v, want KindIO", err)
	}
	_ = w.Close()
}

func TestCloseError(t *testing.T) {
	tbl := buildSampleTable(t)
	// Budget exactly the opening "PAR1" magic (4 bytes) written at construction,
	// so the footer flush inside Close is the first write to fail. No data is
	// written, isolating the Close error branch.
	w, err := NewArrowFileWriter(&budgetWriter{budget: 4}, tbl.Schema())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := w.Close(); !errKind(err, KindIO) {
		t.Errorf("Close error = %v, want KindIO", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	w, err := NewArrowFileWriter(&bytes.Buffer{},
		gruby.NewSchema(gruby.NewField("a", gruby.Int64())))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second close should be nil, got %v", err)
	}
}

func TestWriteTableToNilTable(t *testing.T) {
	if err := WriteTableTo(&bytes.Buffer{}, nil); !errKind(err, KindArgument) {
		t.Errorf("nil table error = %v, want KindArgument", err)
	}
}

func TestWriteTableToConstructError(t *testing.T) {
	// A nil writer makes NewArrowFileWriter (inside WriteTableTo) fail.
	if err := WriteTableTo(nil, buildSampleTable(t)); !errKind(err, KindArgument) {
		t.Errorf("construct error = %v, want KindArgument", err)
	}
}

func TestWriteTableToWriteError(t *testing.T) {
	if err := WriteTableTo(&budgetWriter{budget: 8}, buildSampleTable(t)); !errKind(err, KindIO) {
		t.Errorf("write error = %v, want KindIO", err)
	}
}

func TestWriteTableCreateError(t *testing.T) {
	orig := createFile
	createFile = func(string) (io.WriteCloser, error) { return nil, errFault }
	defer func() { createFile = orig }()
	if err := WriteTable(buildSampleTable(t), "x.parquet"); !errKind(err, KindIO) {
		t.Errorf("create error = %v, want KindIO", err)
	}
}

func TestWriteTableWriteError(t *testing.T) {
	orig := createFile
	createFile = func(string) (io.WriteCloser, error) {
		return failWriteCloser{w: &budgetWriter{budget: 8}}, nil
	}
	defer func() { createFile = orig }()
	if err := WriteTable(buildSampleTable(t), "x.parquet"); !errKind(err, KindIO) {
		t.Errorf("write error = %v, want KindIO", err)
	}
}

func TestWriteTableCloseError(t *testing.T) {
	orig := createFile
	createFile = func(string) (io.WriteCloser, error) {
		return failWriteCloser{w: &bytes.Buffer{}}, nil
	}
	defer func() { createFile = orig }()
	if err := WriteTable(buildSampleTable(t), "x.parquet"); !errKind(err, KindIO) {
		t.Errorf("close error = %v, want KindIO", err)
	}
}

func TestWriteTableRealFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.parquet")
	if err := WriteTable(buildSampleTable(t), path); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// mustArray is a small builder helper for the writer tests.
func mustArray(t *testing.T, dt *gruby.DataType, values []any) *gruby.Array {
	t.Helper()
	a, err := gruby.NewArrayOf(dt, values)
	if err != nil {
		t.Fatalf("build array: %v", err)
	}
	return a
}
