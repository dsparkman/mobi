// Package markdown converts Markdown documents into [github.com/dsparkman/mobi]
// Book values that can be written as Kindle AZW3 files.
//
// # Quick start
//
//	book, err := markdown.NewConverter(
//	    markdown.WithTitle("My Novel"),
//	    markdown.WithAuthor("Jane Smith"),
//	).ConvertFile("novel.md")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	db, err := book.Realize()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	f, _ := os.Create("novel.azw3")
//	defer f.Close()
//	db.Write(f)
//
// # Metadata from front-matter
//
// If the Markdown file begins with a YAML front-matter block (--- ... ---),
// the converter extracts title, author, language, date, description, ISBN,
// rights, subject, and cover image path from it automatically. Explicit
// [Option] values always override front-matter.
//
// # Chapter splitting
//
// By default, each H1 heading (# Title) becomes a top-level Kindle chapter
// and each H2 within it becomes a sub-chapter in the TOC. Control this with
// [WithSplitLevel] and [WithSubSplitLevel].
//
// # Language argument type
//
// Unlike [github.com/dsparkman/mobi.WithLanguage], which takes a
// [golang.org/x/text/language.Tag], this package's [WithLanguage] accepts a
// plain BCP-47 string such as "en", "ja", or "de-AT".
package markdown

import (
	"time"

	"github.com/dsparkman/mobi"
)

// config is the internal, fully-resolved configuration for a single conversion.
// All fields have sensible defaults supplied by defaultConfig.
type config struct {
	title         string
	authors       []string
	publisher     string
	subject       string
	description   string
	isbn          string
	rights        string
	language      string // BCP-47 tag, e.g. "en", "de-AT", "ja"
	publishedDate time.Time
	uniqueID      uint32 // 0 = generate randomly at conversion time

	compression   mobi.Compression
	clippingLimit uint8

	splitLevel    SplitLevel
	subSplitLevel SplitLevel

	customCSS string

	coverImagePath string
	imageBaseDir   string // base directory for resolving relative paths
	embedImages    bool
}

// SplitLevel controls which Markdown heading level triggers a structural break
// when building the chapter hierarchy.
type SplitLevel int

const (
	// SplitNone treats the entire document as a single chapter with no
	// internal TOC structure.
	SplitNone SplitLevel = 0

	// SplitH1 maps each level-1 heading (# Title) to a top-level Kindle
	// chapter. This is the default split level.
	SplitH1 SplitLevel = 1

	// SplitH2 maps each level-2 heading (## Title) to a top-level Kindle
	// chapter, treating H1 headings as part-level groupings.
	SplitH2 SplitLevel = 2
)

func defaultConfig() config {
	return config{
		language:      "en",
		compression:   mobi.CompressionPalmDoc,
		splitLevel:    SplitH1,
		subSplitLevel: SplitH2,
		embedImages:   true,
	}
}

// Option is a functional option for [NewConverter].
type Option func(*config)

// WithTitle sets the book title.
//
// If not set, the converter attempts to extract the title from a YAML
// front-matter "title" field or from the text of the first H1 heading.
// Passing this option overrides both sources.
func WithTitle(t string) Option { return func(c *config) { c.title = t } }

// WithAuthor sets a single author name. For multiple authors use [WithAuthors].
func WithAuthor(a string) Option { return func(c *config) { c.authors = []string{a} } }

// WithAuthors sets one or more author names:
//
//	markdown.WithAuthors("Alice Smith", "Bob Jones")
func WithAuthors(a ...string) Option { return func(c *config) { c.authors = a } }

// WithPublisher sets the publisher name (EXTH record 101).
func WithPublisher(p string) Option { return func(c *config) { c.publisher = p } }

// WithSubject sets the subject or genre (EXTH record 105).
func WithSubject(s string) Option { return func(c *config) { c.subject = s } }

// WithDescription sets the back-cover description (EXTH record 103).
func WithDescription(d string) Option { return func(c *config) { c.description = d } }

// WithISBN sets the book's ISBN (EXTH record 104).
func WithISBN(isbn string) Option { return func(c *config) { c.isbn = isbn } }

// WithRights sets the copyright or rights statement (EXTH record 109).
func WithRights(r string) Option { return func(c *config) { c.rights = r } }

// WithLanguage sets the book language as a BCP-47 tag string, for example
// "en", "ja", or "de-AT". Defaults to "en".
//
// Note: this function takes a plain string, unlike
// [github.com/dsparkman/mobi.WithLanguage] which takes a
// [golang.org/x/text/language.Tag].
func WithLanguage(l string) Option { return func(c *config) { c.language = l } }

// WithPublishedDate sets the original publication date (EXTH record 106).
func WithPublishedDate(d time.Time) Option { return func(c *config) { c.publishedDate = d } }

// WithUniqueID sets the MOBI unique ID embedded in the PalmDB header and used
// as a synthetic ASIN. If 0 or unset, a random uint32 is generated at
// conversion time.
func WithUniqueID(id uint32) Option { return func(c *config) { c.uniqueID = id } }

// WithCompression sets the PDB text compression algorithm.
// Defaults to [github.com/dsparkman/mobi.CompressionPalmDoc], which reduces
// typical prose by 35–45%. Pass [github.com/dsparkman/mobi.CompressionNone]
// for maximum compatibility with older tools.
func WithCompression(comp mobi.Compression) Option {
	return func(c *config) { c.compression = comp }
}

// WithClippingLimit sets the maximum percentage of book text that Kindle
// permits to be clipped and copied (EXTH record 401). Valid range is 0–100;
// 0 means unset. Publishers typically use 10.
func WithClippingLimit(pct uint8) Option { return func(c *config) { c.clippingLimit = pct } }

// WithSplitLevel sets the Markdown heading level that triggers top-level
// chapter breaks. See [SplitLevel] constants for available values.
// Defaults to [SplitH1].
func WithSplitLevel(l SplitLevel) Option { return func(c *config) { c.splitLevel = l } }

// WithSubSplitLevel sets the Markdown heading level that triggers sub-chapter
// breaks within a top-level chapter. Sub-chapters appear as a second level
// in the Kindle TOC. Set to [SplitNone] to disable sub-chapters entirely.
// Defaults to [SplitH2] when the chapter split level is [SplitH1].
func WithSubSplitLevel(l SplitLevel) Option { return func(c *config) { c.subSplitLevel = l } }

// WithCustomCSS appends additional CSS rules to the built-in Kindle-tuned
// stylesheet. The custom rules are injected after the defaults, so they take
// precedence:
//
//	markdown.WithCustomCSS("p { text-indent: 1em; } p.first { text-indent: 0; }")
func WithCustomCSS(css string) Option { return func(c *config) { c.customCSS = css } }

// WithCoverImage sets the path to a cover image. Supported formats are JPEG,
// PNG, and GIF. The image is loaded from disk, and a thumbnail scaled to
// 330×500 pixels is generated automatically.
//
// Relative paths are resolved against the base directory set by
// [WithImageBaseDir], or — when using [Converter.ConvertFile] — against the
// directory containing the Markdown file.
//
// Note: this function takes a file path string, unlike
// [github.com/dsparkman/mobi.WithCoverImage] which takes two [image.Image]
// values.
func WithCoverImage(path string) Option { return func(c *config) { c.coverImagePath = path } }

// WithImageBaseDir sets the base directory used to resolve relative paths for
// both the cover image and inline images embedded in the Markdown. When using
// [Converter.ConvertFile], the file's own directory is used automatically and
// this option is unnecessary unless a different base is required.
func WithImageBaseDir(dir string) Option { return func(c *config) { c.imageBaseDir = dir } }

// WithEmbedImages controls whether local images referenced in the Markdown
// (e.g. ![alt text](./photo.jpg)) are loaded, added to the book's image
// records, and their src attributes rewritten to kindle:embed URIs.
// Defaults to true. Set to false to skip image embedding entirely, leaving
// the original src values intact (they will not render on the device).
func WithEmbedImages(v bool) Option { return func(c *config) { c.embedImages = v } }
