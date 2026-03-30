package markdown_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dsparkman/mobi"
	"github.com/dsparkman/mobi/markdown"
)

// ── Helper ───────────────────────────────────────────────────────────────────

func convert(t *testing.T, src string, opts ...markdown.Option) {
	t.Helper()
	book, err := markdown.NewConverter(opts...).ConvertBytes([]byte(src))
	if err != nil {
		t.Fatalf("ConvertBytes error: %v", err)
	}
	db, err := book.Realize()
	if err != nil {
		t.Fatalf("Realize error: %v", err)
	}
	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("output is empty")
	}
}

func convertAndInspect(t *testing.T, src string, opts ...markdown.Option) []byte {
	t.Helper()
	book, err := markdown.NewConverter(opts...).ConvertBytes([]byte(src))
	if err != nil {
		t.Fatalf("ConvertBytes error: %v", err)
	}
	db, err := book.Realize()
	if err != nil {
		t.Fatalf("Realize error: %v", err)
	}
	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	return buf.Bytes()
}

// ── Basic conversion ─────────────────────────────────────────────────────────

func TestConvert_MinimalDocument(t *testing.T) {
	convert(t, "# Hello\n\nThis is a paragraph.\n")
}

func TestConvert_EmptyDocument(t *testing.T) {
	// An empty document should still produce a valid (if minimal) MOBI.
	convert(t, "", markdown.WithTitle("Empty"))
}

func TestConvert_MultipleChapters(t *testing.T) {
	md := `
# Chapter One
First chapter content.

# Chapter Two
Second chapter content.

# Chapter Three
Third chapter content.
`
	convert(t, md)
}

// ── Front-matter extraction ───────────────────────────────────────────────────

func TestConvert_FrontMatterTitle(t *testing.T) {
	md := `---
title: "The Gopher's Journey"
author: "Alice Coder"
language: "en"
---

# Introduction

Content here.
`
	raw := convertAndInspect(t, md)
	if !bytes.Contains(raw, []byte("The Gopher's Journey")) {
		t.Error("front-matter title not found in output")
	}
	if !bytes.Contains(raw, []byte("Alice Coder")) {
		t.Error("front-matter author not found in output")
	}
}

func TestConvert_FrontMatterOverriddenByOption(t *testing.T) {
	// Explicit option must win over front-matter.
	md := `---
title: "Front Matter Title"
author: "FM Author"
---

Content.
`
	raw := convertAndInspect(t, md,
		markdown.WithTitle("Option Title"),
		markdown.WithAuthor("Option Author"),
	)
	if !bytes.Contains(raw, []byte("Option Title")) {
		t.Error("option title not found in output")
	}
	if bytes.Contains(raw, []byte("Front Matter Title")) {
		t.Error("front-matter title should have been overridden by option")
	}
}

func TestConvert_FrontMatterDate(t *testing.T) {
	md := `---
title: "Dated Book"
date: "2001-09-11"
---
Content.
`
	raw := convertAndInspect(t, md)
	// The published date should appear somewhere in the EXTH section.
	if !bytes.Contains(raw, []byte("2001")) {
		t.Error("published date year not found in output")
	}
}

// ── Chapter splitting ─────────────────────────────────────────────────────────

func TestConvert_SplitNone(t *testing.T) {
	md := `
# Part One
## Section A
Content.
## Section B
Content.
# Part Two
Content.
`
	// SplitNone → single chapter, no NCX hierarchy
	convert(t, md, markdown.WithSplitLevel(markdown.SplitNone))
}

func TestConvert_SplitH1_SubH2(t *testing.T) {
	md := `
# Part One

Intro paragraph.

## Section 1.1

First section.

## Section 1.2

Second section.

# Part Two

## Section 2.1

Third section.
`
	raw := convertAndInspect(t, md,
		markdown.WithSplitLevel(markdown.SplitH1),
		markdown.WithSubSplitLevel(markdown.SplitH2),
	)

	// Sub-chapter titles must appear in the CNCX string pool.
	for _, title := range []string{"Section 1.1", "Section 1.2", "Section 2.1"} {
		if !bytes.Contains(raw, []byte(title)) {
			t.Errorf("sub-chapter title %q not found in output", title)
		}
	}
}

func TestConvert_SplitH2(t *testing.T) {
	md := `
# The Book
## Chapter A
Content A.
## Chapter B
Content B.
`
	convert(t, md, markdown.WithSplitLevel(markdown.SplitH2))
}

// ── Metadata options ─────────────────────────────────────────────────────────

func TestConvert_AllMetadata(t *testing.T) {
	raw := convertAndInspect(t,
		"# Book\n\nContent.",
		markdown.WithTitle("Test Book"),
		markdown.WithAuthors("Author One", "Author Two"),
		markdown.WithPublisher("Test Publisher"),
		markdown.WithDescription("A thorough test"),
		markdown.WithISBN("978-0-00-000001-7"),
		markdown.WithRights("© 2026"),
		markdown.WithLanguage("en"),
		markdown.WithClippingLimit(10),
	)

	checks := []string{
		"Test Book",
		"Author One",
		"Author Two",
		"Test Publisher",
		"A thorough test",
		"978-0-00-000001-7",
	}
	for _, s := range checks {
		if !bytes.Contains(raw, []byte(s)) {
			t.Errorf("%q not found in output", s)
		}
	}
}

// ── Compression ───────────────────────────────────────────────────────────────

func TestConvert_PalmDocCompressionSmaller(t *testing.T) {
	md := "# Test\n\n" + strings.Repeat("The quick brown fox jumps over the lazy dog. ", 200)

	rawComp := convertAndInspect(t, md) // default: PalmDoc
	rawNone := convertAndInspect(t, md, markdown.WithCompression(mobi.CompressionNone))

	if len(rawComp) >= len(rawNone) {
		t.Errorf("compressed (%d bytes) should be < uncompressed (%d bytes)",
			len(rawComp), len(rawNone))
	}
}

// ── HTML sanitisation ─────────────────────────────────────────────────────────

func TestConvert_SelfClosingTags(t *testing.T) {
	// After sanitisation, <br> must become <br/> in the output.
	md := "Line one  \nLine two\n"
	// Two trailing spaces in Markdown → <br> in gomarkdown output.
	raw := convertAndInspect(t, md)
	// The raw <br> (without slash) must not appear; <br/> must.
	if bytes.Contains(raw, []byte("<br>")) {
		t.Error("bare <br> found in output — should be self-closed <br/>")
	}
}

// ── io.Reader entry point ─────────────────────────────────────────────────────

func TestConvert_FromReader(t *testing.T) {
	md := "# From Reader\n\nContent from an io.Reader."
	r := strings.NewReader(md)
	book, err := markdown.NewConverter(markdown.WithTitle("Reader Test")).Convert(r)
	if err != nil {
		t.Fatalf("Convert(reader) error: %v", err)
	}
	db, err := book.Realize()
	if err != nil {
		t.Fatalf("Realize error: %v", err)
	}
	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("output is empty")
	}
}

// ── Reuse converter ───────────────────────────────────────────────────────────

func TestConvert_ConverterReuse(t *testing.T) {
	// Same Converter instance must produce valid output for multiple documents.
	c := markdown.NewConverter(
		markdown.WithAuthor("Shared Author"),
		markdown.WithLanguage("en"),
	)
	docs := []string{
		"# Doc One\nContent one.",
		"# Doc Two\nContent two.",
		"# Doc Three\nContent three.",
	}
	for _, doc := range docs {
		book, err := c.ConvertBytes([]byte(doc))
		if err != nil {
			t.Fatalf("ConvertBytes(%q): %v", doc[:10], err)
		}
		db, err := book.Realize()
		if err != nil {
			t.Fatalf("Realize: %v", err)
		}
		var buf bytes.Buffer
		if err := db.Write(&buf); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
}
