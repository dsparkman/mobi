# Migration guide: leotaku/mobi → dsparkman/mobi

This document describes every breaking change and how to update call sites.

---

## Summary of breaking changes

| Area | Old | New |
|------|-----|-----|
| `Realize()` signature | `func (Book) Realize() pdb.Database` | `func (*Book) Realize() (pdb.Database, error)` |
| Book construction | `mobi.Book{Title: "...", ...}` struct literal | `mobi.NewBook("...", uid, opts...)` constructor |
| `Chapter` type | exported struct with public fields | opaque struct with builder |
| `Chunk` type | exported struct with public `Body` field | exported struct, unchanged |
| `Chunks()` function | `mobi.Chunks(strings...)` | unchanged (compatibility alias) |
| `OverrideTemplate()` | method on `*Book` | `WithTemplate(tpl)` option to `NewBook` |

---

## 1. Book construction

**Before:**
```go
mb := mobi.Book{
    Title:       "My Book",
    Authors:     []string{"Alice"},
    Language:    language.English,
    UniqueID:    rand.Uint32(),
    Chapters:    []mobi.Chapter{ch},
    CSSFlows:    []string{myCSS},
    CoverImage:  coverImg,
    ThumbImage:  thumbImg,
}
db := mb.Realize()
```

**After:**
```go
book, err := mobi.NewBook("My Book", rand.Uint32(),
    mobi.WithAuthors("Alice"),
    mobi.WithLanguage(language.English),
    mobi.WithChapters(ch),
    mobi.WithCSSFlows(myCSS),
    mobi.WithCoverImage(coverImg, thumbImg),
)
if err != nil {
    return err
}
db, err := book.Realize()
if err != nil {
    return err
}
```

The key benefits: `NewBook` validates all inputs eagerly so you get a clean
error before any binary output is attempted, and `Realize` never panics.

---

## 2. Chapter construction

**Before:**
```go
ch := mobi.Chapter{
    Title:  "Chapter 1",
    Chunks: mobi.Chunks("<p>Hello</p>"),
}
```

**After (simple — one-liner):**
```go
ch := mobi.SimpleChapter("Chapter 1", "<p>Hello</p>")
```

**After (builder — multi-chunk or sub-chapters):**
```go
ch := mobi.NewChapter("Part I").
    AddContent("<p>Introduction paragraph.</p>").
    AddSubChapter("Section 1.1", "<p>First section body.</p>").
    AddSubChapter("Section 1.2", "<p>Second section body.</p>").
    Build()
```

The `Chunks()` function still works unchanged as a compatibility helper if you
are constructing `[]Chunk` slices manually, though `SimpleChapter` covers 95%
of use cases.

---

## 3. Realize() error handling

`Realize()` used to silently panic on template errors. It now returns
`(pdb.Database, error)`.

**Before:**
```go
db := mb.Realize()
f, _ := os.Create("out.azw3")
db.Write(f)
```

**After:**
```go
db, err := book.Realize()
if err != nil {
    log.Fatalf("realize: %v", err)
}
f, err := os.Create("out.azw3")
if err != nil {
    log.Fatal(err)
}
if err := db.Write(f); err != nil {
    log.Fatalf("write: %v", err)
}
```

---

## 4. Template override

**Before:**
```go
mb.OverrideTemplate(myTpl)
```

**After:**
```go
book, err := mobi.NewBook("Title", uid,
    mobi.WithTemplate(myTpl),
    ...
)
```

---

## 5. New metadata options (no migration needed)

These are additive — existing code that doesn't use them is unaffected:

```go
mobi.WithDescription("A gripping tale of binary formats")
mobi.WithISBN("978-0-00-000000-0")
mobi.WithRights("© 2026 The Author")
mobi.WithClippingLimit(10)        // 10% of text may be clipped
mobi.WithTTSDisabled(false)       // TTS enabled (default)
mobi.WithStartChapter(1)          // open at chapter index 1 (skip cover)
mobi.WithCompression(mobi.CompressionPalmDoc) // ~40% smaller files
```

---

## 6. Compression

PalmDOC LZ77 compression is now available. It reduces file size by roughly
35–45% for typical prose. Enable it with one option:

```go
mobi.WithCompression(mobi.CompressionPalmDoc)
```

`CompressionNone` (the previous implicit behaviour) remains the default to
avoid any change in output for callers who do not opt in.

---

## 7. Sub-chapters / nested NCX

The NCX table of contents now supports two levels. Use the builder:

```go
ch := mobi.NewChapter("Chapter 3: The Format").
    AddContent(introHTML).
    AddSubChapter("3.1 PalmDB container", section1HTML).
    AddSubChapter("3.2 MOBI header",      section2HTML).
    AddSubChapter("3.3 EXTH metadata",    section3HTML).
    Build()
```

The Kindle will show "Chapter 3: The Format" in the top-level TOC and
reveal the three sections when that entry is expanded.

---

## 8. Module path

Update your `go.mod` import path:

```
# Before
github.com/leotaku/mobi

# After
github.com/dsparkman/mobi
```

Run `go mod tidy` after updating the import path in all files.
