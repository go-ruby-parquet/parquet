// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import "github.com/apache/arrow-go/v18/parquet"

// DefaultRowGroupSize is the number of rows per row group used when no
// [WithRowGroupSize] option is given, matching arrow-go's default row-group
// length.
const DefaultRowGroupSize int64 = 1024 * 1024

// WriteOption configures the Parquet writer, mirroring the knobs red-parquet
// exposes through Parquet::WriterProperties (compression, row-group size and
// dictionary encoding).
type WriteOption func(*writeConfig)

// writeConfig is the resolved set of writer settings.
type writeConfig struct {
	compression  Compression
	rowGroupSize int64
	dictionary   bool
}

// defaultWriteConfig returns red-parquet's defaults: Snappy compression, the
// default row-group size and dictionary encoding enabled.
func defaultWriteConfig() writeConfig {
	return writeConfig{
		compression:  Snappy,
		rowGroupSize: DefaultRowGroupSize,
		dictionary:   true,
	}
}

// WithCompression sets the column-chunk compression codec
// (Parquet::WriterProperties#set_compression).
func WithCompression(c Compression) WriteOption {
	return func(cfg *writeConfig) { cfg.compression = c }
}

// WithRowGroupSize sets the maximum number of rows per row group
// (Parquet::ArrowFileWriter#write_table chunk_size). A non-positive size falls
// back to [DefaultRowGroupSize].
func WithRowGroupSize(n int64) WriteOption {
	return func(cfg *writeConfig) {
		if n <= 0 {
			n = DefaultRowGroupSize
		}
		cfg.rowGroupSize = n
	}
}

// WithDictionary enables or disables dictionary encoding
// (Parquet::WriterProperties#set_enable_dictionary).
func WithDictionary(enabled bool) WriteOption {
	return func(cfg *writeConfig) { cfg.dictionary = enabled }
}

// resolve applies the options over the defaults and returns the arrow-go
// WriterProperties they describe.
func (cfg writeConfig) resolve() (*parquet.WriterProperties, error) {
	codec, err := cfg.compression.codec()
	if err != nil {
		return nil, err
	}
	return parquet.NewWriterProperties(
		parquet.WithCompression(codec),
		parquet.WithMaxRowGroupLength(cfg.rowGroupSize),
		parquet.WithDictionaryDefault(cfg.dictionary),
	), nil
}

// buildWriteConfig folds the options onto the defaults.
func buildWriteConfig(opts []WriteOption) writeConfig {
	cfg := defaultWriteConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
