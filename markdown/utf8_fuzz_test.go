package markdown

import (
	"testing"
	"unicode/utf8"
)

// FuzzValidateUTF8 verifies the consistency invariant:
// validateUTF8(b) == nil  ⟺  utf8.Valid(b)
//
// The stdlib is the oracle. Any divergence is a bug in validateUTF8.
func FuzzValidateUTF8(f *testing.F) {
	seeds := [][]byte{
		{},
		[]byte("Hello, World!"),
		[]byte("日本語"),
		[]byte("café"),
		[]byte("🎉📚"),
		{0x80},              // bare continuation byte — invalid
		{0xC3, 0xA9},        // é — valid 2-byte sequence
		{0xC3},              // truncated 2-byte sequence — invalid
		{0xE2, 0x82, 0xAC},  // € — valid 3-byte sequence
		{0xE2, 0x82},        // truncated 3-byte sequence — invalid
		{0xFF},              // 0xFF is never valid in UTF-8
		{0x48, 0x65, 0x80},  // "He" + invalid
		{0xED, 0xA0, 0x80},  // U+D800 surrogate — overlong/invalid in UTF-8
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		err := validateUTF8(b)
		stdlibSays := utf8.Valid(b)
		ourSays := err == nil
		if ourSays != stdlibSays {
			t.Fatalf("validateUTF8 disagrees with utf8.Valid for input %v:\n"+
				"  validateUTF8 returned error=%v (ok=%v)\n"+
				"  utf8.Valid returned %v",
				b, err, ourSays, stdlibSays)
		}
	})
}
