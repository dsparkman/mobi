module github.com/dsparkman/mobi

go 1.26

require (
	// Markdown parsing — CommonMark + tables + footnotes + autolinks
	github.com/gomarkdown/markdown v0.0.0-20240730141124-034f12af3bf6

	// High-quality image scaling for cover thumbnails (BiLinear interpolation)
	golang.org/x/image v0.15.0

	// BCP-47 language tag matching for MOBI locale field
	golang.org/x/text v0.14.0

	// YAML front-matter parsing
	gopkg.in/yaml.v3 v3.0.1
)
