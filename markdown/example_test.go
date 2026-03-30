package markdown_test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dsparkman/mobi/markdown"
)

// ExampleConverter_ConvertFile shows the simplest possible file conversion.
func ExampleConverter_ConvertFile() {
	book, err := markdown.NewConverter(
		markdown.WithAuthor("Jane Smith"),
		markdown.WithLanguage("en"),
	).ConvertFile("novel.md")
	if err != nil {
		log.Fatal(err)
	}

	db, err := book.Realize()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("novel.azw3")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if err := db.Write(f); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Written: novel.azw3")
}

// ExampleConverter_Convert_stream demonstrates converting from any io.Reader —
// useful for piped input, HTTP responses, or os.Stdin.
func ExampleConverter_Convert_stream() {
	// Could equally be os.Stdin, an http.Response.Body, a gzip.Reader, etc.
	r := strings.NewReader(`---
title: "Stream Example"
author: "Alice"
---

# Chapter One

Hello from a stream.
`)

	book, err := markdown.NewConverter().Convert(r)
	if err != nil {
		log.Fatal(err)
	}

	db, err := book.Realize()
	if err != nil {
		log.Fatal(err)
	}

	f, _ := os.Create("stream.azw3")
	defer func() { _ = f.Close() }()
	_ = db.Write(f)
}

// ExampleConverter_Convert_httpBody shows fetching Markdown from a URL
// and converting the response body directly.
func ExampleConverter_Convert_httpBody() {
	resp, err := http.Get("https://example.com/article.md")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Tee the body through a size limit to protect against huge responses.
	limited := io.LimitReader(resp.Body, 10<<20) // 10 MB cap

	book, err := markdown.NewConverter(
		markdown.WithTitle("Remote Article"),
		markdown.WithLanguage("en"),
	).Convert(limited)
	if err != nil {
		log.Fatal(err)
	}

	db, err := book.Realize()
	if err != nil {
		log.Fatal(err)
	}

	f, _ := os.Create("article.azw3")
	defer func() { _ = f.Close() }()
	_ = db.Write(f)
}

// ExampleConverter_fullMetadata demonstrates every available option.
func ExampleConverter_fullMetadata() {
	md := `---
title: "The Complete Reference"
---

# Introduction

Opening material.

## Background

Historical context.

# Part One: Fundamentals

Core concepts.

## Chapter 1.1

First sub-chapter.

## Chapter 1.2

Second sub-chapter.
`

	book, err := markdown.NewConverter(
		markdown.WithTitle("The Complete Reference"),
		markdown.WithAuthors("Jane Smith", "Bob Jones"),
		markdown.WithPublisher("Gopher Press"),
		markdown.WithDescription("A comprehensive guide to the topic"),
		markdown.WithISBN("978-0-00-000000-0"),
		markdown.WithRights("© 2026 The Authors"),
		markdown.WithLanguage("en"),
		markdown.WithPublishedDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		markdown.WithClippingLimit(10),
		markdown.WithSplitLevel(markdown.SplitH1),
		markdown.WithSubSplitLevel(markdown.SplitH2),
		markdown.WithCoverImage("./cover.jpg"),
		markdown.WithCustomCSS(`
			/* Publisher house style */
			p { text-indent: 1em; }
			p.first { text-indent: 0; }
		`),
	).ConvertBytes([]byte(md))
	if err != nil {
		log.Fatal(err)
	}

	db, err := book.Realize()
	if err != nil {
		log.Fatal(err)
	}

	f, _ := os.Create("reference.azw3")
	defer func() { _ = f.Close() }()
	_ = db.Write(f)
}

// ExampleConverter_reuseForBatch shows converting many files with one Converter.
func ExampleConverter_reuseForBatch() {
	c := markdown.NewConverter(
		markdown.WithAuthor("Series Author"),
		markdown.WithPublisher("My Press"),
		markdown.WithLanguage("en"),
	)

	files := []struct{ in, out string }{
		{"book1.md", "book1.azw3"},
		{"book2.md", "book2.azw3"},
		{"book3.md", "book3.azw3"},
	}

	for _, file := range files {
		book, err := c.ConvertFile(file.in)
		if err != nil {
			log.Printf("skip %s: %v", file.in, err)
			continue
		}
		db, err := book.Realize()
		if err != nil {
			log.Printf("realize %s: %v", file.in, err)
			continue
		}
		f, err := os.Create(file.out)
		if err != nil {
			log.Printf("create %s: %v", file.out, err)
			continue
		}
		if err := db.Write(f); err != nil {
			log.Printf("write %s: %v", file.out, err)
		}
		_ = f.Close()
		fmt.Printf("✓ %s → %s\n", file.in, file.out)
	}
}
