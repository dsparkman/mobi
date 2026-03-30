package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dsparkman/mobi"
)

// buildChapters splits rendered HTML into a []mobi.Chapter hierarchy
// according to the configured SplitLevel and SubSplitLevel.
//
// The algorithm:
//
//  1. If splitLevel == SplitNone: the entire HTML is one chapter.
//
//  2. Otherwise scan for <hN> tags at the split level.
//     Each occurrence starts a new chapter; its heading text becomes the
//     chapter title.
//
//  3. Within each chapter, if subSplitLevel != SplitNone, scan for <hM>
//     tags (M > N). Each occurrence starts a sub-chapter.
//
//  4. Any content before the first chapter heading is collected as a
//     preamble chapter named after the book title (cfg.title).
//
// Title extraction:
//   - If cfg.title is still empty after front-matter merging, the text of
//     the first H1 heading is used as the book title and that heading is
//     NOT repeated as a chapter title.
//
// Return values:
//   - chapters: the complete chapter slice ready for mobi.NewBook
//   - startChapterIdx: the 0-based index of the first "real" content chapter
//     (skipping any preamble chapter), for use with WithStartChapter. -1 if
//     there is only one chapter (no meaningful skip target).
func buildChapters(fullHTML string, cfg config) ([]mobi.Chapter, int, error) {
	if cfg.splitLevel == SplitNone {
		ch := mobi.SimpleChapter(cfg.title, wrapDiv(sanitizeForKindle(fullHTML, cfg)))
		return []mobi.Chapter{ch}, -1, nil
	}

	// ── Find chapter-level headings ──────────────────────────────────────────
	chapRe := headingRegex(int(cfg.splitLevel))
	chapMatches := chapRe.FindAllStringIndex(fullHTML, -1)

	if len(chapMatches) == 0 {
		// No chapter headings at all — one chapter with whole document.
		ch := mobi.SimpleChapter(titleOrUntitled(cfg.title), wrapDiv(sanitizeForKindle(fullHTML, cfg)))
		return []mobi.Chapter{ch}, -1, nil
	}

	var chapters []mobi.Chapter
	startIdx := -1 // index of first real chapter (past preamble)

	// ── Preamble: content before first heading ───────────────────────────────
	// safeSliceHTML verifies the cut point is on a rune boundary. In practice
	// it always is (the match is at '<', which is ASCII), but the check makes
	// that invariant explicit and catches any future regression.
	preambleRaw, err := safeSliceHTML(fullHTML, 0, chapMatches[0][0])
	if err != nil {
		return nil, -1, err
	}
	preamble := strings.TrimSpace(preambleRaw)
	if preamble != "" {
		pCh := mobi.SimpleChapter(titleOrUntitled(cfg.title), wrapDiv(sanitizeForKindle(preamble, cfg)))
		chapters = append(chapters, pCh)
	}

	// ── Title extraction from first H1 ───────────────────────────────────────
	// If cfg.title is still empty, grab it from the first heading we find.
	titleExtracted := false
	if cfg.title == "" {
		first := fullHTML[chapMatches[0][0]:chapEnd(chapMatches, 0, fullHTML)]
		cfg.title = headingText(first)
		titleExtracted = true
	}

	// ── Chapter loop ─────────────────────────────────────────────────────────
	for i, match := range chapMatches {
		start := match[0]
		end := chapEnd(chapMatches, i, fullHTML)

		fragment, err := safeSliceHTML(fullHTML, start, end)
		if err != nil {
			return nil, -1, err
		}

		title := headingText(fragment)
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}

		// Strip the chapter heading from the fragment body — the title is
		// recorded in the NCX; repeating it as an H1 wastes screen space.
		body := stripFirstHeading(fragment, int(cfg.splitLevel))

		// Record first real chapter index (after possible preamble).
		if startIdx < 0 && (preamble == "" || len(chapters) > 0) {
			startIdx = len(chapters)
		}

		// ── Sub-chapter splitting ────────────────────────────────────────────
		if cfg.subSplitLevel != SplitNone && int(cfg.subSplitLevel) > int(cfg.splitLevel) {
			cb, err := buildSubChapters(title, body, cfg)
			if err != nil {
				return nil, -1, err
			}
			// If the first chapter heading was the title source, don't
			// duplicate it as a chapter title when there's only that heading
			// and sub-chapters (edge case for single-H1 docs).
			if titleExtracted && i == 0 && len(chapMatches) == 1 && len(cb.subs) > 0 {
				// Title already captured; promote sub-chapters to top level
				// only when there are actual sub-chapters to promote. If there
				// are no sub-chapters, fall through to cb.build() so the intro
				// content is not silently discarded.
				chapters = append(chapters, cb.subChaptersAsTopLevel()...)
				continue
			}
			chapters = append(chapters, cb.build())
		} else {
			ch := mobi.SimpleChapter(title, wrapDiv(sanitizeForKindle(body, cfg)))
			chapters = append(chapters, ch)
		}
	}

	if startIdx < 0 {
		startIdx = 0
	}

	return chapters, startIdx, nil
}

// chapBuilder accumulates content for a chapter with sub-chapters.
type chapBuilder struct {
	title string
	intro string // content before the first sub-chapter heading
	subs  []subChapData
	cfg   config
}

type subChapData struct {
	title string
	body  string
}

func buildSubChapters(chapTitle, chapBody string, cfg config) (*chapBuilder, error) {
	cb := &chapBuilder{title: chapTitle, cfg: cfg}
	subRe := headingRegex(int(cfg.subSplitLevel))
	subMatches := subRe.FindAllStringIndex(chapBody, -1)

	if len(subMatches) == 0 {
		cb.intro = chapBody
		return cb, nil
	}

	cb.intro = chapBody[:subMatches[0][0]]

	for i, match := range subMatches {
		start := match[0]
		end := chapEnd(subMatches, i, chapBody)
		fragment := chapBody[start:end]
		subTitle := headingText(fragment)
		if subTitle == "" {
			subTitle = fmt.Sprintf("Section %d", i+1)
		}
		body := stripFirstHeading(fragment, int(cfg.subSplitLevel))
		cb.subs = append(cb.subs, subChapData{title: subTitle, body: body})
	}
	return cb, nil
}

func (cb *chapBuilder) build() mobi.Chapter {
	b := mobi.NewChapter(cb.title)
	introHTML := wrapDiv(sanitizeForKindle(cb.intro, cb.cfg))
	if strings.TrimSpace(introHTML) != "<div></div>" {
		b.AddContent(introHTML)
	} else {
		// Always need at least one content block for the chapter to be valid.
		b.AddContent("<div></div>")
	}
	for _, s := range cb.subs {
		b.AddSubChapter(s.title, wrapDiv(sanitizeForKindle(s.body, cb.cfg)))
	}
	return b.Build()
}

// subChaptersAsTopLevel promotes sub-chapters to full chapters. Used when a
// document has a single H1 (which became the title) with multiple H2s.
func (cb *chapBuilder) subChaptersAsTopLevel() []mobi.Chapter {
	out := make([]mobi.Chapter, 0, len(cb.subs))
	for _, s := range cb.subs {
		out = append(out, mobi.SimpleChapter(s.title, wrapDiv(sanitizeForKindle(s.body, cb.cfg))))
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// headingRegex matches an opening <hN> tag (N = level).
func headingRegex(level int) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`(?i)<h%d[\s>]`, level))
}

// chapEnd returns the end index of chapMatches[i] within src.
func chapEnd(matches [][]int, i int, src string) int {
	if i+1 < len(matches) {
		return matches[i+1][0]
	}
	return len(src)
}

// headingText extracts the plain text of the first heading in fragment.
var reHeading = regexp.MustCompile(`(?is)<h[1-6][^>]*>(.*?)</h[1-6]>`)
var reTag = regexp.MustCompile(`<[^>]+>`)

func headingText(fragment string) string {
	m := reHeading.FindStringSubmatch(fragment)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(reTag.ReplaceAllString(m[1], ""))
}

// stripFirstHeading removes the first <hN>...</hN> block from fragment.
func stripFirstHeading(fragment string, level int) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<h%d[^>]*>.*?</h%d>`, level, level))
	replaced := false
	return re.ReplaceAllStringFunc(fragment, func(s string) string {
		if replaced {
			return s
		}
		replaced = true
		return ""
	})
}

// wrapDiv wraps content in a <div> as required by the KF8 chunk structure.
func wrapDiv(content string) string {
	return "<div>" + content + "</div>"
}

func titleOrUntitled(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Untitled"
	}
	return title
}
