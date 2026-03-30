package mobi

import (
	"fmt"
	"strings"
	"text/template"

	r "github.com/dsparkman/mobi/records"
)

// chaptersToText assembles all chapter content into a single UTF-8 string
// and produces the ChunkInfo / ChapterInfo slices needed by the KF8 index
// records.
//
// The assembled string has the structure:
//
//	[skeleton-head][chunk-body][skeleton-head][chunk-body]...
//
// Each skeleton-head is the output of the KF8 HTML template (the full
// <html>...<body aid="XXXX"> wrapper). Each chunk-body is the caller's
// raw HTML fragment.
//
// SubChapters are included in the text stream contiguously after their
// parent chapter's initial chunks. Their ChapterInfo entries record byte
// offsets that point into the middle of the parent chapter's region,
// allowing the NCX to navigate to them independently.
func chaptersToText(b *Book, tpl *template.Template) (
	html string,
	chunks []r.ChunkInfo,
	chaps []r.ChapterInfo,
	err error,
) {
	buf := new(strings.Builder)
	// Pre-allocate with capacity estimates to avoid repeated re-allocation.
	chunks = make([]r.ChunkInfo, 0, len(b.chapters)*2)
	chaps = make([]r.ChapterInfo, 0, len(b.chapters)*2)
	chunkID := 0

	for chapID, chap := range b.chapters {
		chapStart := buf.Len()

		// Emit parent chapter chunks.
		for _, chunk := range chap.chunks {
			inv := newInventory(b, chap.title, chapID, chunkID)
			head, tErr := runTemplate(*tpl, inv)
			if tErr != nil {
				return "", nil, nil, fmt.Errorf("chapter %d: %w", chapID, tErr)
			}
			chunks = append(chunks, r.ChunkInfo{
				PreStart:      buf.Len(),
				PreLength:     len(head),
				ContentStart:  buf.Len() + len(head),
				ContentLength: len(chunk.Body),
			})
			buf.WriteString(head)
			buf.WriteString(chunk.Body)
			chunkID++
		}

		// NCX entry for this chapter.
		chaps = append(chaps, r.ChapterInfo{
			Title:  chap.title,
			Start:  chapStart,
			Length: buf.Len() - chapStart,
		})

		// Emit sub-chapter chunks.
		for _, sub := range chap.subChapters {
			subStart := buf.Len()

			for _, chunk := range sub.chunks {
				// Sub-chapters share the parent's chapID in the skeleton/chunk
				// index but get their own sequential chunkID.
				inv := newInventory(b, sub.title, chapID, chunkID)
				head, tErr := runTemplate(*tpl, inv)
				if tErr != nil {
					return "", nil, nil, fmt.Errorf("chapter %d subchapter %q: %w",
						chapID, sub.title, tErr)
				}
				chunks = append(chunks, r.ChunkInfo{
					PreStart:      buf.Len(),
					PreLength:     len(head),
					ContentStart:  buf.Len() + len(head),
					ContentLength: len(chunk.Body),
				})
				buf.WriteString(head)
				buf.WriteString(chunk.Body)
				chunkID++
			}

			// NCX entry for sub-chapter — points into the assembled stream.
			// Depth=1 marks this as a level-2 NCX entry.
			chaps = append(chaps, r.ChapterInfo{
				Title:  sub.title,
				Start:  subStart,
				Length: buf.Len() - subStart,
				Depth:  1,
			})
		}
	}

	return buf.String(), chunks, chaps, nil
}

// textToRecords splits the assembled HTML string into 4096-byte PDB text
// records, optionally compressing each record with PalmDOC LZ77.
func textToRecords(html string, chapters []r.ChapterInfo, comp Compression) ([]r.TextRecord, error) {
	provider := r.NewTrailProvider(chapters)
	totalLen := len(html)

	recordCount := totalLen / r.TextRecordMaxSize
	if totalLen%r.TextRecordMaxSize != 0 {
		recordCount++
	}
	if totalLen == 0 {
		recordCount = 1 // always at least one record (caught by validate if truly empty)
	}

	records := make([]r.TextRecord, 0, recordCount)

	for i := range recordCount {
		from := i * r.TextRecordMaxSize
		to := min(from+r.TextRecordMaxSize, totalLen)

		raw := []byte(html[from:to])
		trail := provider.Get(from, to)

		switch comp {
		case CompressionPalmDoc:
			compressed, err := palmdocCompress(raw)
			if err != nil {
				return nil, fmt.Errorf("record %d: %w", i, err)
			}
			records = append(records, r.NewCompressedTextRecord(compressed, trail))
		default: // CompressionNone
			records = append(records, r.NewTextRecord(string(raw), trail))
		}
	}
	return records, nil
}

// inventory is the data passed to the KF8 skeleton HTML template for each chunk.
type inventory struct {
	Mobi    *Book
	Chapter struct {
		Title string
		ID    int
	}
	Chunk struct {
		ID int
	}
}

func newInventory(b *Book, chapTitle string, chapID, chunkID int) inventory {
	var inv inventory
	inv.Mobi = b
	inv.Chapter.Title = chapTitle
	inv.Chapter.ID = chapID
	inv.Chunk.ID = chunkID
	return inv
}

func runTemplate(tpl template.Template, v any) (string, error) {
	var buf strings.Builder
	if err := tpl.Execute(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}
