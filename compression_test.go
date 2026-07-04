// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import "testing"

func TestCompressionString(t *testing.T) {
	cases := map[Compression]string{
		Uncompressed:    "uncompressed",
		Snappy:          "snappy",
		Gzip:            "gzip",
		Zstd:            "zstd",
		Compression(99): "unknown",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("Compression(%d).String() = %q, want %q", int(c), got, want)
		}
	}
}

func TestCompressionCodec(t *testing.T) {
	for _, c := range []Compression{Uncompressed, Snappy, Gzip, Zstd} {
		if _, err := c.codec(); err != nil {
			t.Errorf("codec(%s) unexpected error: %v", c, err)
		}
	}
	if _, err := Compression(99).codec(); !errKind(err, KindArgument) {
		t.Errorf("codec(bad) error = %v, want KindArgument", err)
	}
}

func TestParseCompression(t *testing.T) {
	cases := map[string]Compression{
		"":             Uncompressed,
		"none":         Uncompressed,
		"uncompressed": Uncompressed,
		"snappy":       Snappy,
		":snappy":      Snappy,
		"GZIP":         Gzip,
		"gz":           Gzip,
		" zstd ":       Zstd,
	}
	for in, want := range cases {
		got, err := ParseCompression(in)
		if err != nil {
			t.Errorf("ParseCompression(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseCompression(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseCompression("lzo"); !errKind(err, KindArgument) {
		t.Errorf("ParseCompression(bad) error = %v, want KindArgument", err)
	}
}
