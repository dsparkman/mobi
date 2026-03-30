# mobi — a rock-solid Go library for writing KF8/AZW3 Kindle books

A fork of [leotaku/mobi](https://github.com/leotaku/mobi) with no
backwards-compatibility constraints. Every change that improves the API
or spec coverage is in scope.

## What's different from the upstream

| Feature | leotaku/mobi | this fork |
|---------|-------------|-----------|
| `Realize()` return | `pdb.Database` (panics on error) | `(pdb.Database, error)` |
| Book construction | bare struct literal | `NewBook()` + functional options |
| Input validation | none before binary output | validated at construction |
| SubChapters / nested NCX | not supported | supported (depth 2) |
| PalmDOC LZ77 compression | not supported | `WithCompression(CompressionPalmDoc)` |
| EXTH description | missing | `WithDescription()` |
| EXTH ISBN | missing | `WithISBN()` |
| EXTH rights | missing | `WithRights()` |
| EXTH clipping limit | missing | `WithClippingLimit(n)` |
| EXTH TTS disable | missing | `WithTTSDisabled(bool)` |
| EXTH start-reading offset | missing | `WithStartChapter(idx)` |
| Language locale regions | stripped | preserved |

## Quick start

```go
import (
    "github.com/dsparkman/mobi"
    "golang.org/x/text/language"
)

// Build chapters
ch1 := mobi.NewChapter("The Beginning").
    AddContent("<p>It was a dark and stormy night.</p>").
    AddSubChapter("Act I", "<p>The protagonist awoke.</p>").
    Build()

ch2 := mobi.SimpleChapter("The End", "<p>And they all used Go.</p>")

// Construct and validate the book
book, err := mobi.NewBook("My Novel", uniqueID,
    mobi.WithAuthors("Jane Smith"),
    mobi.WithPublisher("Gopher Press"),
    mobi.WithDescription("A tale of two goroutines"),
    mobi.WithLanguage(language.English),
    mobi.WithCompression(mobi.CompressionPalmDoc), // ~40% smaller
    mobi.WithClippingLimit(10),
    mobi.WithStartChapter(1),                       // skip cover on open
    mobi.WithCoverImage(coverImg, thumbImg),
    mobi.WithChapters(ch1, ch2),
)
if err != nil {
    log.Fatal(err)
}

// Produce binary output
db, err := book.Realize()
if err != nil {
    log.Fatal(err)
}
f, _ := os.Create("novel.azw3")
defer f.Close()
if err := db.Write(f); err != nil {
    log.Fatal(err)
}

// For sideloading: copy cover thumbnail
fmt.Println("thumbnail:", book.GetThumbFilename())
```

## Sideloading to Kindle

1. Connect Kindle via USB.
2. Copy `novel.azw3` → `Kindle/documents/`
3. Optional cover art: copy a JPEG named `book.GetThumbFilename()` →
   `Kindle/system/thumbnails/`
4. Eject and open the book.

## Subpackages

The fork retains and modifies all upstream subpackages:

- `pdb` — PalmDB container read/write
- `records` — KF8 binary record constructors (chunk, skeleton, NCX, FDST, image)
- `types` — EXTH record type constants and magic record constructors
- `jfif` — JFIF APP0 marker injection for Kindle-compatible JPEG encoding

The `records.ChapterInfo` struct gains a `Depth int` field (0 = top-level,
1 = sub-chapter) which drives the NCX depth-2 index generation.

The `records.NewCompressedTextRecord()` constructor is added alongside the
existing `NewTextRecord()` to support PalmDOC-compressed records.

## Known limitations

- NCX depth is capped at 2 (part → chapter). Depth 3+ would require a more
  complex TAGX section and is not implemented.
- Huff/CDIC compression (type 17480, the proprietary Mobipocket scheme) is
  not implemented. PalmDOC LZ77 gives ~40% savings for prose which is
  sufficient for sideloaded books.
- Audio/video AUDI/VIDE records are not supported.
- Old Kindle hardware (Gen 1, 2, DX) without KF8 support is not targeted.

## Running tests

```sh
go test ./...
```

The compression package has a round-trip fuzzer:

```sh
go test -fuzz=FuzzPalmDoc ./...
```

## License

MIT — same as the upstream leotaku/mobi. Attribution to Leo Gaskin for the
original implementation.
