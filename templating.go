package mobi

import (
	"text/template"

	r "github.com/dsparkman/mobi/records"
)

const defaultTemplateString = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title>{{ .Mobi.Title }}</title>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    {{- range $i, $_ := .Mobi.CSSFlows }}
    <link rel="stylesheet" type="text/css" href="kindle:flow:{{ $i | inc | base32 }}?mime=text/css"/>
    {{- end }}
  </head>
  <body aid="{{ .Chunk.ID | base32 }}">
  </body>
</html>`

var funcMap = template.FuncMap{
	"inc": func(i int) int {
		return i + 1
	},
	"base32": func(i int) string {
		return r.To32(i)
	},
}

var defaultTemplate = template.Must(template.New("default").Funcs(funcMap).Parse(defaultTemplateString))
