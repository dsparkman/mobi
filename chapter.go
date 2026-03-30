package mobi

// Chapter represents a top-level chapter in a [Book].
//
// A Chapter contains one or more [Chunk] values of KF8 XHTML content and a
// title used as the entry in the Kindle table of contents (NCX). It may also
// contain [SubChapter] values, which appear as a second level in the Kindle
// TOC navigation.
//
// Do not construct Chapter directly; use [SimpleChapter] for a single-content
// chapter or [NewChapter] for the builder pattern which supports sub-chapters.
type Chapter struct {
	title       string
	chunks      []Chunk
	subChapters []SubChapter
}

// SubChapter is a second-level navigation entry within a [Chapter].
//
// Sub-chapters do not introduce new KF8 skeleton records — they contribute
// NCX index entries that point to byte offsets within the parent chapter's
// assembled text stream, allowing the Kindle TOC to jump directly to them.
//
// Sub-chapters are created by [ChapterBuilder.AddSubChapter]; they cannot be
// constructed directly.
type SubChapter struct {
	title      string
	byteOffset int // byte offset from the start of the parent chapter's text
	chunks     []Chunk
}

// Chunk is a single unit of KF8 XHTML content within a chapter.
//
// Each Chunk corresponds to one skeleton/content pair in the assembled text
// stream. For almost all books, one Chunk per chapter (via [SimpleChapter] or
// [ChapterBuilder.AddContent]) is correct. Multiple chunks are an advanced
// feature for splitting very large chapters across the 4096-byte PDB record
// boundary without splitting mid-element.
//
// Body must be a valid KF8 XHTML fragment: no <html>, <head>, or <body>
// wrapper; all void elements self-closed (<br/>, <hr/>, <img .../>).
type Chunk struct {
	// Body is the raw XHTML fragment that forms this chunk's content.
	Body string
}

// ChapterBuilder constructs a [Chapter] incrementally, accumulating content
// and sub-chapters in order. Obtain one with [NewChapter]; call [ChapterBuilder.Build]
// to finalise.
type ChapterBuilder struct {
	title       string
	chunks      []Chunk
	subChapters []SubChapter
	bytesSoFar  int
}

// NewChapter creates a [ChapterBuilder] with the given title.
// Chain [ChapterBuilder.AddContent] and [ChapterBuilder.AddSubChapter] calls,
// then call [ChapterBuilder.Build] to obtain the finished [Chapter].
//
//	ch := mobi.NewChapter("Part I: Foundations").
//	    AddContent("<p>Overview paragraph.</p>").
//	    AddSubChapter("1.1 Background", "<p>Historical context.</p>").
//	    AddSubChapter("1.2 Setup",      "<p>Installation steps.</p>").
//	    Build()
func NewChapter(title string) *ChapterBuilder {
	return &ChapterBuilder{title: title}
}

// AddContent appends an XHTML fragment as a new [Chunk] in this chapter.
// The body must follow the same rules as [Chunk.Body].
// Returns the receiver to allow method chaining.
func (cb *ChapterBuilder) AddContent(body string) *ChapterBuilder {
	cb.chunks = append(cb.chunks, Chunk{Body: body})
	cb.bytesSoFar += len(body)
	return cb
}

// AddSubChapter appends a sub-chapter with its own title and XHTML content.
// The byte offset is recorded automatically at the time of the call, relative
// to the start of the parent chapter's accumulated content. This offset is
// used to generate a precise NCX entry so the Kindle TOC can navigate directly
// to the sub-chapter.
//
// Returns the receiver to allow method chaining.
func (cb *ChapterBuilder) AddSubChapter(title string, body string) *ChapterBuilder {
	cb.subChapters = append(cb.subChapters, SubChapter{
		title:      title,
		byteOffset: cb.bytesSoFar,
		chunks:     []Chunk{{Body: body}},
	})
	cb.bytesSoFar += len(body)
	return cb
}

// Build finalises the builder and returns the constructed [Chapter].
// The builder may be discarded after this call; the returned Chapter is
// an independent value.
func (cb *ChapterBuilder) Build() Chapter {
	chunks := make([]Chunk, len(cb.chunks))
	copy(chunks, cb.chunks)
	subs := make([]SubChapter, len(cb.subChapters))
	copy(subs, cb.subChapters)
	return Chapter{
		title:       cb.title,
		chunks:      chunks,
		subChapters: subs,
	}
}

// SimpleChapter creates a [Chapter] with a single [Chunk] from a raw XHTML body.
// This is the most convenient constructor for books where each chapter is a
// single continuous block of content.
//
//	ch := mobi.SimpleChapter("Chapter 1", "<p>Once upon a time…</p>")
func SimpleChapter(title, htmlBody string) Chapter {
	return Chapter{
		title:  title,
		chunks: []Chunk{{Body: htmlBody}},
	}
}

// Chunks converts one or more XHTML body strings into a []Chunk slice.
// Provided for compatibility with code migrating from github.com/leotaku/mobi.
// Prefer [SimpleChapter] or [NewChapter] for new code.
func Chunks(bodies ...string) []Chunk {
	out := make([]Chunk, len(bodies))
	for i, b := range bodies {
		out[i] = Chunk{Body: b}
	}
	return out
}

// Title returns the chapter's table-of-contents title.
func (c Chapter) Title() string { return c.title }

// SubChapters returns the chapter's sub-chapter list, which may be empty.
// Sub-chapters appear as a second level of navigation in the Kindle TOC.
func (c Chapter) SubChapters() []SubChapter { return c.subChapters }
