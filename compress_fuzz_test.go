package mobi

import (
	"bytes"
	"testing"
)

// FuzzPalmDoc verifies that for any byte sequence, compress→decompress
// produces the original input. This is the most important correctness
// property of the compressor.
func FuzzPalmDoc(f *testing.F) {
	// Seed corpus: edge cases likely to expose encoding bugs
	seeds := [][]byte{
		{},
		{0x00},
		{0x08},
		{0x20, 0x41},                     // space + 'A' — space compression trigger
		{0x20, 0x20},                     // two spaces — second is NOT in 0x40–0x7F range
		make([]byte, 4096),               // full block of zeros
		make([]byte, 4097),               // crosses block boundary
		bytes.Repeat([]byte("abc"), 500), // repetitive
		func() []byte { // all byte values
			b := make([]byte, 256)
			for i := range b {
				b[i] = byte(i)
			}
			return b
		}(),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		compressed, err := palmdocCompress(src)
		if err != nil {
			t.Fatalf("compress error: %v", err)
		}
		got := palmdocDecompress(compressed)
		if !bytes.Equal(got, src) {
			t.Fatalf("round-trip mismatch:\ninput    len=%d\noutput   len=%d",
				len(src), len(got))
		}
	})
}
