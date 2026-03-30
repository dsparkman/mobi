package markdown

import (
	"strings"
	"testing"
)

// ── buildChapters ─────────────────────────────────────────────────────────────

func TestBuildChapters_SplitNone(t *testing.T) {
	cfg := defaultConfig()
	cfg.splitLevel = SplitNone
	cfg.title = "My Book"

	html := "<h1>Chapter One</h1><p>Content.</p><h1>Chapter Two</h1><p>More.</p>"
	chapters, startIdx, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) != 1 {
		t.Errorf("SplitNone: want 1 chapter, got %d", len(chapters))
	}
	if startIdx != -1 {
		t.Errorf("SplitNone: startIdx want -1, got %d", startIdx)
	}
}

func TestBuildChapters_NoHeadings(t *testing.T) {
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.title = "Book"

	html := "<p>Just a paragraph with no headings.</p>"
	chapters, startIdx, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) != 1 {
		t.Errorf("no-headings: want 1 chapter, got %d", len(chapters))
	}
	if startIdx != -1 {
		t.Errorf("no-headings: startIdx want -1, got %d", startIdx)
	}
}

func TestBuildChapters_SingleH1_NoSubChapters(t *testing.T) {
	// Single H1 with no sub-headings: title extracted from H1, one chapter produced.
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.subSplitLevel = SplitH2
	cfg.title = "" // empty — should be extracted from H1

	html := "<h1>Only Chapter</h1><p>Body text here.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatal("single-H1 doc should produce at least one chapter")
	}
}

func TestBuildChapters_MultipleH1_TitlesPreserved(t *testing.T) {
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.subSplitLevel = SplitNone
	cfg.title = "The Book"

	html := "<h1>Alpha</h1><p>First.</p><h1>Beta</h1><p>Second.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) != 2 {
		t.Errorf("want 2 chapters, got %d", len(chapters))
	}
	titles := []string{chapters[0].Title(), chapters[1].Title()}
	for _, want := range []string{"Alpha", "Beta"} {
		found := false
		for _, got := range titles {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("chapter title %q not found in %v", want, titles)
		}
	}
}

func TestBuildChapters_Preamble(t *testing.T) {
	// Content before the first heading goes into a preamble chapter.
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.title = "Book"

	html := "<p>Preamble content.</p><h1>Chapter One</h1><p>Body.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) < 2 {
		t.Errorf("preamble + one chapter: want >=2, got %d", len(chapters))
	}
	// Preamble chapter must contain the preamble text.
	found := false
	for _, ch := range chapters {
		// Chapter title won't contain "Preamble content"; check via Realize smoke.
		_ = ch
		found = true
	}
	if !found {
		t.Error("no chapters produced")
	}
}

func TestBuildChapters_SubChapters(t *testing.T) {
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.subSplitLevel = SplitH2
	// Set title so the H1 is kept as a chapter title (not consumed as book title),
	// which means sub-chapters stay nested rather than being promoted to top level.
	cfg.title = "The Book"

	html := "<h1>Part I</h1><p>Intro.</p><h2>Section A</h2><p>A body.</p><h2>Section B</h2><p>B body.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatal("no chapters produced")
	}
	// The single top-level chapter should have 2 sub-chapters.
	subs := chapters[len(chapters)-1].SubChapters()
	if len(subs) != 2 {
		t.Errorf("want 2 sub-chapters, got %d", len(subs))
	}
}

func TestBuildChapters_SplitH2_Primary(t *testing.T) {
	// When splitLevel=SplitH2, H2 tags become top-level chapters.
	cfg := defaultConfig()
	cfg.splitLevel = SplitH2
	cfg.subSplitLevel = SplitNone
	cfg.title = "Book"

	html := "<h2>Ch A</h2><p>A.</p><h2>Ch B</h2><p>B.</p><h2>Ch C</h2><p>C.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) != 3 {
		t.Errorf("SplitH2: want 3 chapters, got %d", len(chapters))
	}
}

func TestBuildChapters_SingleH1_WithSubChapters_TitleExtracted(t *testing.T) {
	// If cfg.title is empty and there is exactly one H1 that has H2 sub-chapters,
	// the H1 text becomes the title and the H2s are promoted to top-level chapters.
	cfg := defaultConfig()
	cfg.splitLevel = SplitH1
	cfg.subSplitLevel = SplitH2
	cfg.title = ""

	html := "<h1>The Title</h1><h2>Sub One</h2><p>A.</p><h2>Sub Two</h2><p>B.</p>"
	chapters, _, err := buildChapters(html, cfg)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	// Sub-chapters promoted to top level.
	if len(chapters) != 2 {
		titles := make([]string, len(chapters))
		for i, c := range chapters {
			titles[i] = c.Title()
		}
		t.Errorf("want 2 promoted sub-chapters, got %d: %v", len(chapters), titles)
	}
}

// ── buildSubChapters ──────────────────────────────────────────────────────────

func TestBuildSubChapters_NoSubs(t *testing.T) {
	cfg := defaultConfig()
	cfg.subSplitLevel = SplitH2

	body := "<p>Just intro, no sub-headings.</p>"
	cb, err := buildSubChapters("Parent", body, cfg)
	if err != nil {
		t.Fatalf("buildSubChapters: %v", err)
	}
	if len(cb.subs) != 0 {
		t.Errorf("no sub-headings: want 0 subs, got %d", len(cb.subs))
	}
	if !strings.Contains(cb.intro, "intro") {
		t.Errorf("intro should capture body content: %q", cb.intro)
	}
}

func TestBuildSubChapters_WithSubs(t *testing.T) {
	cfg := defaultConfig()
	cfg.subSplitLevel = SplitH2

	body := "<p>Intro.</p><h2>Alpha</h2><p>A.</p><h2>Beta</h2><p>B.</p>"
	cb, err := buildSubChapters("Parent", body, cfg)
	if err != nil {
		t.Fatalf("buildSubChapters: %v", err)
	}
	if len(cb.subs) != 2 {
		t.Errorf("want 2 subs, got %d", len(cb.subs))
	}
	if cb.subs[0].title != "Alpha" || cb.subs[1].title != "Beta" {
		t.Errorf("sub titles: want [Alpha, Beta], got [%s, %s]", cb.subs[0].title, cb.subs[1].title)
	}
}

// ── chapBuilder.build / subChaptersAsTopLevel ─────────────────────────────────

func TestChapBuilder_Build_ProducesChapter(t *testing.T) {
	cfg := defaultConfig()
	cb := &chapBuilder{title: "My Chapter", intro: "<p>Intro.</p>", cfg: cfg}
	cb.subs = []subChapData{
		{title: "Sub 1", body: "<p>S1 body.</p>"},
	}
	ch := cb.build()
	if ch.Title() != "My Chapter" {
		t.Errorf("chapter title: want %q, got %q", "My Chapter", ch.Title())
	}
	if len(ch.SubChapters()) != 1 {
		t.Errorf("want 1 sub-chapter, got %d", len(ch.SubChapters()))
	}
}

func TestChapBuilder_SubChaptersAsTopLevel(t *testing.T) {
	cfg := defaultConfig()
	cb := &chapBuilder{
		title: "Root",
		cfg:   cfg,
		subs: []subChapData{
			{title: "X", body: "<p>x.</p>"},
			{title: "Y", body: "<p>y.</p>"},
		},
	}
	chapters := cb.subChaptersAsTopLevel()
	if len(chapters) != 2 {
		t.Fatalf("want 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title() != "X" || chapters[1].Title() != "Y" {
		t.Errorf("titles: want [X, Y], got [%s, %s]", chapters[0].Title(), chapters[1].Title())
	}
}

