package markdown

import (
	"testing"
	"time"
)

func TestStripFrontMatter_WithBlock(t *testing.T) {
	src := []byte("---\ntitle: Test\n---\n# Content\n")
	got := stripFrontMatter(src)
	want := "# Content\n"
	if string(got) != want {
		t.Errorf("stripFrontMatter:\nwant %q\ngot  %q", want, got)
	}
}

func TestStripFrontMatter_NoBlock(t *testing.T) {
	src := []byte("# Just Content\n\nNo front matter here.\n")
	got := stripFrontMatter(src)
	if string(got) != string(src) {
		t.Error("stripFrontMatter mutated document with no front-matter")
	}
}

func TestParseFrontMatter_AllFields(t *testing.T) {
	src := []byte(`---
title: "My Book"
author: Single Author
authors:
  - Author One
  - Author Two
publisher: "Test Press"
language: "de"
date: "2023-06-15"
description: "A test description"
isbn: "978-0-00-000000-0"
rights: "© 2023"
cover: "./cover.jpg"
subject: "Testing"
---
Content.
`)
	fm, err := parseFrontMatter(src)
	if err != nil {
		t.Fatalf("parseFrontMatter error: %v", err)
	}
	if fm == nil {
		t.Fatal("expected front-matter, got nil")
	}
	if fm.Title != "My Book" {
		t.Errorf("title: want %q, got %q", "My Book", fm.Title)
	}
	if len(fm.Authors) != 2 {
		t.Errorf("authors: want 2, got %d", len(fm.Authors))
	}
	if fm.Language != "de" {
		t.Errorf("language: want %q, got %q", "de", fm.Language)
	}
	if fm.Cover != "./cover.jpg" {
		t.Errorf("cover: want %q, got %q", "./cover.jpg", fm.Cover)
	}
}

func TestMergeFrontMatter_OptionWins(t *testing.T) {
	src := []byte("---\ntitle: FM Title\nauthor: FM Author\n---\nContent.\n")
	cfg := defaultConfig()
	cfg.title = "Option Title" // explicit option set

	merged, err := mergeFrontMatter(src, cfg)
	if err != nil {
		t.Fatalf("mergeFrontMatter error: %v", err)
	}
	if merged.title != "Option Title" {
		t.Errorf("option title should win: want %q, got %q", "Option Title", merged.title)
	}
}

func TestMergeFrontMatter_FMFillsEmpty(t *testing.T) {
	src := []byte("---\ntitle: FM Title\nauthor: FM Author\n---\nContent.\n")
	cfg := defaultConfig() // title is ""

	merged, err := mergeFrontMatter(src, cfg)
	if err != nil {
		t.Fatalf("mergeFrontMatter error: %v", err)
	}
	if merged.title != "FM Title" {
		t.Errorf("FM title should fill empty: want %q, got %q", "FM Title", merged.title)
	}
	if len(merged.authors) != 1 || merged.authors[0] != "FM Author" {
		t.Errorf("FM author should fill empty: got %v", merged.authors)
	}
}

func TestParseDate_Formats(t *testing.T) {
	cases := []struct {
		input string
		year  int
	}{
		{"2023-06-15", 2023},
		{"2023-06-15T12:00:00Z", 2023},
		{"January 1, 2000", 2000},
		{"Jan 1, 2000", 2000},
	}
	for _, tc := range cases {
		got, err := parseDate(tc.input)
		if err != nil {
			t.Errorf("parseDate(%q): %v", tc.input, err)
			continue
		}
		if got.Year() != tc.year {
			t.Errorf("parseDate(%q): want year %d, got %d", tc.input, tc.year, got.Year())
		}
	}
}

func TestParseDate_Invalid(t *testing.T) {
	_, err := parseDate("not a date")
	if err == nil {
		t.Error("expected error for invalid date string")
	}
}

func TestMergeFrontMatter_MalformedYAML(t *testing.T) {
	// Malformed YAML front-matter must not abort the conversion.
	src := []byte("---\ntitle: [unclosed\n---\nContent.\n")
	cfg := defaultConfig()
	cfg.title = "Fallback"
	merged, err := mergeFrontMatter(src, cfg)
	if err != nil {
		t.Errorf("malformed YAML front-matter should be non-fatal, got error: %v", err)
	}
	if merged.title != "Fallback" {
		t.Errorf("title should be unchanged: want %q, got %q", "Fallback", merged.title)
	}
	_ = time.Now() // suppress import warning in minimal build
}
