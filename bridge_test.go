// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"bytes"
	"testing"

	xarrow "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	gruby "github.com/go-ruby-arrow/arrow"
)

// readViaBridge drives a ReadTable so the toGrubyTable bridge (and its injected
// seams) execute.
func readViaBridge(t *testing.T) (*gruby.Table, error) {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteTableTo(&buf, buildSampleTable(t)); err != nil {
		t.Fatalf("write: %v", err)
	}
	return ReadTableBytes(buf.Bytes())
}

func TestBridgeIPCWriteError(t *testing.T) {
	orig := ipcWriteRecord
	ipcWriteRecord = func(*ipc.Writer, xarrow.RecordBatch) error { return errFault }
	defer func() { ipcWriteRecord = orig }()
	if _, err := readViaBridge(t); !errKind(err, KindIO) {
		t.Errorf("ipc write error = %v, want KindIO", err)
	}
}

func TestBridgeIPCCloseError(t *testing.T) {
	orig := ipcCloseWriter
	ipcCloseWriter = func(*ipc.Writer) error { return errFault }
	defer func() { ipcCloseWriter = orig }()
	if _, err := readViaBridge(t); !errKind(err, KindIO) {
		t.Errorf("ipc close error = %v, want KindIO", err)
	}
}

func TestBridgeDecodeError(t *testing.T) {
	orig := decodeGrubyTable
	decodeGrubyTable = func([]byte) (*gruby.Table, error) { return nil, errFault }
	defer func() { decodeGrubyTable = orig }()
	if _, err := readViaBridge(t); err == nil {
		t.Error("expected decode error")
	}
}

func TestSchemaToGrubyNonNullable(t *testing.T) {
	// A schema with a non-nullable field exercises the NonNull branch of the
	// arrow-go -> go-ruby-arrow schema conversion.
	xs := xarrow.NewSchema([]xarrow.Field{
		{Name: "req", Type: xarrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "opt", Type: xarrow.BinaryTypes.String, Nullable: true},
	}, nil)
	gs := schemaToGruby(xs)
	if gs.NumFields() != 2 {
		t.Fatalf("fields = %d, want 2", gs.NumFields())
	}
	if gs.Field(0).NullableQ() {
		t.Error("field 0 should be non-nullable")
	}
	if !gs.Field(1).NullableQ() {
		t.Error("field 1 should be nullable")
	}
}
