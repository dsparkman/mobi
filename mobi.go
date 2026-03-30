// Package mobi writes KF8-format Kindle ebooks (AZW3/MOBI files).
//
// It is a fork of github.com/leotaku/mobi with a stable, idiomatic API:
// all errors are returned rather than panicked, construction is validated
// eagerly, and every spec-defined EXTH metadata field is exposed.
//
// # Quick start
//
//	ch := mobi.NewChapter("Chapter One").
//	    AddContent("<p>It was a dark and stormy night.</p>").
//	    AddSubChapter("Scene 1", "<p>The door creaked open.</p>").
//	    Build()
//
//	book, err := mobi.NewBook("My Novel", rand.Uint32(),
//	    mobi.WithAuthors("Jane Smith"),
//	    mobi.WithLanguage(language.English),
//	    mobi.WithCompression(mobi.CompressionPalmDoc),
//	    mobi.WithChapters(ch),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	db, err := book.Realize()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	f, err := os.Create("novel.azw3")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer f.Close()
//	if err := db.Write(f); err != nil {
//	    log.Fatal(err)
//	}
//
// # Content rules
//
// Chapter bodies must be XHTML fragments — no <html>, <head>, or <body>
// wrapper. Void elements must be self-closed (<br/>, <hr/>, <img .../>).
// CSS belongs in [WithCSSFlows], not inline <style> tags.
//
// # Sideloading covers
//
// Call [Book.GetThumbFilename] to obtain the filename a JPEG must have when
// placed in Kindle/system/thumbnails/ for the cover to display on sideloaded
// books. Newer Kindle firmware looks up covers by filename rather than reading
// the embedded thumbnail record.
//
// # Subpackage
//
// For converting Markdown files directly to AZW3, see the sibling
// [github.com/dsparkman/mobi/markdown] package.
package mobi

import (
	"cmp"
	"fmt"
	"image"
	"strings"
	"text/template"
	"time"

	"github.com/dsparkman/mobi/pdb"
	r "github.com/dsparkman/mobi/records"
	t "github.com/dsparkman/mobi/types"
	"golang.org/x/text/language"
)

// Compression selects the text compression algorithm written into PDB text
// records. The value is stored directly in the PalmDOC header compression
// field (offset 0, record 0).
type Compression uint16

const (
	// CompressionNone stores text as raw UTF-8 bytes. Produces the largest
	// files but requires no decompression step. Use when maximum tool
	// compatibility is more important than file size.
	CompressionNone Compression = 1

	// CompressionPalmDoc compresses text with the PalmDOC variant of LZ77,
	// operating on independent 4096-byte blocks. Typical prose compresses
	// 35–45% smaller than CompressionNone. This is the recommended setting
	// for books intended for sideloading.
	CompressionPalmDoc Compression = 2
)

// Book holds all information needed to produce a KF8-format MOBI or AZW3 file.
//
// Construct with [NewBook] rather than a struct literal — all fields are
// unexported. NewBook validates the configuration eagerly so that
// [Book.Realize] will not return a validation error if construction succeeded.
type Book struct {
	title    string
	uniqueID uint32
	language language.Tag

	authors      []string
	contributors []string
	publisher    string
	subject      string
	description  string
	isbn         string
	rights       string

	createdDate   time.Time
	publishedDate time.Time

	docType       string
	compression   Compression
	clippingLimit uint8
	ttsDisabled   bool
	startChapter  int
	fixedLayout   bool
	rightToLeft   bool
	vertical      bool

	chapters []Chapter
	cssFlows []string
	images   []image.Image
	cover    image.Image
	thumb    image.Image

	tpl *template.Template
}

// BookOption is a functional option for [NewBook].
// Each With* function in this package returns a BookOption.
type BookOption func(*Book)

// NewBook constructs a Book, applies all options, and validates the result.
//
// title must be non-empty. uniqueID should be a random uint32 for sideloaded
// books (use math/rand.Uint32); it is encoded as a synthetic ASIN used for
// cover thumbnail lookup on the device. At least one chapter must be supplied
// via [WithChapters] — NewBook returns an error if the chapter list is empty.
//
// If NewBook returns a nil error, [Book.Realize] will not return a validation
// error; only template-rendering or compression failures can occur at that stage.
func NewBook(title string, uniqueID uint32, opts ...BookOption) (*Book, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("mobi: title must not be empty")
	}

	b := &Book{
		title:        title,
		uniqueID:     uniqueID,
		language:     language.English,
		createdDate:  time.Now(),
		docType:      "EBOK",
		compression:  CompressionNone,
		startChapter: -1,
	}

	for _, o := range opts {
		o(b)
	}

	if err := b.validate(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Book) validate() error {
	if len(b.chapters) == 0 {
		return fmt.Errorf("mobi: book must contain at least one chapter")
	}
	for i, ch := range b.chapters {
		if len(ch.chunks) == 0 {
			return fmt.Errorf("mobi: chapter %d (%q) must contain at least one chunk", i, ch.title)
		}
	}
	if b.startChapter >= len(b.chapters) {
		return fmt.Errorf("mobi: startChapter index %d out of range (book has %d chapters)",
			b.startChapter, len(b.chapters))
	}
	if b.clippingLimit > 100 {
		return fmt.Errorf("mobi: clippingLimit must be 0–100, got %d", b.clippingLimit)
	}
	return nil
}

// WithAuthors sets the book's author list (EXTH record 100).
// Multiple calls overwrite the previous value; pass multiple arguments for
// co-authored books:
//
//	mobi.WithAuthors("Alice Liddell", "Bob Dodgson")
func WithAuthors(a ...string) BookOption { return func(b *Book) { b.authors = a } }

// WithContributors sets contributors such as editors or translators
// (EXTH record 108).
func WithContributors(c ...string) BookOption { return func(b *Book) { b.contributors = c } }

// WithPublisher sets the publisher name (EXTH record 101).
func WithPublisher(p string) BookOption { return func(b *Book) { b.publisher = p } }

// WithSubject sets the subject or genre (EXTH record 105).
func WithSubject(s string) BookOption { return func(b *Book) { b.subject = s } }

// WithDescription sets the book description shown in library software and
// the Kindle store (EXTH record 103).
func WithDescription(d string) BookOption { return func(b *Book) { b.description = d } }

// WithISBN sets the book's ISBN (EXTH record 104).
// Amazon recommends using a Kindle-specific ISBN distinct from any print or
// EPUB edition.
func WithISBN(isbn string) BookOption { return func(b *Book) { b.isbn = isbn } }

// WithRights sets the copyright or rights statement (EXTH record 109).
func WithRights(r string) BookOption { return func(b *Book) { b.rights = r } }

// WithLanguage sets the book language. The argument must be a [language.Tag]
// from golang.org/x/text/language, not a plain string:
//
//	mobi.WithLanguage(language.English)
//	mobi.WithLanguage(language.Japanese)
//	mobi.WithLanguage(language.MustParse("de-AT"))
//
// The tag is written to both the MOBI header locale field and EXTH record 524.
// Defaults to language.English.
func WithLanguage(lang language.Tag) BookOption { return func(b *Book) { b.language = lang } }

// WithCreatedDate sets the database creation timestamp written into the
// PalmDB header. Defaults to [time.Now] at construction time.
func WithCreatedDate(d time.Time) BookOption { return func(b *Book) { b.createdDate = d } }

// WithPublishedDate sets the original publication date (EXTH record 106),
// formatted as ISO 8601 in UTC when written to the file.
func WithPublishedDate(d time.Time) BookOption { return func(b *Book) { b.publishedDate = d } }

// WithDocType sets the Kindle document type (EXTH record 501).
// Known values: "EBOK" (ebook, default), "PDOC" (personal document),
// "EBSP" (ebook sample).
func WithDocType(dt string) BookOption { return func(b *Book) { b.docType = dt } }

// WithCompression sets the PDB text compression algorithm.
// Defaults to [CompressionNone]. Use [CompressionPalmDoc] for production
// books — it reduces file size by roughly 35–45% for typical prose with no
// loss of content.
func WithCompression(c Compression) BookOption { return func(b *Book) { b.compression = c } }

// WithClippingLimit sets the maximum percentage of book text that Kindle
// allows to be clipped and copied (EXTH record 401). Valid range is 0–100;
// 0 means unset (Kindle applies its own default). Returns an error from
// [NewBook] if the value exceeds 100.
//
// Publishers typically set this to 10.
func WithClippingLimit(v uint8) BookOption { return func(b *Book) { b.clippingLimit = v } }

// WithTTSDisabled controls Kindle's text-to-speech feature (EXTH record 404).
// Pass true to disable TTS; false (the default) leaves it enabled.
func WithTTSDisabled(v bool) BookOption { return func(b *Book) { b.ttsDisabled = v } }

// WithStartChapter sets the 0-based index of the chapter the Kindle opens to
// on first launch (EXTH record 116). Useful for skipping a cover page or front
// matter. Returns an error from [NewBook] if the index is out of range.
func WithStartChapter(idx int) BookOption { return func(b *Book) { b.startChapter = idx } }

// WithFixedLayout enables fixed-layout mode (EXTH record 122, value "true"),
// used for comics, magazines, and content where exact page geometry matters.
// Most prose books should leave this at its default of false.
func WithFixedLayout(v bool) BookOption { return func(b *Book) { b.fixedLayout = v } }

// WithRightToLeft marks the book as right-to-left (EXTH record 527, "rtl")
// for Arabic, Hebrew, and similar scripts. Combine with [WithVertical] for
// Japanese vertical text.
func WithRightToLeft(v bool) BookOption { return func(b *Book) { b.rightToLeft = v } }

// WithVertical enables vertical writing mode in combination with
// [WithRightToLeft], setting the primary writing mode to "vertical-rl"
// (EXTH record 525). Has no effect if WithRightToLeft is not also set.
func WithVertical(v bool) BookOption { return func(b *Book) { b.vertical = v } }

// WithCoverImage sets the full-resolution cover and its thumbnail.
// Both arguments are required — passing only one is a compile error.
//
// The cover is stored as a PDB image record; the thumbnail is stored
// separately and associated via EXTH records 201–203. Produce the thumbnail
// by scaling the cover to fit within 330×500 pixels while preserving aspect
// ratio; [golang.org/x/image/draw].BiLinear.Scale gives good quality.
//
// See also [Book.GetThumbFilename].
func WithCoverImage(img, thumb image.Image) BookOption {
	return func(b *Book) { b.cover = img; b.thumb = thumb }
}

// WithImages adds inline images to the book's PDB image records. Inline images
// are referenced from chapter HTML via kindle:embed:XXXX URIs, where XXXX is
// the 1-based image index formatted as a 4-character base-32 string.
func WithImages(imgs ...image.Image) BookOption { return func(b *Book) { b.images = imgs } }

// WithCSSFlows sets the CSS flow records injected into the book's FDST section.
// Each string becomes a separate flow record referenced from the KF8 skeleton
// HTML via a kindle:flow URI. A single stylesheet string covers most books:
//
//	mobi.WithCSSFlows("body { line-height: 1.5; } h1 { font-size: 1.6em; }")
//
// The [github.com/dsparkman/mobi/markdown] package supplies a complete
// Kindle-tuned stylesheet automatically.
func WithCSSFlows(css ...string) BookOption { return func(b *Book) { b.cssFlows = css } }

// WithChapters sets the book's chapter list. At least one chapter is required;
// [NewBook] returns an error if the list is empty.
//
// Build chapters with [SimpleChapter] for single-content chapters or
// [NewChapter] for the builder which supports sub-chapters.
func WithChapters(chapters ...Chapter) BookOption {
	return func(b *Book) { b.chapters = chapters }
}

// WithTemplate overrides the Go text/template used to generate the KF8 skeleton
// HTML that wraps each chunk. The template receives an internal inventory value
// containing the book, chapter title and ID, and chunk ID. Use this only if the
// default skeleton is incompatible with your content — a malformed template
// produces an unreadable file.
func WithTemplate(tpl template.Template) BookOption {
	return func(b *Book) { b.tpl = &tpl }
}

// Title returns the book's title.
// Exposed as a method so Go text/template can access it via {{ .Mobi.Title }}.
func (b *Book) Title() string { return b.title }

// CSSFlows returns the book's CSS flow strings.
// Exposed as a method so Go text/template can iterate via {{ range .Mobi.CSSFlows }}.
func (b *Book) CSSFlows() []string { return b.cssFlows }

// GetThumbFilename returns the filename a JPEG image must have when placed in
// the Kindle's thumbnails directory (Kindle/system/thumbnails/) for the cover
// to display correctly on sideloaded books.
//
// Newer Kindle firmware ignores the embedded PDB thumbnail record and instead
// performs a filesystem lookup using this exact filename. Copy a 330×500-pixel
// JPEG with this name to the thumbnails directory after transferring the AZW3
// file to the device.
//
//	fmt.Println(book.GetThumbFilename())
//	// thumbnail_000000001a2b3c4d_EBOK_portrait.jpg
func (b *Book) GetThumbFilename() string {
	return fmt.Sprintf("thumbnail_%v_EBOK_portrait.jpg", encodeASIN(b.uniqueID))
}

// Realize converts the Book into a [pdb.Database] ready for writing.
// Call [pdb.Database.Write] on the result to serialise it to any [io.Writer].
//
// Realize never panics. All errors, including template rendering failures and
// compression errors, are returned as Go error values. If [NewBook] returned
// a nil error, the only failures Realize can produce are template expansion
// errors (when a custom template is set via [WithTemplate]) and internal
// compression errors (extremely unlikely for valid UTF-8 content).
func (b *Book) Realize() (pdb.Database, error) {
	tpl := b.tpl
	if tpl == nil {
		tpl = defaultTemplate
	}

	html, chunks, chaps, err := chaptersToText(b, tpl)
	if err != nil {
		return pdb.Database{}, fmt.Errorf("mobi: template rendering failed: %w", err)
	}

	text := html + strings.Join(b.cssFlows, "")

	textRecords, err := textToRecords(text, chaps, b.compression)
	if err != nil {
		return pdb.Database{}, fmt.Errorf("mobi: text compression failed: %w", err)
	}

	db := pdb.NewDatabase(b.title, b.createdDate)

	null, err := b.createNullRecord()
	if err != nil {
		return pdb.Database{}, err
	}
	db.AddRecord(null)

	null.PalmDocHeader.TextRecordCount = uint16(len(textRecords))
	null.PalmDocHeader.TextLength = uint32(len(text))
	null.PalmDocHeader.Compression = uint16(b.compression)
	for _, rec := range textRecords {
		db.AddRecord(rec)
	}

	if len(textRecords) > 0 {
		lastLen := textRecords[len(textRecords)-1].Length()
		if lastLen%4 != 0 {
			db.AddRecord(make(pdb.RawRecord, 4-(lastLen%4)))
		}
	}
	null.MOBIHeader.FirstNonBookIndex = uint32(db.Idx() + 1)

	chunk, cncx := r.ChunkIndexRecord(chunks)
	ch := r.ChunkHeaderIndexRecord(len(text), len(chunks))
	null.MOBIHeader.ChunkIndex = uint32(db.AddRecord(ch))
	db.AddRecord(chunk)
	db.AddRecord(cncx)

	skeleton := r.SkeletonIndexRecord(chunks)
	sh := r.SkeletonHeaderIndexRecord(len(skeleton.IDXTEntries))
	null.MOBIHeader.SkeletonIndex = uint32(db.AddRecord(sh))
	db.AddRecord(skeleton)

	ncx, ncxCncx := r.NCXIndexRecord(chaps)
	nh := r.NCXHeaderIndexRecord(len(chaps))
	null.MOBIHeader.INDXRecordOffset = uint32(db.AddRecord(nh))
	db.AddRecord(ncx)
	db.AddRecord(ncxCncx)

	allImages := make([]image.Image, len(b.images))
	copy(allImages, b.images)
	if b.cover != nil {
		allImages = append(allImages, b.cover)
	}
	if b.thumb != nil {
		allImages = append(allImages, b.thumb)
	}
	if len(allImages) > 0 {
		null.MOBIHeader.FirstImageIndex = uint32(db.Idx() + 1)
		null.EXTHSection.AddInt(t.EXTHKF8CountResources, len(allImages))
	}
	for _, img := range allImages {
		db.AddRecord(r.NewImageRecord(img))
	}

	flows := append([]string{html}, b.cssFlows...)
	db.AddRecord(r.NewFDSTRecord(flows...))
	null.MOBIHeader.Unknown3OrFDSTEntryCount = uint32(len(b.cssFlows) + 1)
	null.MOBIHeader.FirstContentRecordNumberOrFDSTNumberMSB = 0
	null.MOBIHeader.LastContentRecordNumberOrFDSTNumberLSB = uint16(db.Idx())

	db.AddRecord(t.NewFLISRecord())
	null.MOBIHeader.FLISRecordCount = 1
	null.MOBIHeader.FLISRecordNumber = uint32(db.Idx())

	db.AddRecord(t.NewFCISRecord(uint32(len(text))))
	null.MOBIHeader.FCISRecordCount = 1
	null.MOBIHeader.FCISRecordNumber = uint32(db.Idx())

	db.AddRecord(t.EOFRecord)
	db.ReplaceRecord(0, null)

	return db, nil
}

func (b *Book) createNullRecord() (r.NullRecord, error) {
	null := r.NewNullRecord(b.title)
	lastImageID := len(b.images)

	null.MOBIHeader.UniqueID = b.uniqueID
	null.MOBIHeader.Locale = matchLocale(b.language)

	ex := &null.EXTHSection
	ex.AddString(t.EXTHTitle, b.title)
	ex.AddString(t.EXTHUpdatedTitle, b.title)
	ex.AddString(t.EXTHAuthor, b.authors...)
	ex.AddString(t.EXTHContributor, b.contributors...)
	ex.AddString(t.EXTHPublisher, b.publisher)
	ex.AddString(t.EXTHSubject, b.subject)
	ex.AddString(t.EXTHASIN, encodeASIN(b.uniqueID))
	ex.AddString(t.EXTHLanguage, b.language.String())

	if b.description != "" {
		ex.AddString(t.EXTHDescription, b.description)
	}
	if b.isbn != "" {
		ex.AddString(t.EXTHISBN, b.isbn)
	}
	if b.rights != "" {
		ex.AddString(t.EXTHRights, b.rights)
	}
	if !b.publishedDate.IsZero() {
		ex.AddString(t.EXTHPublishingDate,
			b.publishedDate.UTC().Format("2006-01-02T15:04:05.000000+00:00"))
	}

	ex.AddString(t.EXTHDocType, cmp.Or(b.docType, "EBOK"))

	if b.clippingLimit > 0 {
		ex.AddInt(t.EXTHClippingLimit, int(b.clippingLimit))
	}
	if b.ttsDisabled {
		ex.AddInt(t.EXTHTtsDisable, 1)
	} else {
		ex.AddInt(t.EXTHTtsDisable, 0)
	}
	if b.startChapter >= 0 {
		// Placeholder — the actual byte offset is patched after assembly.
		ex.AddInt(t.EXTHStartReading, 0)
	}

	if b.fixedLayout {
		ex.AddString(t.EXTHFixedLayout, "true")
	}
	if b.rightToLeft {
		if b.vertical {
			ex.AddString(t.EXTHPrimaryWritingMode, "vertical-rl")
		} else {
			ex.AddString(t.EXTHPrimaryWritingMode, "horizontal-rl")
		}
		ex.AddString(t.EXTHPageProgressionDirection, "rtl")
	}

	if b.cover != nil {
		ex.AddInt(t.EXTHCoverOffset, lastImageID)
		ex.AddInt(t.EXTHHasFakeCover, 0)
		ex.AddString(t.EXTHKF8CoverURI, fmt.Sprintf("kindle:embed:%v", r.To32(lastImageID+1)))
		lastImageID++
	}
	if b.thumb != nil {
		ex.AddInt(t.EXTHThumbOffset, lastImageID)
	}

	return null, nil
}

func encodeASIN(id uint32) string {
	return fmt.Sprintf("%016x", id)
}
