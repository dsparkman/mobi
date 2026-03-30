package pdb

import (
	"bytes"
	"testing"
	"time"
)

// FuzzPDBReadDatabase verifies that ReadDatabase never panics on arbitrary
// byte input and that a valid database survives a write→read round-trip.
//
// Properties:
//  1. ReadDatabase never panics (it may return an error).
//  2. A database that was written by Write can always be read back.
//  3. The round-tripped database has the same Name and record count.
func FuzzPDBReadDatabase(f *testing.F) {
	// Seed 1: a minimal hand-crafted valid PalmDB built by the library itself.
	var valid bytes.Buffer
	db := NewDatabase("Fuzz Seed", time.Unix(0, 0))
	db.AddRecord(RawRecord("alpha"))
	db.AddRecord(RawRecord("beta"))
	_ = db.Write(&valid)
	f.Add(valid.Bytes())

	// Seed 2: empty input (should error cleanly).
	f.Add([]byte{})

	// Seed 3: just the magic bytes but truncated.
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})

	// Seed 4: a plausible-looking header with a record count of zero.
	header := make([]byte, PalmDBHeaderLength+2) // header + 2-byte padding
	f.Add(header)

	f.Fuzz(func(t *testing.T, raw []byte) {
		// Property 1: must not panic.
		parsed, err := ReadDatabase(bytes.NewReader(raw))
		if err != nil {
			return // error is fine; panic is not
		}

		// Property 2+3: if parsing succeeded, write→read must be stable.
		var buf bytes.Buffer
		if err := parsed.Write(&buf); err != nil {
			// Write failures on a parsed DB would be unexpected.
			t.Fatalf("Write failed after successful Read: %v", err)
		}

		reparsed, err := ReadDatabase(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("re-Read failed after Write of parsed DB: %v", err)
		}
		if parsed.Name != reparsed.Name {
			t.Fatalf("round-trip name mismatch: %q → %q", parsed.Name, reparsed.Name)
		}
		if len(parsed.Records) != len(reparsed.Records) {
			t.Fatalf("round-trip record count mismatch: %d → %d",
				len(parsed.Records), len(reparsed.Records))
		}
	})
}
