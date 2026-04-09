package export

import (
	"bytes"
	"fmt"
	"html"

	"github.com/dingbo/dingovault/internal/parser"
	"github.com/yuin/goldmark"
)

// MarkdownFileToStandaloneHTML converts vault Markdown (optional YAML frontmatter stripped) to a full HTML document.
func MarkdownFileToStandaloneHTML(raw []byte, title string) ([]byte, error) {
	body := raw
	if _, b, ok := parser.SplitFrontmatter(raw); ok {
		body = b
	}
	var buf bytes.Buffer
	if err := goldmark.Convert(body, &buf); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	esc := html.EscapeString(title)
	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:42rem;margin:2rem auto;line-height:1.55;padding:0 1rem;}
pre,code{font-family:ui-monospace,monospace;}
pre{overflow:auto;padding:.75rem;background:#f4f4f5;border-radius:6px;}
@media (prefers-color-scheme: dark){
body{background:#111;color:#eee;}
pre{background:#1a1a1c;}
}
</style>
</head>
<body>
%s
</body>
</html>`, esc, buf.String())
	return []byte(doc), nil
}
