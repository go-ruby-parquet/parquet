<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-parquet/brand/main/social/go-ruby-parquet-parquet.png" alt="go-ruby-parquet/parquet" width="720"></p>

# parquet — go-ruby-parquet

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-parquet.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`red-parquet`](https://arrow.apache.org/docs/ruby/)
gem** — reading and writing Apache Parquet files as Apache Arrow tables. The real
`red-parquet` gem binds the C++ library **libparquet** through GObject
introspection over Apache Arrow, so it cannot ship inside a CGO-free static
binary. This package mirrors `red-parquet`'s observable Ruby surface —
`Parquet::ArrowFileReader`, `Parquet::ArrowFileWriter`, the `Parquet::Writer`
convenience, `Table#save` / `Table.load` format dispatch and the
`Parquet::*Error` tree — on top of the Parquet reader/writer of
[`github.com/apache/arrow-go/v18/parquet`](https://github.com/apache/arrow-go)
and its Arrow bridge `parquet/pqarrow`, the official pure-Go Apache Parquet
implementation. It **does not reimplement the Parquet format**; it re-presents
arrow-go's Parquet stack through Ruby's naming and semantics.

It pairs with [go-ruby-arrow](https://github.com/go-ruby-arrow/arrow): the tables
this library reads and writes are **go-ruby-arrow `Table`s**, so the two compose
at the Ruby level — build a `Table` with go-ruby-arrow, persist it here, load it
back, hand it to any go-ruby-arrow consumer unchanged. It is a sibling of
[go-ruby-marshal](https://github.com/go-ruby-marshal/marshal) and the other
`go-ruby-*` satellite libraries, and a backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby).

> **Consumes, does not reinvent.** The columnar decode/encode, row groups,
> compression codecs and Parquet metadata all come from `arrow-go`. The value
> this package adds is the faithful Ruby surface and the go-ruby-arrow `Table`
> interop, verified wire-compatible with `arrow-go`'s canonical
> `pqarrow.FileReader`/writer in both directions.

## Install

```sh
go get github.com/go-ruby-parquet/parquet
```

## Usage

```go
package main

import (
	"bytes"
	"fmt"

	arrow "github.com/go-ruby-arrow/arrow"
	parquet "github.com/go-ruby-parquet/parquet"
)

func main() {
	schema := arrow.NewSchema(
		arrow.NewField("id", arrow.Int64()),
		arrow.NewField("name", arrow.StringType()),
	)
	id, _ := arrow.NewArrayOf(arrow.Int64(), []any{int64(1), int64(2), nil})
	name, _ := arrow.NewArrayOf(arrow.StringType(), []any{"a", "b", "c"})
	table, _ := arrow.NewTable(schema, []*arrow.Array{id, name})

	// Write the Arrow table to Parquet (Zstd, 1000-row row groups).
	var buf bytes.Buffer
	_ = parquet.WriteTableTo(&buf, table,
		parquet.WithCompression(parquet.Zstd),
		parquet.WithRowGroupSize(1000))

	// Read it back as a go-ruby-arrow Table.
	back, _ := parquet.ReadTableBytes(buf.Bytes())
	fmt.Println(back.NumRows(), back.NumColumns()) // 3 2

	col, _ := back.Column("name")
	v, _ := col.Get(2)
	fmt.Println(v) // c
}
```

## Ruby-to-Go mapping

| Ruby (`red-parquet`)            | Go (this package) |
| ------------------------------- | ----------------- |
| `Parquet::ArrowFileReader.new`  | `NewArrowFileReader` (IO) / `OpenArrowFileReader` (path) |
| `#read_table`                   | `(*ArrowFileReader).ReadTable` |
| `#read_row_group(i)`            | `(*ArrowFileReader).ReadRowGroup` |
| `#n_rows` / `#n_row_groups`     | `NumRows` / `NumRowGroups` |
| `#schema`                       | `(*ArrowFileReader).Schema` |
| `Parquet::ArrowFileWriter.new`  | `NewArrowFileWriter` — `Write`, `Close` |
| `Parquet::Writer.write(t, path)`| `WriteTable` / `WriteTableTo` |
| `Arrow::Table#save("x.parquet")`| `Save` |
| `Arrow::Table.load("x.parquet")`| `Load` |
| `Parquet::WriterProperties`     | `WithCompression` / `WithRowGroupSize` / `WithDictionary` |
| `Parquet::Error` tree           | `*Error` (`Kind` + `RubyClass()`) |

Compression symbols (`:uncompressed` / `:snappy` / `:gzip` / `:zstd`) map to the
[`Compression`](compression.go) constants and [`ParseCompression`].

## Round-trip & wire compatibility

Every Arrow column type — `Int8`..`Int64`, `UInt8`..`UInt64`, `Float32`/`Float64`,
`Boolean`, `String`, `Timestamp`, `Date32`, `Decimal128`, `List` and `Struct`,
each with nulls — round-trips through Parquet with values and schema preserved,
under every codec (uncompressed / snappy / gzip / zstd). Cross-library **wire
compatibility is verified, not asserted**: a file written here is read back by
`arrow-go`'s canonical `pqarrow.ReadTable`, and a file written by `arrow-go`'s
canonical `pqarrow.WriteTable` is read here — both directions, entirely in memory
(no network). Parquet is a little-endian on-disk format; `arrow-go` handles the
byte swap on big-endian targets (s390x), so files round-trip identically across
all six supported 64-bit arches.

## Tests & coverage

The suite is deterministic and dependency-light (no libparquet, **CGO=0**):
all-types/all-codecs round-trips, per-row-group reads, path and IO constructors,
the full error tree, and the cross-checks against `arrow-go`'s canonical
reader/writer that pin wire compatibility.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

CGO-free, `gofmt` + `go vet` clean, and green across the six 64-bit Go targets
(amd64, arm64, riscv64, loong64, ppc64le, s390x — the last big-endian) and three
OSes (Linux, macOS, Windows).

## Scope

This covers `red-parquet`'s Arrow-table read/write path plus compression, row
groups and dictionary encoding. It does not (yet) cover the lower-level
column-chunk statistics DSL, Bloom filters, encryption, or the Dataset API; those
are additive follow-ups on the same `arrow-go` foundation. See
[`doc.go`](doc.go) for the authoritative scope note.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-parquet/parquet authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
