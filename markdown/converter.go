package markdown

import (
	"cmp"
	"fmt"
	"image"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsparkman/mobi"
	"golang.org/x/text/language"
)

// Converter holds the configuration for a Markdown-to-MOBI conversion.
//
// A Converter is safe to reuse across multiple documents and safe for
// concurrent use: each call to [Converter.Convert], [Converter.ConvertFile],
// or [Converter.ConvertBytes] operates on an independent copy of the
// configuration. Options set at construction apply to every conversion unless
// overridden by YAML front-matter in the document (explicit options always
// take precedence over front-matter).
type Converter struct {
	cfg config
}

// NewConverter creates a Converter with the given [Option] values applied.
// All options are optional; sensible defaults are used for any omitted field.
//
//	c := markdown.NewConverter(
//	    markdown.WithAuthor("Jane Smith"),
//	    markdown.WithLanguage("en"),
//	    markdown.WithCompression(mobi.CompressionPalmDoc),
//	)
func NewConverter(opts ...Option) *Converter {
	c := &Converter{cfg: defaultConfig()}
	for _, o := range opts {
		o(&c.cfg)
	}
	return c
}

// Convert reads all bytes from r and converts the Markdown content to a
// [github.com/dsparkman/mobi.Book].
//
// The reader is consumed in full before any processing begins — true
// streaming is not possible because the Markdown parser and front-matter
// extractor both require random access. For untrusted or large inputs,
// wrap r with [io.LimitReader] to cap memory usage:
//
//	limited := io.LimitReader(resp.Body, 10<<20) // 10 MB cap
//	book, err := c.Convert(limited)
//
// Closing r is the caller's responsibility. Convert does not close it.
func (c *Converter) Convert(r io.Reader) (*mobi.Book, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("markdown: read input: %w", err)
	}
	return c.ConvertBytes(src)
}

// ConvertFile reads the Markdown file at path and converts it to a
// [github.com/dsparkman/mobi.Book].
//
// The file's directory is automatically used as the base for resolving
// relative image paths (cover image and inline images), unless overridden
// by [WithImageBaseDir].
func (c *Converter) ConvertFile(path string) (*mobi.Book, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("markdown: read file %q: %w", path, err)
	}
	cfg := c.cfg
	if cfg.imageBaseDir == "" {
		cfg.imageBaseDir = filepath.Dir(path)
	}
	return convertBytes(src, cfg)
}

// ConvertBytes converts a Markdown byte slice to a [github.com/dsparkman/mobi.Book].
//
// src must be valid UTF-8; the converter returns a descriptive error with the
// byte offset if any invalid sequence is detected.
func (c *Converter) ConvertBytes(src []byte) (*mobi.Book, error) {
	return convertBytes(src, c.cfg)
}

// convertBytes is the shared pipeline for all three public entry points.
func convertBytes(src []byte, cfg config) (*mobi.Book, error) {
	// Step 0: reject non-UTF-8 input before any processing. The MOBI format
	// mandates UTF-8 (header encoding field 65001); a bad sequence here would
	// produce a garbled or firmware-rejected output file.
	if err := validateUTF8(src); err != nil {
		return nil, err
	}

	// Step 1: render Markdown to HTML, stripping front-matter first.
	fullHTML, err := renderMarkdown(src, cfg)
	if err != nil {
		return nil, fmt.Errorf("markdown: render: %w", err)
	}

	// Step 1.5: embed local images referenced in the HTML.
	var inlineImages []image.Image
	if cfg.embedImages {
		var imgErr error
		fullHTML, inlineImages, imgErr = embedLocalImages(fullHTML, cfg.imageBaseDir, nil)
		if imgErr != nil {
			return nil, fmt.Errorf("markdown: embed images: %w", imgErr)
		}
	}

	// Step 2: merge YAML front-matter into cfg. Explicit options win;
	// front-matter only fills fields still at their zero value.
	cfg, err = mergeFrontMatter(src, cfg)
	if err != nil {
		return nil, fmt.Errorf("markdown: front-matter: %w", err)
	}

	// Step 3: split the HTML into a chapter/sub-chapter hierarchy.
	chapters, startChapterIdx, err := buildChapters(fullHTML, cfg)
	if err != nil {
		return nil, fmt.Errorf("markdown: chapter split: %w", err)
	}

	// Step 4: parse the BCP-47 language string into a language.Tag.
	lang, err := language.Parse(cfg.language)
	if err != nil {
		lang = language.English
	}

	// Step 5: assemble mobi.BookOption values from the resolved config.
	uid := cfg.uniqueID
	if uid == 0 {
		uid = rand.Uint32()
	}

	bookOpts := []mobi.BookOption{
		mobi.WithAuthors(cfg.authors...),
		mobi.WithPublisher(cfg.publisher),
		mobi.WithSubject(cfg.subject),
		mobi.WithLanguage(lang),
		mobi.WithCompression(cfg.compression),
		mobi.WithCSSFlows(buildCSS(cfg.customCSS)),
		mobi.WithChapters(chapters...),
	}
	if cfg.description != "" {
		bookOpts = append(bookOpts, mobi.WithDescription(cfg.description))
	}
	if cfg.isbn != "" {
		bookOpts = append(bookOpts, mobi.WithISBN(cfg.isbn))
	}
	if cfg.rights != "" {
		bookOpts = append(bookOpts, mobi.WithRights(cfg.rights))
	}
	if !cfg.publishedDate.IsZero() {
		bookOpts = append(bookOpts, mobi.WithPublishedDate(cfg.publishedDate))
	}
	if cfg.clippingLimit > 0 {
		bookOpts = append(bookOpts, mobi.WithClippingLimit(cfg.clippingLimit))
	}
	if startChapterIdx >= 0 {
		bookOpts = append(bookOpts, mobi.WithStartChapter(startChapterIdx))
	}
	if len(inlineImages) > 0 {
		bookOpts = append(bookOpts, mobi.WithImages(inlineImages...))
	}
	if cfg.coverImagePath != "" {
		cover, thumb, err := loadAndScaleCover(cfg.coverImagePath, cfg.imageBaseDir)
		if err != nil {
			return nil, fmt.Errorf("markdown: cover image: %w", err)
		}
		bookOpts = append(bookOpts, mobi.WithCoverImage(cover, thumb))
	}

	// Step 6: construct and validate the Book. NewBook validates all inputs
	// eagerly so that Realize() will not return a validation error.
	title := cmp.Or(strings.TrimSpace(cfg.title), "Untitled")

	book, err := mobi.NewBook(title, uid, bookOpts...)
	if err != nil {
		return nil, fmt.Errorf("markdown: build book: %w", err)
	}
	return book, nil
}
