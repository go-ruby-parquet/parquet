// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"errors"
	"reflect"
	"testing"
	"time"

	gruby "github.com/go-ruby-arrow/arrow"
)

// ts builds a UTC time truncated to microseconds (Arrow's timestamp unit here),
// so it survives a Parquet round-trip byte-for-byte.
func ts(y int, mo time.Month, d, h, mi, s, us int) time.Time {
	return time.Date(y, mo, d, h, mi, s, us*1000, time.UTC)
}

// day builds a UTC midnight time, the granularity a Date32 column preserves.
func day(y int, mo time.Month, d int) time.Time {
	return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
}

// sampleColumns lists every column type the round-trip exercises, with the
// exact go-ruby-arrow value literals to append (including nulls).
var sampleColumns = []struct {
	name   string
	dt     *gruby.DataType
	values []any
}{
	{"i8", gruby.Int8(), []any{int8(-1), int8(2), nil}},
	{"i16", gruby.Int16(), []any{int16(-300), nil, int16(300)}},
	{"i32", gruby.Int32(), []any{int32(-70000), int32(70000), nil}},
	{"i64", gruby.Int64(), []any{int64(1), int64(-2), int64(3)}},
	{"u8", gruby.UInt8(), []any{uint8(0), uint8(255), nil}},
	{"u16", gruby.UInt16(), []any{uint16(0), nil, uint16(65535)}},
	{"u32", gruby.UInt32(), []any{uint32(4000000000), nil, uint32(1)}},
	{"u64", gruby.UInt64(), []any{uint64(1), uint64(2), nil}},
	{"f32", gruby.Float32(), []any{float32(1.5), nil, float32(-2.25)}},
	{"f64", gruby.Float64(), []any{1.25, -3.5, nil}},
	{"b", gruby.Boolean(), []any{true, nil, false}},
	{"s", gruby.StringType(), []any{"alpha", "", nil}},
	{"t", gruby.Timestamp(), []any{ts(2026, 7, 4, 12, 0, 0, 500), nil, ts(2000, 1, 1, 0, 0, 0, 0)}},
	{"d", gruby.Date(), []any{day(2026, 7, 4), day(1970, 1, 2), nil}},
	{"dec", gruby.Decimal128(10, 2), []any{"123.45", nil, "-0.01"}},
	{"list", gruby.ListOf(gruby.Int64()), []any{[]any{int64(1), int64(2)}, []any{}, nil}},
	{"struct", gruby.StructOf(
		gruby.NewField("x", gruby.Int64()),
		gruby.NewField("y", gruby.StringType()),
	), []any{
		map[string]any{"x": int64(7), "y": "seven"},
		map[string]any{"x": int64(8), "y": "eight"},
		nil,
	}},
}

// buildSampleTable materializes the kitchen-sink table used across the tests.
func buildSampleTable(t *testing.T) *gruby.Table {
	t.Helper()
	fields := make([]*gruby.Field, len(sampleColumns))
	cols := make([]*gruby.Array, len(sampleColumns))
	for i, c := range sampleColumns {
		fields[i] = gruby.NewField(c.name, c.dt)
		arr, err := gruby.NewArrayOf(c.dt, c.values)
		if err != nil {
			t.Fatalf("build column %s: %v", c.name, err)
		}
		cols[i] = arr
	}
	tbl, err := gruby.NewTable(gruby.NewSchema(fields...), cols)
	if err != nil {
		t.Fatalf("build sample table: %v", err)
	}
	return tbl
}

// assertSameTable checks that got matches the sample table in shape, schema
// field names/types and every value (via ToHash).
func assertSameTable(t *testing.T, got *gruby.Table) {
	t.Helper()
	if got.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", got.NumRows())
	}
	if int(got.NumColumns()) != len(sampleColumns) {
		t.Fatalf("NumColumns = %d, want %d", got.NumColumns(), len(sampleColumns))
	}
	want := buildSampleTable(t)
	wantHash := want.ToHash()
	gotHash := got.ToHash()
	for _, c := range sampleColumns {
		if !reflect.DeepEqual(gotHash[c.name], wantHash[c.name]) {
			t.Errorf("column %q:\n got %#v\nwant %#v", c.name, gotHash[c.name], wantHash[c.name])
		}
	}
	// Schema field names and type names must be preserved by the round-trip.
	gs := got.Schema()
	ws := want.Schema()
	if gs.NumFields() != ws.NumFields() {
		t.Fatalf("schema fields = %d, want %d", gs.NumFields(), ws.NumFields())
	}
	for i := 0; i < ws.NumFields(); i++ {
		if gs.Field(i).Name() != ws.Field(i).Name() {
			t.Errorf("field %d name = %q, want %q", i, gs.Field(i).Name(), ws.Field(i).Name())
		}
		if gs.Field(i).DataType().Name() != ws.Field(i).DataType().Name() {
			t.Errorf("field %d (%s) type = %q, want %q", i, ws.Field(i).Name(),
				gs.Field(i).DataType().Name(), ws.Field(i).DataType().Name())
		}
	}
}

// errKind reports whether err is an [*Error] of the given kind.
func errKind(err error, kind ErrorKind) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == kind
}
