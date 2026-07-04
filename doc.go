// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-parquet/parquet authors

// Package parquet is a pure-Go (CGO=0), MRI-faithful implementation of the Ruby
// red-parquet gem's core surface — reading and writing Apache Parquet files as
// Apache Arrow tables.
//
// # Relationship to upstream
//
// The real red-parquet gem is a thin Ruby binding over the C++ library
// libparquet (via GObject introspection over Apache Arrow), so it cannot be
// shipped in a CGO-free static binary. This package mirrors red-parquet's
// observable Ruby surface — Parquet::ArrowFileReader, Parquet::ArrowFileWriter,
// the Parquet::Writer convenience, Table#save / Table.load format dispatch and
// the Parquet::*Error tree — on top of the Parquet reader/writer of
// [github.com/apache/arrow-go/v18/parquet] and its Arrow bridge
// [github.com/apache/arrow-go/v18/parquet/pqarrow], the official pure-Go Apache
// Parquet implementation. It does not reimplement the Parquet format; it
// re-presents arrow-go's Parquet stack through Ruby's naming and semantics so it
// can back an embedded Ruby (go-embedded-ruby / rbgo) with no cgo.
//
// # Interoperability with go-ruby-arrow
//
// The Arrow tables this package reads and writes are
// [github.com/go-ruby-arrow/arrow] Tables — the same type red-arrow is mapped
// to — so the two libraries compose at the Ruby level: build a Table with
// go-ruby-arrow, persist it here, load it back, and hand it to any go-ruby-arrow
// consumer unchanged.
//
// # Ruby-to-Go mapping
//
//	Parquet::ArrowFileReader -> *ArrowFileReader (ReadTable/ReadRowGroup/NumRows/…)
//	Parquet::ArrowFileWriter -> *ArrowFileWriter (Write/Close)
//	Parquet::Writer.write     -> WriteTable / WriteTableTo / Save
//	Arrow::Table.load         -> Load / ReadTable
//	Arrow::Table#save         -> Save
//	Parquet::Error (tree)     -> *Error (Kind + RubyClass mapping)
//
// # Compression and encoding
//
// The writer honours red-parquet's per-file compression symbols
// (:uncompressed / :snappy / :gzip / :zstd) via [WithCompression], the
// row-group size via [WithRowGroupSize], and dictionary encoding via
// [WithDictionary]. Every codec round-trips.
//
// # Wire compatibility
//
// A Parquet file written here is read back by arrow-go's canonical
// pqarrow.FileReader, and a file written by arrow-go's canonical writer is read
// here — both directions are verified by the differential tests, not asserted.
// Parquet is a little-endian on-disk format; arrow-go handles the byte swap on
// big-endian targets (s390x), so the same files round-trip identically across
// all six supported 64-bit arches.
package parquet
