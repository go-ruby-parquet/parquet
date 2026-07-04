// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import "testing"

func TestBuildWriteConfigDefaults(t *testing.T) {
	cfg := buildWriteConfig(nil)
	if cfg.compression != Snappy {
		t.Errorf("default compression = %v, want Snappy", cfg.compression)
	}
	if cfg.rowGroupSize != DefaultRowGroupSize {
		t.Errorf("default rowGroupSize = %d, want %d", cfg.rowGroupSize, DefaultRowGroupSize)
	}
	if !cfg.dictionary {
		t.Error("default dictionary should be enabled")
	}
}

func TestWriteOptions(t *testing.T) {
	cfg := buildWriteConfig([]WriteOption{
		WithCompression(Gzip),
		WithRowGroupSize(500),
		WithDictionary(false),
	})
	if cfg.compression != Gzip || cfg.rowGroupSize != 500 || cfg.dictionary {
		t.Errorf("options not applied: %+v", cfg)
	}
}

func TestWithRowGroupSizeNonPositive(t *testing.T) {
	cfg := buildWriteConfig([]WriteOption{WithRowGroupSize(0)})
	if cfg.rowGroupSize != DefaultRowGroupSize {
		t.Errorf("rowGroupSize(0) = %d, want default %d", cfg.rowGroupSize, DefaultRowGroupSize)
	}
	cfg = buildWriteConfig([]WriteOption{WithRowGroupSize(-5)})
	if cfg.rowGroupSize != DefaultRowGroupSize {
		t.Errorf("rowGroupSize(-5) = %d, want default", cfg.rowGroupSize)
	}
}

func TestResolveOK(t *testing.T) {
	if _, err := buildWriteConfig(nil).resolve(); err != nil {
		t.Errorf("resolve default: %v", err)
	}
}

func TestResolveBadCompression(t *testing.T) {
	cfg := buildWriteConfig([]WriteOption{WithCompression(Compression(99))})
	if _, err := cfg.resolve(); !errKind(err, KindArgument) {
		t.Errorf("resolve(bad) error = %v, want KindArgument", err)
	}
}
