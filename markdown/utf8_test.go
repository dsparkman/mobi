package markdown

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ── validateUTF8 ─────────────────────────────────────────────────────────────

func TestValidateUTF8_ValidASCII(t *testing.T) {
	if err := validateUTF8([]byte("Hello, World!")); err != nil {
		t.Errorf("valid ASCII rejected: %v", err)
	}
}

func TestValidateUTF8_ValidMultibyte(t *testing.T) {
	cases := []string{
		"日本語テスト",             // Japanese
		"Héllo Wörld",        // Latin extended
		"中文内容",               // Chinese
		"Ελληνικά",           // Greek
		"한국어",                // Korean
		"العربية",            // Arabic (RTL)
		"עִברִית",            // Hebrew (RTL)
		"Привет мир",         // Cyrillic
		"🎉📚🔖",                // Emoji (4-byte sequences)
		"café résumé naïve",  // Common accented Latin
		"\u00e9\u00e8\u00ea", // é è ê (2-byte each)
		"\u4e2d\u6587",       // 中文 (3-byte each)
		"\U0001F4DA",         // 📚 (4-byte)
	}
	for _, s := range cases {
		if err := validateUTF8([]byte(s)); err != nil {
			t.Errorf("valid UTF-8 %q rejected: %v", s, err)
		}
	}
}

func TestValidateUTF8_InvalidSequence(t *testing.T) {
	// 0x80 is a UTF-8 continuation byte that cannot appear alone.
	bad := []byte{0x48, 0x65, 0x80, 0x6c, 0x6f} // "He\x80lo"
	err := validateUTF8(bad)
	if err == nil {
		t.Fatal("expected error for invalid UTF-8 byte sequence")
	}
	// Error message must mention the byte offset.
	if !strings.Contains(err.Error(), "offset 2") {
		t.Errorf("error should mention offset 2, got: %v", err)
	}
}

func TestValidateUTF8_InvalidSequenceAtStart(t *testing.T) {
	bad := []byte{0xFF, 0x48, 0x65, 0x6c, 0x6c, 0x6f}
	err := validateUTF8(bad)
	if err == nil {
		t.Fatal("expected error for invalid UTF-8 at offset 0")
	}
	if !strings.Contains(err.Error(), "offset 0") {
		t.Errorf("error should mention offset 0, got: %v", err)
	}
}

func TestValidateUTF8_InvalidSequenceAtEnd(t *testing.T) {
	// A truncated multi-byte sequence at the end of the input.
	bad := []byte("Hello\xc3") // \xc3 starts a 2-byte sequence but nothing follows
	err := validateUTF8(bad)
	if err == nil {
		t.Fatal("expected error for truncated UTF-8 sequence")
	}
}

func TestValidateUTF8_Empty(t *testing.T) {
	// Empty input is valid UTF-8.
	if err := validateUTF8([]byte{}); err != nil {
		t.Errorf("empty input should be valid: %v", err)
	}
}

func TestValidateUTF8_AllASCIIBytes(t *testing.T) {
	// All 128 ASCII byte values (0x00–0x7F) are valid single-byte UTF-8.
	ascii := make([]byte, 128)
	for i := range ascii {
		ascii[i] = byte(i)
	}
	if err := validateUTF8(ascii); err != nil {
		t.Errorf("all ASCII bytes should be valid UTF-8: %v", err)
	}
}

// ── safeSliceHTML ─────────────────────────────────────────────────────────────

func TestSafeSliceHTML_ASCII(t *testing.T) {
	s := "<h1>Title</h1><p>Body</p>"
	got, err := safeSliceHTML(s, 0, 15) // "<h1>Title</h1>"
	if err != nil {
		t.Fatalf("safeSliceHTML error: %v", err)
	}
	if got != s[:15] {
		t.Errorf("want %q, got %q", s[:15], got)
	}
}

func TestSafeSliceHTML_MultiByteSafeAtTagBoundary(t *testing.T) {
	// Japanese heading followed by paragraph — the slice point is the '<'
	// at the start of <p>, which is ASCII and always a rune boundary.
	s := "<h1>日本語の見出し</h1><p>テキスト</p>"
	// Find the '<p' position (always ASCII, always a rune boundary).
	pStart := strings.Index(s, "<p>")
	if pStart < 0 {
		t.Fatal("test setup: <p> not found")
	}
	// Slice up to <p> — all of the h1 content.
	got, err := safeSliceHTML(s, 0, pStart)
	if err != nil {
		t.Fatalf("safeSliceHTML error on multibyte content: %v", err)
	}
	if !strings.Contains(got, "日本語") {
		t.Errorf("expected Japanese text in slice, got %q", got)
	}
}

func TestSafeSliceHTML_MidRuneDetected(t *testing.T) {
	// 'é' is U+00E9: bytes 0xC3 0xA9. Slicing at offset 1 lands in the middle.
	s := "aéb"
	// 'a' is at 0, 'é' starts at 1 (2 bytes: 0xC3 0xA9), 'b' is at 3.
	_, err := safeSliceHTML(s, 0, 2) // offset 2 = 0xA9, a continuation byte
	if err == nil {
		t.Fatal("expected error: slice end at mid-rune byte")
	}
	if !strings.Contains(err.Error(), "mid-rune") {
		t.Errorf("error should mention mid-rune, got: %v", err)
	}
}

func TestSafeSliceHTML_OutOfRange(t *testing.T) {
	s := "hello"
	_, err := safeSliceHTML(s, 0, 100)
	if err == nil {
		t.Fatal("expected error for out-of-range end index")
	}
}

func TestSafeSliceHTML_StartGreaterThanEnd(t *testing.T) {
	s := "hello"
	_, err := safeSliceHTML(s, 3, 1)
	if err == nil {
		t.Fatal("expected error for start > end")
	}
}

func TestSafeSliceHTML_EmptySlice(t *testing.T) {
	s := "hello"
	got, err := safeSliceHTML(s, 2, 2)
	if err != nil {
		t.Fatalf("empty slice should succeed: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ── Integration: full pipeline with non-ASCII content ─────────────────────────

func TestConvertBytes_JapaneseMarkdown(t *testing.T) {
	md := `---
title: "日本語の本"
author: "山田太郎"
language: "ja"
---

# 第一章：始まり

これは最初の章です。日本語のテキストが正しく処理されることを確認します。

## セクション 1.1

詳細な内容がここに入ります。

# 第二章：終わり

これは最後の章です。
`
	cfg := defaultConfig()
	cfg.title = ""
	cfg.language = "ja"

	merged, err := mergeFrontMatter([]byte(md), cfg)
	if err != nil {
		t.Fatalf("mergeFrontMatter: %v", err)
	}
	if merged.title != "日本語の本" {
		t.Errorf("title: want %q, got %q", "日本語の本", merged.title)
	}

	// Verify the source validates as UTF-8.
	if err := validateUTF8([]byte(md)); err != nil {
		t.Fatalf("Japanese source rejected as invalid UTF-8: %v", err)
	}

	// Verify rendered HTML is valid UTF-8.
	html, err := renderMarkdown([]byte(md), merged)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if !utf8.ValidString(html) {
		t.Error("rendered HTML is not valid UTF-8")
	}

	// Verify chapter splitting preserves multi-byte content.
	chapters, _, err := buildChapters(html, merged)
	if err != nil {
		t.Fatalf("buildChapters: %v", err)
	}
	if len(chapters) < 2 {
		t.Errorf("expected at least 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title() != "第一章：始まり" {
		t.Errorf("chapter 0 title: want %q, got %q", "第一章：始まり", chapters[0].Title())
	}
}

func TestConvertBytes_RTLArabicMarkdown(t *testing.T) {
	md := `# مرحبا

هذا نص عربي. يجب أن يعمل بشكل صحيح مع تنسيق MOBI.

## قسم فرعي

محتوى القسم الفرعي.
`
	if err := validateUTF8([]byte(md)); err != nil {
		t.Fatalf("Arabic source rejected as invalid UTF-8: %v", err)
	}

	html, err := renderMarkdown([]byte(md), defaultConfig())
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if !utf8.ValidString(html) {
		t.Error("Arabic rendered HTML is not valid UTF-8")
	}
	if !strings.Contains(html, "مرحبا") {
		t.Error("Arabic heading text lost during rendering")
	}
}

func TestConvertBytes_EmojiInContent(t *testing.T) {
	md := "# Chapter 🎉\n\nContent with emoji 📚 and more text.\n"
	if err := validateUTF8([]byte(md)); err != nil {
		t.Fatalf("emoji source rejected: %v", err)
	}

	html, err := renderMarkdown([]byte(md), defaultConfig())
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if !utf8.ValidString(html) {
		t.Error("emoji HTML is not valid UTF-8")
	}
	// Emoji must survive the pipeline unchanged.
	if !strings.Contains(html, "🎉") {
		t.Error("emoji 🎉 lost during rendering")
	}
}

func TestConvertBytes_InvalidUTF8Rejected(t *testing.T) {
	// Inject an invalid byte sequence that looks plausible (Latin-1 accented char)
	// — a common mistake when copy-pasting from Windows editors.
	bad := []byte("# Title\n\nCaf\xe9 au lait.\n") // \xe9 is é in Latin-1, not UTF-8
	err := validateUTF8(bad)
	if err == nil {
		t.Fatal("expected validateUTF8 to reject Latin-1 encoded content")
	}
}

// ── runeCount ─────────────────────────────────────────────────────────────────

func TestRuneCount(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"hello", 5},
		{"日本語", 3},  // 3 runes, 9 bytes
		{"café", 4}, // 4 runes, 5 bytes
		{"🎉📚", 2},   // 2 runes, 8 bytes
		{"", 0},
	}
	for _, tc := range cases {
		got := runeCount(tc.s)
		if got != tc.want {
			t.Errorf("runeCount(%q): want %d, got %d", tc.s, tc.want, got)
		}
	}
}
