// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

package parquet

import (
	"strings"

	"github.com/apache/arrow-go/v18/parquet/compress"
)

// Compression selects the Parquet column-chunk compression codec, mirroring
// red-parquet's per-file compression symbols (:uncompressed, :snappy, :gzip,
// :zstd) passed to Parquet::WriterProperties#set_compression.
type Compression int

const (
	// Uncompressed stores column chunks with no compression.
	Uncompressed Compression = iota
	// Snappy is red-parquet's default codec — fast, moderate ratio.
	Snappy
	// Gzip (DEFLATE) trades speed for a better ratio.
	Gzip
	// Zstd offers a strong ratio at competitive speed.
	Zstd
)

// String returns the Ruby symbol name of the codec (without the leading colon),
// e.g. "snappy", matching red-parquet's compression names.
func (c Compression) String() string {
	switch c {
	case Uncompressed:
		return "uncompressed"
	case Snappy:
		return "snappy"
	case Gzip:
		return "gzip"
	case Zstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// codec maps the Ruby-facing [Compression] onto arrow-go's compress.Compression.
func (c Compression) codec() (compress.Compression, error) {
	switch c {
	case Uncompressed:
		return compress.Codecs.Uncompressed, nil
	case Snappy:
		return compress.Codecs.Snappy, nil
	case Gzip:
		return compress.Codecs.Gzip, nil
	case Zstd:
		return compress.Codecs.Zstd, nil
	default:
		return 0, newError(KindArgument, "unknown compression %d", int(c))
	}
}

// ParseCompression resolves a red-parquet compression symbol/string (case- and
// leading-colon-insensitive, e.g. "snappy", ":gzip", "ZSTD") to a
// [Compression]. An unrecognized name yields an [*Error] of [KindArgument].
func ParseCompression(name string) (Compression, error) {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), ":")) {
	case "", "none", "uncompressed":
		return Uncompressed, nil
	case "snappy":
		return Snappy, nil
	case "gzip", "gz":
		return Gzip, nil
	case "zstd":
		return Zstd, nil
	default:
		return 0, newError(KindArgument, "unknown compression %q", name)
	}
}
