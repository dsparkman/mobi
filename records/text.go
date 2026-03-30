package records

import "io"

const TextRecordMaxSize = 4096 // 0x1000

type TextRecord struct {
	data  []byte
	trail []byte
}

func NewTextRecord(s string, trail TrailingData) TextRecord {
	if len(s) > TextRecordMaxSize {
		panic("TextRecord too large")
	}
	return TextRecord{
		data:  []byte(s),
		trail: trail.Encode(),
	}
}

// NewCompressedTextRecord stores already-compressed PalmDOC bytes as a text
// record. The size limit is not enforced on the compressed payload because
// compressed data is typically smaller than the 4096-byte uncompressed limit,
// and in the worst case (incompressible input) only marginally larger.
func NewCompressedTextRecord(data []byte, trail TrailingData) TextRecord {
	return TextRecord{
		data:  data,
		trail: trail.Encode(),
	}
}

func (r TextRecord) Write(w io.Writer) error {
	_, err := w.Write(r.data)
	if err != nil {
		return err
	}

	_, err = w.Write(r.trail)
	return err
}

func (r TextRecord) Length() int {
	return len(r.data) + len(r.trail)
}
