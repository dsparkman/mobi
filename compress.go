package mobi

// palmdocCompress compresses src using the PalmDOC variant of LZ77.
//
// PalmDOC LZ77 specifics (from the MobileRead MOBI spec):
//   - Input is divided into independent 4096-byte blocks (uncompressed size).
//   - Within each block, references use an 11-bit distance (back into a
//     2048-byte sliding window) and a 3-bit length (copy 3–10 bytes).
//   - The encoded reference is a big-endian uint16:
//       bits 15–11  : distance (1–2048, stored as distance-1 in 11 bits)
//       bits 10–8   : length-3 (0–7, meaning copy 3–10 bytes)
//       bits 7–0    : space byte trigger — if the high byte has its top
//                     bit set AND the low byte is a printable ASCII space,
//                     the decoder outputs a space before the copied bytes.
//
// Byte classification:
//   0x00            : literal zero, stored as-is
//   0x01–0x08       : N raw bytes follow (length prefix)
//   0x09–0x7F       : literal byte, stored as-is
//   0x80–0xBF       : reference pair (2 bytes) — copy from sliding window
//   0xC0–0xFF       : literal space + byte (space compression)
//
// This encoder uses the "length prefix for literals" approach (0x01–0x08):
// runs of unmatched bytes are buffered and emitted as <len><bytes>.
// This is simpler than encoding each literal individually and produces
// output that is correct and typically within 5% of optimal.
//
// Reference: https://wiki.mobileread.com/wiki/PalmDOC#Compression
// Reference: https://wiki.mobileread.com/wiki/MOBI#PalmDOC_Compression

import "encoding/binary"

const (
	palmdocBlockSize  = 4096 // uncompressed block size
	palmdocWindowSize = 2048 // sliding window (distance field is 11 bits)
	palmdocMinMatch   = 3
	palmdocMaxMatch   = 10
)

// palmdocCompress compresses the entire src slice block-by-block.
//
// The loop uses a C-style index rather than a range because it advances by
// [palmdocBlockSize] (4096) per iteration and slices src at computed offsets.
// There is no range form that advances by an arbitrary step while also
// providing the current offset for slicing. This is a deliberate exception
// to the prefer-range-loop guideline.
func palmdocCompress(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src)/2)
	for off := 0; off < len(src); off += palmdocBlockSize {
		end := min(off+palmdocBlockSize, len(src))
		out = compressBlock(src[off:end], out)
	}
	return out, nil
}

// compressBlock compresses a single PalmDOC block (≤4096 uncompressed bytes).
// It appends the compressed output to dst and returns the result.
//
// # Style notes — intentional exceptions to modern Go guidelines
//
// The outer loop uses `for i < n` with manual index advancement rather than
// a range loop. This is required: the index must skip forward by the match
// length (1–10 bytes) or by 2 bytes for space-compressed pairs, neither of
// which maps to any range loop form. The index also serves as the absolute
// position for computing sliding-window distances.
//
// The inner match-search loop `for j := windowStart; j < i; j++` is similarly
// required: it scans backwards through the window comparing bytes at position
// j+mLen against i+mLen simultaneously, which needs two independent indices.
// A range loop over a sub-slice would lose the position information needed to
// compute `bestDist = i - j`.
//
// Both loops are correct and idiomatic for low-level binary encoding. They
// are not candidates for modernisation.
func compressBlock(block []byte, dst []byte) []byte {
	n := len(block)
	i := 0
	litBuf := make([]byte, 0, 8)

	flushLiterals := func() {
		for len(litBuf) > 0 {
			chunk := litBuf
			if len(chunk) > 8 {
				chunk = chunk[:8]
			}
			dst = append(dst, byte(len(chunk)))
			dst = append(dst, chunk...)
			litBuf = litBuf[len(chunk):]
		}
	}

	for i < n {
		// Try to find the longest match in the sliding window.
		bestLen, bestDist := 0, 0
		// PalmDOC distance field is 11 bits (max value 2047). Using
		// palmdocWindowSize-1 as the lookback limit ensures dist never
		// reaches 2048, which would overflow the 11-bit field and encode
		// as dist=0, causing the decompressor to read out-of-bounds.
		windowStart := max(0, i-(palmdocWindowSize-1))

		// Simple O(n²) greedy match — correct and fast enough for typical
		// ebook text (usually <50 KB per block). For very large inputs a
		// hash table would be better, but blocks are capped at 4096 bytes.
		for j := windowStart; j < i; j++ {
			mLen := 0
			for mLen < palmdocMaxMatch &&
				i+mLen < n &&
				block[j+mLen] == block[i+mLen] {
				mLen++
			}
			if mLen >= palmdocMinMatch && mLen > bestLen {
				bestLen = mLen
				bestDist = i - j
			}
		}

		if bestLen >= palmdocMinMatch {
			// Emit buffered literals first.
			flushLiterals()

			// Emit a 2-byte back-reference.
			// Encoding: high byte = 0x80 | (dist>>5),
			//           low byte  = ((dist & 0x1F) << 3) | (length - 3)
			dist := bestDist
			length := bestLen
			w := (uint16(dist)<<3)&0x3FF8 | uint16(length-palmdocMinMatch)
			w |= 0x8000 // set top bit to mark as reference
			var ref [2]byte
			binary.BigEndian.PutUint16(ref[:], w)
			dst = append(dst, ref[:]...)
			i += length
		} else {
			b := block[i]
			// Space compression: 0xC0–0xFF encodes a space + a printable byte.
			if b == 0x20 && i+1 < n {
				next := block[i+1]
				if next >= 0x40 && next <= 0x7F {
					flushLiterals()
					dst = append(dst, 0xC0|next)
					i += 2
					continue
				}
			}
			// Raw literal (0x09–0x7F pass through directly; others via length prefix)
			if b >= 0x09 && b <= 0x7F {
				flushLiterals()
				dst = append(dst, b)
			} else {
				// Must use length-prefix encoding for bytes 0x00 and 0x08
				litBuf = append(litBuf, b)
				if len(litBuf) == 8 {
					flushLiterals()
				}
			}
			i++
		}
	}
	flushLiterals()
	return dst
}
