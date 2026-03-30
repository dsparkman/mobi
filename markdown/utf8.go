package markdown

import (
	"fmt"
	"unicode/utf8"
)

// validateUTF8 checks that src is valid UTF-8 and returns a descriptive error
// if not. It is called at the very start of the conversion pipeline so that a
// bad byte sequence is caught once, at the entry point, rather than discovered
// mid-pipeline as a garbled character or silently written into the output file.
//
// The error includes the exact byte offset of the first invalid sequence so
// callers can locate the problem in their source file:
//
//	// Latin-1 'é' (0xe9) is not valid UTF-8:
//	err := validateUTF8([]byte("caf\xe9"))
//	// err: "markdown: input is not valid UTF-8: invalid byte sequence at offset 3 (0xe9)"
//
// Why this matters: the MOBI format mandates UTF-8 encoding (header field
// value 65001). A non-UTF-8 source produces a file that Kindle firmware may
// reject or render incorrectly.
//
// # Style note — intentional exception to the range-loop guideline
//
// The loop below uses the classic C-style `for i := 0; i < len(src);` form
// rather than `for i, b := range src`. This is a deliberate exception.
//
// [utf8.DecodeRune] returns the byte width of the decoded rune (1–4 bytes).
// The loop body advances `i += size` to skip past the full multi-byte sequence.
// A `for range` loop over a []byte advances by exactly 1 byte per iteration
// and would therefore examine every continuation byte as a fresh start,
// producing wrong offset values and possibly missing the error entirely.
//
// The manual index loop is the correct and idiomatic pattern for this specific
// use case. It is not a candidate for modernisation.
func validateUTF8(src []byte) error {
	if utf8.Valid(src) {
		return nil
	}
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf(
				"markdown: input is not valid UTF-8: invalid byte sequence at offset %d (0x%02x)",
				i, src[i],
			)
		}
		i += size
	}
	return nil // unreachable; satisfies the compiler
}

// safeSliceHTML returns html[start:end], first verifying that both indices
// fall on UTF-8 rune boundaries. In practice every cut point produced by the
// chapter splitter is the start of an ASCII '<' byte (0x3C), so this can
// never actually fail — the Go regexp package guarantees that match indices
// are on rune boundaries for string inputs. The check exists to make that
// invariant explicit and to surface any future regression as a clear error
// rather than a silently corrupted character.
func safeSliceHTML(html string, start, end int) (string, error) {
	if start < 0 || end > len(html) || start > end {
		return "", fmt.Errorf(
			"markdown: slice [%d:%d] out of range for html of length %d",
			start, end, len(html),
		)
	}
	if start > 0 && !utf8.RuneStart(html[start]) {
		return "", fmt.Errorf(
			"markdown: slice start %d is mid-rune (byte 0x%02x is a UTF-8 continuation byte)",
			start, html[start],
		)
	}
	if end < len(html) && !utf8.RuneStart(html[end]) {
		return "", fmt.Errorf(
			"markdown: slice end %d is mid-rune (byte 0x%02x is a UTF-8 continuation byte)",
			end, html[end],
		)
	}
	return html[start:end], nil
}

// runeCount returns the number of Unicode code points in s.
// Used in tests and diagnostics.
func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}
