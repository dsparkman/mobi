package mobi_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/dsparkman/mobi"
	"github.com/dsparkman/mobi/pdb"
	"golang.org/x/text/language"
)

// ── Construction validation ──────────────────────────────────────────────────

func TestNewBook_EmptyTitle(t *testing.T) {
	_, err := mobi.NewBook("", rand.Uint32(),
		mobi.WithChapters(mobi.SimpleChapter("Ch1", "<p>text</p>")))
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestNewBook_NoChapters(t *testing.T) {
	_, err := mobi.NewBook("Title", rand.Uint32())
	if err == nil {
		t.Fatal("expected error for book with no chapters")
	}
}

func TestNewBook_EmptyChunk(t *testing.T) {
	ch := mobi.NewChapter("Empty").Build()
	_, err := mobi.NewBook("Title", rand.Uint32(), mobi.WithChapters(ch))
	if err == nil {
		t.Fatal("expected error for chapter with no chunks")
	}
}

func TestNewBook_InvalidStartChapter(t *testing.T) {
	_, err := mobi.NewBook("Title", rand.Uint32(),
		mobi.WithChapters(mobi.SimpleChapter("Ch1", "<p>x</p>")),
		mobi.WithStartChapter(99),
	)
	if err == nil {
		t.Fatal("expected error for out-of-range startChapter")
	}
}

func TestNewBook_InvalidClippingLimit(t *testing.T) {
	_, err := mobi.NewBook("Title", rand.Uint32(),
		mobi.WithChapters(mobi.SimpleChapter("Ch1", "<p>x</p>")),
		mobi.WithClippingLimit(101),
	)
	if err == nil {
		t.Fatal("expected error for clippingLimit > 100")
	}
}

// ── Realize returns error, not panic ─────────────────────────────────────────

func TestRealize_NoError_SimpleBook(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Chapter 1", "<p>Hello Kindle.</p>")),
		mobi.WithAuthors("Test Author"),
	)
	db, err := book.Realize()
	if err != nil {
		t.Fatalf("Realize() unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		t.Fatalf("db.Write() error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("written database is empty")
	}
}

// ── PDB structural checks ────────────────────────────────────────────────────

func TestRealize_PDBMagicBytes(t *testing.T) {
	book := mustBook(t, mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")))
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	raw := buf.Bytes()

	// PalmDBHeader layout: 32 (Name) + 2 + 2 + 4*7 (timestamps + info fields) = 60
	// Type "BOOK" is at offset 60, Creator "MOBI" at offset 64.
	if string(raw[60:64]) != "BOOK" {
		t.Errorf("expected BOOK type at offset 60, got %q", raw[60:64])
	}
	if string(raw[64:68]) != "MOBI" {
		t.Errorf("expected MOBI creator at offset 64, got %q", raw[64:68])
	}

	rec0Offset := int(binary.BigEndian.Uint32(raw[78:82]))
	mobiMagic := raw[rec0Offset+16 : rec0Offset+20]
	if string(mobiMagic) != "MOBI" {
		t.Errorf("expected MOBI magic in record 0, got %q", mobiMagic)
	}
}

func TestRealize_EXTHPresent(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithDescription("A great book"),
		mobi.WithISBN("978-3-16-148410-0"),
	)
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	raw := buf.Bytes()

	if !bytes.Contains(raw, []byte("EXTH")) {
		t.Error("EXTH section not found in output")
	}
	if !bytes.Contains(raw, []byte("A great book")) {
		t.Error("description not found in EXTH")
	}
	if !bytes.Contains(raw, []byte("978-3-16-148410-0")) {
		t.Error("ISBN not found in EXTH")
	}
}

func TestRealize_FLISFCISEOFPresent(t *testing.T) {
	book := mustBook(t, mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")))
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	raw := buf.Bytes()

	if !bytes.Contains(raw, []byte("FLIS")) {
		t.Error("FLIS magic record not found")
	}
	if !bytes.Contains(raw, []byte("FCIS")) {
		t.Error("FCIS magic record not found")
	}
	eof := []byte{0xE9, 0x8E, 0x0D, 0x0A}
	if !bytes.Contains(raw, eof) {
		t.Error("EOF magic record not found")
	}
}

// ── Compression ──────────────────────────────────────────────────────────────

func TestRealize_CompressionNone(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", strings.Repeat("<p>Hello</p>", 20))),
		mobi.WithCompression(mobi.CompressionNone),
	)
	db, _ := book.Realize()
	var buf bytes.Buffer
	_ = db.Write(&buf)

	rec0Off := int(binary.BigEndian.Uint32(buf.Bytes()[78:82]))
	comp := binary.BigEndian.Uint16(buf.Bytes()[rec0Off : rec0Off+2])
	if comp != 1 {
		t.Errorf("expected compression type 1 (none), got %d", comp)
	}
}

func TestRealize_CompressionPalmDoc(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", strings.Repeat("<p>Hello world</p>", 50))),
		mobi.WithCompression(mobi.CompressionPalmDoc),
	)
	dbComp, err := book.Realize()
	assertNoErr(t, err)

	bookNone := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", strings.Repeat("<p>Hello world</p>", 50))),
		mobi.WithCompression(mobi.CompressionNone),
	)
	dbNone, _ := bookNone.Realize()

	var bufComp, bufNone bytes.Buffer
	_ = dbComp.Write(&bufComp)
	_ = dbNone.Write(&bufNone)

	if bufComp.Len() >= bufNone.Len() {
		t.Errorf("compressed file (%d bytes) should be smaller than uncompressed (%d bytes)",
			bufComp.Len(), bufNone.Len())
	}

	raw := bufComp.Bytes()
	rec0Off := int(binary.BigEndian.Uint32(raw[78:82]))
	comp := binary.BigEndian.Uint16(raw[rec0Off : rec0Off+2])
	if comp != 2 {
		t.Errorf("expected compression type 2 (palmdoc), got %d", comp)
	}
}

// ── SubChapters ──────────────────────────────────────────────────────────────

func TestRealize_SubChapters(t *testing.T) {
	ch := mobi.NewChapter("Part I").
		AddContent("<p>Introduction.</p>").
		AddSubChapter("Section 1.1", "<p>First section.</p>").
		AddSubChapter("Section 1.2", "<p>Second section.</p>").
		Build()

	book := mustBook(t, mobi.WithChapters(ch))
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	raw := buf.Bytes()

	if !bytes.Contains(raw, []byte("Section 1.1")) {
		t.Error("SubChapter title 'Section 1.1' not found in output")
	}
	if !bytes.Contains(raw, []byte("Section 1.2")) {
		t.Error("SubChapter title 'Section 1.2' not found in output")
	}
}

// ── Metadata round-trip ──────────────────────────────────────────────────────

func TestRealize_MetadataInOutput(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithAuthors("Alice Liddell", "Bob Dodgson"),
		mobi.WithPublisher("Wonderland Press"),
		mobi.WithDescription("Down the rabbit hole"),
		mobi.WithISBN("0-19-853453-1"),
		mobi.WithRights("© 1865"),
		mobi.WithLanguage(language.English),
		mobi.WithPublishedDate(time.Date(1865, 11, 26, 0, 0, 0, 0, time.UTC)),
		mobi.WithClippingLimit(10),
	)
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	raw := buf.Bytes()

	for _, s := range []string{
		"Alice Liddell", "Bob Dodgson", "Wonderland Press",
		"Down the rabbit hole", "0-19-853453-1", "1865",
	} {
		if !bytes.Contains(raw, []byte(s)) {
			t.Errorf("expected %q in output but not found", s)
		}
	}
}

// ── ThumbFilename ─────────────────────────────────────────────────────────────

func TestGetThumbFilename_Format(t *testing.T) {
	book := mustBook(t, mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")))
	name := book.GetThumbFilename()
	if !strings.HasPrefix(name, "thumbnail_") {
		t.Errorf("unexpected thumb filename: %q", name)
	}
	if !strings.HasSuffix(name, "_EBOK_portrait.jpg") {
		t.Errorf("unexpected thumb filename suffix: %q", name)
	}
}

// ── PDB header field sizes ───────────────────────────────────────────────────

func TestPDBHeaderLength(t *testing.T) {
	length := measureStruct(t, pdb.PalmDBHeader{})
	if length != pdb.PalmDBHeaderLength {
		t.Errorf("PalmDBHeader binary size: want %d, got %d", pdb.PalmDBHeaderLength, length)
	}
}

func TestRecordHeaderLength(t *testing.T) {
	length := measureStruct(t, pdb.RecordHeader{})
	if length != pdb.RecordHeaderLength {
		t.Errorf("RecordHeader binary size: want %d, got %d", pdb.RecordHeaderLength, length)
	}
}

// ── PDB read/write symmetry ───────────────────────────────────────────────────

func TestPDBReadWrite(t *testing.T) {
	w := bytes.NewBuffer(nil)
	db := pdb.NewDatabase("Test Book", time.Unix(0, 0))
	db.AddRecord(pdb.RawRecord("dog"))
	db.AddRecord(pdb.RawRecord("cat"))
	db.AddRecord(pdb.RawRecord("fish"))
	assertNoErr(t, db.Write(w))

	r := bytes.NewReader(w.Bytes())
	rdb, err := pdb.ReadDatabase(r)
	assertNoErr(t, err)

	if db.Name != rdb.Name {
		t.Errorf("name mismatch: %q vs %q", db.Name, rdb.Name)
	}
	if len(db.Records) != len(rdb.Records) {
		t.Errorf("record count mismatch: %d vs %d", len(db.Records), len(rdb.Records))
	}
}

// ── Book accessor methods ─────────────────────────────────────────────────────

func TestBook_Title(t *testing.T) {
	book := mustBook(t, mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")))
	if book.Title() != "Test Book" {
		t.Errorf("Title(): want %q, got %q", "Test Book", book.Title())
	}
}

func TestBook_CSSFlows(t *testing.T) {
	css := "body { color: red; }"
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithCSSFlows(css),
	)
	flows := book.CSSFlows()
	if len(flows) != 1 || flows[0] != css {
		t.Errorf("CSSFlows(): want [%q], got %v", css, flows)
	}
}

// ── Chapter API ───────────────────────────────────────────────────────────────

func TestChapter_Title(t *testing.T) {
	ch := mobi.SimpleChapter("My Title", "<p>body</p>")
	if ch.Title() != "My Title" {
		t.Errorf("Chapter.Title(): want %q, got %q", "My Title", ch.Title())
	}
}

func TestChapter_SubChapters_Empty(t *testing.T) {
	ch := mobi.SimpleChapter("Ch", "<p>body</p>")
	if subs := ch.SubChapters(); len(subs) != 0 {
		t.Errorf("SimpleChapter should have no sub-chapters, got %d", len(subs))
	}
}

func TestChapter_SubChapters_Present(t *testing.T) {
	ch := mobi.NewChapter("Part I").
		AddContent("<p>Intro.</p>").
		AddSubChapter("Sec 1", "<p>First.</p>").
		AddSubChapter("Sec 2", "<p>Second.</p>").
		Build()
	subs := ch.SubChapters()
	if len(subs) != 2 {
		t.Fatalf("expected 2 sub-chapters, got %d", len(subs))
	}
}

func TestChunks_Helper(t *testing.T) {
	chunks := mobi.Chunks("<p>one</p>", "<p>two</p>", "<p>three</p>")
	if len(chunks) != 3 {
		t.Fatalf("Chunks(): want 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Body != "<p>one</p>" {
		t.Errorf("chunks[0].Body = %q, want %q", chunks[0].Body, "<p>one</p>")
	}
}

// ── Additional Book options ───────────────────────────────────────────────────

func TestRealize_WithStartChapter_Valid(t *testing.T) {
	book := mustBook(t,
		mobi.WithChapters(
			mobi.SimpleChapter("Ch1", "<p>one</p>"),
			mobi.SimpleChapter("Ch2", "<p>two</p>"),
		),
		mobi.WithStartChapter(1),
	)
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	if buf.Len() == 0 {
		t.Fatal("output is empty")
	}
}

func TestRealize_Contributors(t *testing.T) {
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithContributors("Ed Editor", "Tr Translator"),
	)
	for _, c := range []string{"Ed Editor", "Tr Translator"} {
		if !bytes.Contains(raw, []byte(c)) {
			t.Errorf("contributor %q not found in output", c)
		}
	}
}

func TestRealize_CSSFlows(t *testing.T) {
	css := "/* custom kindle css */"
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithCSSFlows(css),
	)
	if !bytes.Contains(raw, []byte(css)) {
		t.Error("CSS flow content not found in output")
	}
}

func TestRealize_DocType(t *testing.T) {
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithDocType("PDOC"),
	)
	if !bytes.Contains(raw, []byte("PDOC")) {
		t.Error("custom DocType 'PDOC' not found in output")
	}
}

func TestRealize_TTSDisabled(t *testing.T) {
	// Just verify the book realizes without error when TTS is disabled.
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithTTSDisabled(true),
	)
	_, err := book.Realize()
	assertNoErr(t, err)
}

func TestRealize_FixedLayout(t *testing.T) {
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithFixedLayout(true),
	)
	if !bytes.Contains(raw, []byte("true")) {
		t.Error("fixed-layout 'true' value not found in output")
	}
}

func TestRealize_RightToLeft(t *testing.T) {
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithRightToLeft(true),
	)
	if !bytes.Contains(raw, []byte("rtl")) {
		t.Error("rtl direction not found in output")
	}
}

func TestRealize_RightToLeftVertical(t *testing.T) {
	raw := realizeAndGet(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithRightToLeft(true),
		mobi.WithVertical(true),
	)
	if !bytes.Contains(raw, []byte("vertical-rl")) {
		t.Error("vertical-rl writing mode not found in output")
	}
}

func TestRealize_WithImages(t *testing.T) {
	// Create a minimal 1×1 RGBA image.
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	book := mustBook(t,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
		mobi.WithImages(img),
	)
	db, err := book.Realize()
	assertNoErr(t, err)

	var buf bytes.Buffer
	assertNoErr(t, db.Write(&buf))
	if buf.Len() == 0 {
		t.Fatal("output is empty with images")
	}
}

func TestGetThumbFilename_ASINFormat(t *testing.T) {
	// uniqueID=1 → ASIN must be 16 hex chars padded with zeros.
	book, err := mobi.NewBook("T", 1,
		mobi.WithChapters(mobi.SimpleChapter("Ch", "<p>x</p>")),
	)
	if err != nil {
		t.Fatal(err)
	}
	name := book.GetThumbFilename()
	// Expected: thumbnail_0000000000000001_EBOK_portrait.jpg
	want := "thumbnail_0000000000000001_EBOK_portrait.jpg"
	if name != want {
		t.Errorf("GetThumbFilename(): want %q, got %q", want, name)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func realizeAndGet(t *testing.T, opts ...mobi.BookOption) []byte {
	t.Helper()
	book := mustBook(t, opts...)
	db, err := book.Realize()
	if err != nil {
		t.Fatalf("Realize(): %v", err)
	}
	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		t.Fatalf("Write(): %v", err)
	}
	return buf.Bytes()
}

// ── Original helpers ──────────────────────────────────────────────────────────

func mustBook(t *testing.T, opts ...mobi.BookOption) *mobi.Book {
	t.Helper()
	allOpts := append([]mobi.BookOption{mobi.WithLanguage(language.English)}, opts...)
	book, err := mobi.NewBook("Test Book", rand.Uint32(), allOpts...)
	if err != nil {
		t.Fatalf("NewBook() unexpected error: %v", err)
	}
	return book
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// measureStruct uses binary.Write to determine the on-wire size of a struct.
func measureStruct(t *testing.T, v any) int {
	t.Helper()
	var buf bytes.Buffer
	if err := binary.Write(&buf, pdb.Endian, v); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}
	return buf.Len()
}
