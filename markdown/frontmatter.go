package markdown

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// frontMatter holds the subset of YAML front-matter fields recognised by the
// converter. Fields not listed here are silently ignored.
//
// A front-matter block must appear at the very top of the Markdown file,
// delimited by triple dashes:
//
//	---
//	title: "The Go Chronicles"
//	author: "Jane Smith"
//	authors:
//	  - "Jane Smith"
//	  - "Bob Coder"
//	publisher: "Gopher Press"
//	language: "en"
//	date: 2024-01-15
//	description: "A tale of two goroutines"
//	isbn: "978-0-00-000000-0"
//	rights: "© 2024 Jane Smith"
//	cover: "./cover.jpg"
//	subject: "Fiction / Technology"
//	---
type frontMatter struct {
	Title       string   `yaml:"title"`
	Author      string   `yaml:"author"`  // single-author shorthand
	Authors     []string `yaml:"authors"` // preferred for multiple authors
	Publisher   string   `yaml:"publisher"`
	Language    string   `yaml:"language"`
	Date        string   `yaml:"date"` // ISO 8601: "2024-01-15"
	Description string   `yaml:"description"`
	ISBN        string   `yaml:"isbn"`
	Rights      string   `yaml:"rights"`
	Cover       string   `yaml:"cover"` // relative path to cover image
	Subject     string   `yaml:"subject"`
}

// stripFrontMatter removes a leading YAML front-matter block (--- ... ---)
// from the Markdown source and returns the remaining content. If no
// front-matter block is present, src is returned unchanged.
func stripFrontMatter(src []byte) []byte {
	if !bytes.HasPrefix(src, []byte("---")) {
		return src
	}
	rest := src[3:]
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return src // malformed block — treat entire document as content
	}
	after := rest[idx+4:]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}
	return after
}

// mergeFrontMatter parses any YAML front-matter found in src and merges the
// values into cfg. The merge is additive and non-destructive: an explicit
// [Option] value always wins; front-matter only fills fields that are still
// at their zero value after options have been applied.
//
// Malformed YAML is treated as a non-fatal condition. The function returns
// the input cfg unchanged rather than aborting the conversion, because
// discarding a whole book over a stray bracket would be worse than missing
// metadata.
func mergeFrontMatter(src []byte, cfg config) (config, error) {
	fm, err := parseFrontMatter(src)
	if err != nil {
		// Non-fatal: malformed front-matter is silently ignored.
		return cfg, nil //nolint:nilerr — intentional degraded operation
	}
	if fm == nil {
		return cfg, nil
	}

	if cfg.title == "" && fm.Title != "" {
		cfg.title = fm.Title
	}
	if len(cfg.authors) == 0 {
		if len(fm.Authors) > 0 {
			cfg.authors = fm.Authors
		} else if fm.Author != "" {
			cfg.authors = []string{fm.Author}
		}
	}
	if cfg.publisher == "" && fm.Publisher != "" {
		cfg.publisher = fm.Publisher
	}
	// Only override the default language ("en") if the front-matter
	// provides an explicit value.
	if cfg.language == "en" && fm.Language != "" {
		cfg.language = fm.Language
	}
	if cfg.description == "" && fm.Description != "" {
		cfg.description = fm.Description
	}
	if cfg.isbn == "" && fm.ISBN != "" {
		cfg.isbn = fm.ISBN
	}
	if cfg.rights == "" && fm.Rights != "" {
		cfg.rights = fm.Rights
	}
	if cfg.subject == "" && fm.Subject != "" {
		cfg.subject = fm.Subject
	}
	if cfg.coverImagePath == "" && fm.Cover != "" {
		cfg.coverImagePath = fm.Cover
	}
	if cfg.publishedDate.IsZero() && fm.Date != "" {
		if t, err := parseDate(fm.Date); err == nil {
			cfg.publishedDate = t
		}
	}

	return cfg, nil
}

// parseFrontMatter extracts and unmarshals the YAML block if one is present.
// Returns nil (not an error) when no front-matter block exists.
func parseFrontMatter(src []byte) (*frontMatter, error) {
	if !bytes.HasPrefix(src, []byte("---")) {
		return nil, nil
	}
	rest := src[3:]
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil, nil
	}
	block := rest[:idx]

	var fm frontMatter
	if err := yaml.Unmarshal(block, &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}

// parseDate attempts to parse a date string using several common ISO 8601 and
// human-readable formats. Returns an error if none match.
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"January 2, 2006",
		"Jan 2, 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date format: %q", s)
}
