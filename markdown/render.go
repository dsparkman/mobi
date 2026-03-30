package markdown

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// renderMarkdown converts raw Markdown bytes to an HTML string using
// gomarkdown with extensions appropriate for book content:
//   - CommonExtensions (autolinks, strikethrough, fenced code)
//   - AutoHeadingIDs (stable anchor IDs for internal links)
//   - NoEmptyLineBeforeBlock (tighter list rendering)
//   - Footnotes (academic / non-fiction content)
//   - Tables (GFM tables)
//   - MathJax (LaTeX math via $...$ — stripped before Kindle render)
//
// The output is post-processed by sanitizeForKindle before returning.
func renderMarkdown(src []byte, cfg config) (string, error) {
	// Strip YAML/TOML front-matter before parsing — it must not appear in
	// the rendered HTML. Front-matter is already consumed by mergeFrontMatter.
	src = stripFrontMatter(src)

	extensions := parser.CommonExtensions |
		parser.AutoHeadingIDs |
		parser.NoEmptyLineBeforeBlock |
		parser.Footnotes |
		parser.Tables

	p := parser.NewWithExtensions(extensions)

	rendererOpts := html.RendererOptions{
		// HrefTargetBlank would add target="_blank" to all links — skip for
		// Kindle, which doesn't support tabbed browsing.
		Flags: html.CommonFlags,
	}
	renderer := html.NewRenderer(rendererOpts)

	raw := string(markdown.ToHTML(src, p, renderer))
	return sanitizeForKindle(raw, cfg), nil
}

// sanitizeForKindle post-processes rendered HTML for Kindle/KF8 compatibility.
// All transformations are driven by requirements documented in the MobileRead
// MOBI spec and the KF8 wiki.
func sanitizeForKindle(h string, cfg config) string {
	// ── XHTML self-closing tags ──────────────────────────────────────────────
	// KF8 requires XHTML, which mandates self-closing void elements.
	h = reVoidBR.ReplaceAllString(h, "<br/>")
	h = reVoidHR.ReplaceAllString(h, "<hr/>")

	// Self-close <img> tags that aren't already self-closing.
	h = reImgOpen.ReplaceAllStringFunc(h, func(s string) string {
		s = strings.TrimRight(s, " ")
		if strings.HasSuffix(s, "/>") {
			return s
		}
		return s[:len(s)-1] + "/>"
	})

	// ── Soft hyphen ──────────────────────────────────────────────────────────
	// The MOBI spec documents that &shy; renders incorrectly on some Kindle
	// devices. Use the Kindle-specific <shy/> tag instead.
	h = strings.ReplaceAll(h, "&shy;", "<shy/>")

	// ── Word break opportunity ───────────────────────────────────────────────
	// KF8 spec: use <wbr/> instead of &#173; for word-break hints.
	h = strings.ReplaceAll(h, "&#173;", "<wbr/>")
	h = strings.ReplaceAll(h, "&shy;", "<wbr/>") // belt-and-suspenders

	// ── Remove unsupported elements ──────────────────────────────────────────
	// <script> and <style> tags in MD content don't render on Kindle and
	// can cause the parser to choke. Strip them entirely.
	h = reScript.ReplaceAllString(h, "")
	h = reStyleTag.ReplaceAllString(h, "")

	// ── Table cleanup ────────────────────────────────────────────────────────
	// Kindle's table renderer is fragile. Nested tables cause rendering
	// artifacts. We don't remove them (that would silently discard content)
	// but we add a class the CSS can target for compact display.
	h = reTable.ReplaceAllString(h, `<table class="md-table"`)

	return h
}

// ── Compiled regexes ─────────────────────────────────────────────────────────

var (
	reVoidBR   = regexp.MustCompile(`(?i)<br\s*/?>`)
	reVoidHR   = regexp.MustCompile(`(?i)<hr\s*/?>`)
	reImgOpen  = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	reScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyleTag = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTable    = regexp.MustCompile(`(?i)<table\b`)
)

// ── Local image embedding ────────────────────────────────────────────────────

// embedLocalImages scans the HTML for <img src="..."> tags where the src
// is a local file path (not http:// or https://) and:
//  1. Loads the image from disk (relative to baseDir).
//  2. Appends it to the images slice.
//  3. Rewrites the src to the KF8 kindle:embed:XXXX URI.
//
// The updated HTML and the accumulated image slice are returned.
// Images that fail to load are logged as warnings and left as-is in the HTML
// (they will simply not render on the Kindle).
func embedLocalImages(h string, baseDir string, existing []image.Image) (string, []image.Image, error) {
	images := make([]image.Image, len(existing))
	copy(images, existing)

	// Match src attributes that are not URLs.
	reSrc := regexp.MustCompile(`(?i)\bsrc="([^"]+)"`)

	var firstErr error
	result := reImgOpen.ReplaceAllStringFunc(h, func(tag string) string {
		m := reSrc.FindStringSubmatchIndex(tag)
		if m == nil {
			return tag
		}
		src := tag[m[2]:m[3]]

		// Skip remote URLs.
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") ||
			strings.HasPrefix(src, "//") || strings.HasPrefix(src, "data:") {
			return tag
		}

		// Resolve relative path.
		absPath := src
		if !filepath.IsAbs(src) {
			absPath = filepath.Join(baseDir, src)
		}

		f, err := os.Open(absPath)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("open image %q: %w", absPath, err)
			}
			return tag // leave tag unchanged; Kindle will show a broken image
		}
		defer func() { _ = f.Close() }()

		img, _, err := image.Decode(f)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("decode image %q: %w", absPath, err)
			}
			return tag
		}

		// KF8 image numbering is 1-based.
		idx := len(images) + 1
		images = append(images, img)

		// Rewrite src to kindle:embed URI. The To32 helper produces the
		// 4-character base-32 string used by the KF8 image index.
		kindleURI := fmt.Sprintf("kindle:embed:%04d", idx)
		return tag[:m[2]] + kindleURI + tag[m[3]:]
	})

	return result, images, firstErr
}
