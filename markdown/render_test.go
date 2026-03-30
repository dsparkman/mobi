package markdown

import (
	"strings"
	"testing"
)

// ── sanitizeForKindle ─────────────────────────────────────────────────────────

func TestSanitize_BRSelfClose(t *testing.T) {
	cfg := defaultConfig()
	cases := []struct{ in, want string }{
		{"<br>", "<br/>"},
		{"<BR>", "<br/>"},
		{"<br />", "<br/>"},
		{"<br/>", "<br/>"},
	}
	for _, tc := range cases {
		got := sanitizeForKindle(tc.in, cfg)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitize_HRSelfClose(t *testing.T) {
	cfg := defaultConfig()
	got := sanitizeForKindle("<hr>", cfg)
	if got != "<hr/>" {
		t.Errorf("sanitize(<hr>) = %q, want %q", got, "<hr/>")
	}
}

func TestSanitize_ImgSelfClose(t *testing.T) {
	cfg := defaultConfig()
	// gomarkdown may emit <img src="x.jpg"> without self-close
	got := sanitizeForKindle(`<img src="x.jpg">`, cfg)
	if !strings.HasSuffix(strings.TrimSpace(got), "/>") {
		t.Errorf("img tag should be self-closed, got %q", got)
	}
	if strings.Contains(got, `<img src="x.jpg">`) {
		t.Error("bare <img> should have been self-closed")
	}
}

func TestSanitize_ShyToWbr(t *testing.T) {
	cfg := defaultConfig()
	got := sanitizeForKindle("word&shy;break", cfg)
	if strings.Contains(got, "&shy;") {
		t.Error("&shy; should have been replaced")
	}
	if !strings.Contains(got, "<shy/>") && !strings.Contains(got, "<wbr/>") {
		t.Error("expected <shy/> or <wbr/> replacement for &shy;")
	}
}

func TestSanitize_ScriptRemoval(t *testing.T) {
	cfg := defaultConfig()
	html := `<p>Text</p><script type="text/javascript">alert('xss')</script><p>More</p>`
	got := sanitizeForKindle(html, cfg)
	if strings.Contains(got, "<script") || strings.Contains(got, "alert") {
		t.Errorf("script tag should be removed, got: %q", got)
	}
	if !strings.Contains(got, "Text") || !strings.Contains(got, "More") {
		t.Error("surrounding content should be preserved")
	}
}

func TestSanitize_StyleTagRemoval(t *testing.T) {
	cfg := defaultConfig()
	html := `<p>Text</p><style>body{color:red}</style><p>More</p>`
	got := sanitizeForKindle(html, cfg)
	if strings.Contains(got, "<style") || strings.Contains(got, "color:red") {
		t.Errorf("style tag should be removed, got: %q", got)
	}
}

func TestSanitize_TableClass(t *testing.T) {
	cfg := defaultConfig()
	got := sanitizeForKindle("<table>", cfg)
	if !strings.Contains(got, `class="md-table"`) {
		t.Errorf("table should get md-table class, got: %q", got)
	}
}

func TestSanitize_TableClassNotDoubled(t *testing.T) {
	cfg := defaultConfig()
	// If the table already has a class attribute it should still get md-table
	// (the regex replaces the opening <table tag unconditionally).
	got := sanitizeForKindle(`<table class="existing">`, cfg)
	if !strings.Contains(got, `class="md-table"`) {
		t.Errorf("table should have md-table class, got: %q", got)
	}
}

// ── embedLocalImages ──────────────────────────────────────────────────────────

func TestEmbedLocalImages_SkipsHTTP(t *testing.T) {
	html := `<img src="http://example.com/image.jpg">`
	got, imgs, err := embedLocalImages(html, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("should not have loaded any images, got %d", len(imgs))
	}
	if got != html {
		t.Errorf("HTML should be unchanged for remote src, got %q", got)
	}
}

func TestEmbedLocalImages_SkipsHTTPS(t *testing.T) {
	html := `<img src="https://cdn.example.com/photo.png">`
	got, imgs, _ := embedLocalImages(html, "", nil)
	if len(imgs) != 0 {
		t.Errorf("should not have loaded any images, got %d", len(imgs))
	}
	if got != html {
		t.Errorf("HTML should be unchanged for https src")
	}
}

func TestEmbedLocalImages_SkipsDataURI(t *testing.T) {
	html := `<img src="data:image/png;base64,abc123">`
	got, imgs, _ := embedLocalImages(html, "", nil)
	if len(imgs) != 0 {
		t.Errorf("should not have loaded any images for data URI, got %d", len(imgs))
	}
	if got != html {
		t.Errorf("HTML should be unchanged for data URI src")
	}
}

func TestEmbedLocalImages_SkipsProtocolRelative(t *testing.T) {
	html := `<img src="//cdn.example.com/img.jpg">`
	got, imgs, _ := embedLocalImages(html, "", nil)
	if len(imgs) != 0 {
		t.Errorf("should not load protocol-relative URL as local image")
	}
	if got != html {
		t.Errorf("HTML should be unchanged for protocol-relative URL")
	}
}

func TestEmbedLocalImages_MissingFileNonFatal(t *testing.T) {
	// A local path that doesn't exist should leave the tag unchanged and
	// return an error (the caller decides whether to abort).
	html := `<img src="nonexistent_image_xyz.png">`
	got, imgs, err := embedLocalImages(html, "/tmp", nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
	if len(imgs) != 0 {
		t.Errorf("should not add image for missing file, got %d", len(imgs))
	}
	// The tag must be left unchanged so the user sees a broken image
	// rather than silently losing content.
	if got != html {
		t.Errorf("HTML tag should be left unchanged for missing file")
	}
}

// ── headingText ───────────────────────────────────────────────────────────────

func TestHeadingText_Simple(t *testing.T) {
	got := headingText("<h1>My Title</h1>")
	if got != "My Title" {
		t.Errorf("headingText: want %q, got %q", "My Title", got)
	}
}

func TestHeadingText_WithAttributes(t *testing.T) {
	got := headingText(`<h2 id="sec1">Section One</h2>`)
	if got != "Section One" {
		t.Errorf("headingText: want %q, got %q", "Section One", got)
	}
}

func TestHeadingText_InnerTags(t *testing.T) {
	got := headingText("<h1><em>Italic</em> Title</h1>")
	if got != "Italic Title" {
		t.Errorf("headingText strips inner tags: want %q, got %q", "Italic Title", got)
	}
}

func TestHeadingText_Missing(t *testing.T) {
	got := headingText("<p>No heading here</p>")
	if got != "" {
		t.Errorf("headingText with no heading: want %q, got %q", "", got)
	}
}

// ── stripFirstHeading ─────────────────────────────────────────────────────────

func TestStripFirstHeading_RemovesFirst(t *testing.T) {
	html := "<h1>Title</h1><p>Body</p><h1>Second</h1>"
	got := stripFirstHeading(html, 1)
	if strings.Contains(got, "<h1>Title</h1>") {
		t.Error("first h1 should be removed")
	}
	if !strings.Contains(got, "<h1>Second</h1>") {
		t.Error("second h1 should be preserved")
	}
	if !strings.Contains(got, "<p>Body</p>") {
		t.Error("body content should be preserved")
	}
}

func TestStripFirstHeading_OnlyFirst(t *testing.T) {
	// With two identical headings, only the first should be stripped.
	html := "<h2>A</h2><p>x</p><h2>A</h2>"
	got := stripFirstHeading(html, 2)
	count := strings.Count(got, "<h2>A</h2>")
	if count != 1 {
		t.Errorf("expected exactly 1 remaining h2, got %d in: %q", count, got)
	}
}

// ── wrapDiv ───────────────────────────────────────────────────────────────────

func TestWrapDiv(t *testing.T) {
	got := wrapDiv("<p>content</p>")
	want := "<div><p>content</p></div>"
	if got != want {
		t.Errorf("wrapDiv: want %q, got %q", want, got)
	}
}

func TestWrapDiv_Empty(t *testing.T) {
	got := wrapDiv("")
	if got != "<div></div>" {
		t.Errorf("wrapDiv empty: want %q, got %q", "<div></div>", got)
	}
}

// ── titleOrUntitled ───────────────────────────────────────────────────────────

func TestTitleOrUntitled_Empty(t *testing.T) {
	if got := titleOrUntitled(""); got != "Untitled" {
		t.Errorf("titleOrUntitled(%q) = %q, want %q", "", got, "Untitled")
	}
}

func TestTitleOrUntitled_Whitespace(t *testing.T) {
	if got := titleOrUntitled("   "); got != "Untitled" {
		t.Errorf("titleOrUntitled(whitespace) = %q, want %q", got, "Untitled")
	}
}

func TestTitleOrUntitled_NonEmpty(t *testing.T) {
	if got := titleOrUntitled("My Book"); got != "My Book" {
		t.Errorf("titleOrUntitled(%q) = %q, want %q", "My Book", got, "My Book")
	}
}
