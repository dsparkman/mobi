package mobi

import (
	"bytes"
	"strings"
	"testing"
)

// palmdocDecompress is the reference decoder used only in tests to verify
// round-trip correctness of the compressor. Not exported — production readers
// are the Kindle device itself and Calibre.
func palmdocDecompress(src []byte) []byte {
	out := make([]byte, 0, len(src)*2)
	i := 0
	for i < len(src) {
		b := src[i]
		i++
		switch {
		case b == 0x00:
			out = append(out, 0x00)
		case b >= 0x01 && b <= 0x08:
			count := int(b)
			out = append(out, src[i:i+count]...)
			i += count
		case b <= 0x7F:
			out = append(out, b)
		case b >= 0x80 && b <= 0xBF:
			b2 := src[i]
			i++
			w := (uint16(b) << 8) | uint16(b2)
			dist := int((w & 0x3FF8) >> 3)
			length := int(w&0x0007) + 3
			from := len(out) - dist
			for range length {
				out = append(out, out[from])
				from++
			}
		default: // 0xC0–0xFF: space + byte
			out = append(out, 0x20, b&0x7F)
		}
	}
	return out
}

func TestPalmDocRoundTrip_Simple(t *testing.T) {
	cases := []string{
		"Hello, World!",
		"the the the the the the",
		"AAAAABBBBBCCCCC",
		strings.Repeat("Go is a statically typed language. ", 50),
		"",
		string(make([]byte, 100)), // all zeros
	}
	for _, tc := range cases {
		compressed, err := palmdocCompress([]byte(tc))
		if err != nil {
			t.Fatalf("compress(%q): %v", tc[:min(len(tc), 30)], err)
		}
		got := palmdocDecompress(compressed)
		if !bytes.Equal(got, []byte(tc)) {
			t.Errorf("round-trip mismatch for %q:\nwant %d bytes\ngot  %d bytes",
				tc[:min(len(tc), 30)], len(tc), len(got))
		}
	}
}

func TestPalmDocRoundTrip_LargeProseBlock(t *testing.T) {
	prose := strings.Repeat(
		"It was the best of times, it was the worst of times, "+
			"it was the age of wisdom, it was the age of foolishness. ", 40)
	compressed, err := palmdocCompress([]byte(prose))
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, []byte(prose)) {
		t.Errorf("round-trip mismatch: want %d bytes, got %d", len(prose), len(got))
	}
	ratio := float64(len(compressed)) / float64(len(prose))
	t.Logf("compression ratio: %.2f (%.0f → %.0f bytes)", ratio, float64(len(prose)), float64(len(compressed)))
	if ratio > 0.85 {
		t.Errorf("expected ratio < 0.85 for repetitive prose, got %.2f", ratio)
	}
}

func TestPalmDocRoundTrip_Binary(t *testing.T) {
	src := make([]byte, 256)
	for i := range src {
		src[i] = byte(i)
	}
	compressed, err := palmdocCompress(src)
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, src) {
		t.Errorf("binary round-trip mismatch")
	}
}

func TestPalmDocRoundTrip_MultiBlock(t *testing.T) {
	// Input spans multiple 4096-byte blocks to exercise inter-block boundaries.
	// Blocks are compressed independently, so a match cannot cross blocks.
	src := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 200))
	if len(src) <= 4096 {
		t.Fatalf("test input too short (%d bytes); needs >4096 to cover multi-block", len(src))
	}
	compressed, err := palmdocCompress(src)
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, src) {
		t.Errorf("multi-block round-trip mismatch: want %d bytes, got %d", len(src), len(got))
	}
}

func TestPalmDocRoundTrip_MaxWindowDistance(t *testing.T) {
	// A sequence where the best match is exactly 2047 bytes back (max legal dist).
	// Anything farther would overflow the 11-bit distance field.
	pad := bytes.Repeat([]byte("x"), 2047)
	marker := []byte("UNIQUE_MARKER")
	// Layout: [2047 bytes padding][marker][2047 bytes padding][marker]
	// When compressing the second marker the best match is 2047+len(marker) back,
	// well within the window. The first copy at dist=2047+13 is outside the window
	// so only the second repetition is compressed.
	src := append(append(append(pad, marker...), pad...), marker...)
	compressed, err := palmdocCompress(src)
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, src) {
		t.Errorf("max-window-distance round-trip mismatch")
	}
}

func TestPalmDocRoundTrip_SpaceCompression(t *testing.T) {
	// Space + printable-ASCII pairs use the 0xC0–0xFF encoding path.
	src := []byte(" A B C D E F  G H I J K L M N O P Q R S T U V W X Y Z")
	compressed, err := palmdocCompress(src)
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, src) {
		t.Errorf("space-compression round-trip mismatch: want %q, got %q", src, got)
	}
}

func TestPalmDocRoundTrip_AllZeros(t *testing.T) {
	// Zero bytes must be encoded via the length-prefix path (0x01–0x08).
	src := make([]byte, 64)
	compressed, err := palmdocCompress(src)
	if err != nil {
		t.Fatal(err)
	}
	got := palmdocDecompress(compressed)
	if !bytes.Equal(got, src) {
		t.Errorf("all-zeros round-trip mismatch")
	}
}

func TestPalmDocRoundTrip_SingleByte(t *testing.T) {
	for _, b := range []byte{0x00, 0x01, 0x08, 0x09, 0x7F, 0x80, 0xBF, 0xC0, 0xFF} {
		src := []byte{b}
		compressed, err := palmdocCompress(src)
		if err != nil {
			t.Fatalf("compress(0x%02X): %v", b, err)
		}
		got := palmdocDecompress(compressed)
		if !bytes.Equal(got, src) {
			t.Errorf("single-byte 0x%02X round-trip mismatch: want %v, got %v", b, src, got)
		}
	}
}
