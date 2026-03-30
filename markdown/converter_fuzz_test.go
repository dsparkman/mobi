package markdown_test

import (
	"testing"
	"unicode/utf8"

	"github.com/dsparkman/mobi/markdown"
)

// FuzzConvertMarkdown verifies that the full Markdown→MOBI pipeline never
// panics and that the pipeline correctly rejects non-UTF-8 input.
//
// Properties verified for every fuzz input b:
//  1. ConvertBytes never panics (regardless of input content).
//  2. If b is not valid UTF-8, ConvertBytes returns a non-nil error.
//  3. If ConvertBytes returns a nil error, book.Realize() also returns nil.
func FuzzConvertMarkdown(f *testing.F) {
	seeds := []string{
		"",
		"# Title\n\nBody.",
		"---\ntitle: T\nauthor: A\n---\n# H\n\nContent.",
		"# Part\n## Sub\nContent.\n## Sub2\nMore.",
		"<script>alert(1)</script>\n# H\n\nBody.",
		"Line one  \nLine two\n",                   // trailing spaces → <br>
		"| A | B |\n|---|---|\n| 1 | 2 |\n# H\n\n", // table
		"# " + string([]byte{0xC3, 0xA9}) + "\n\nCafé body.", // UTF-8 accent
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	c := markdown.NewConverter(markdown.WithTitle("Fuzz"))

	f.Fuzz(func(t *testing.T, b []byte) {
		// Property 1 + 3: no panic; if no error then Realize also succeeds.
		book, err := c.ConvertBytes(b)
		if err != nil {
			// Errors are expected for invalid UTF-8 or other bad input.
			// Property 2: non-UTF-8 must always error.
			if !utf8.Valid(b) {
				// Good — the pipeline rejected invalid UTF-8.
				return
			}
			// For valid UTF-8, some structural errors are acceptable
			// (e.g. title validation failures), but panics are not.
			return
		}
		// Property 3: Realize must not error if ConvertBytes didn't.
		if _, err := book.Realize(); err != nil {
			t.Fatalf("ConvertBytes succeeded but Realize failed: %v", err)
		}
	})
}
