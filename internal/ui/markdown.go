package ui

import (
	"bytes"
	"html"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var kindWikilink = ast.NewNodeKind("Wikilink")

type wikilinkNode struct {
	ast.BaseInline
	target string
}

func (n *wikilinkNode) Kind() ast.NodeKind { return kindWikilink }

func (n *wikilinkNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"target": n.target}, nil)
}

// wikilinkParser claims `[[...]]` ahead of goldmark's link parser. Code spans
// and fenced blocks are consumed earlier, so they never reach it.
type wikilinkParser struct{}

func (wikilinkParser) Trigger() []byte { return []byte{'['} }

func (wikilinkParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("[[")) {
		return nil
	}
	end := bytes.Index(line, []byte("]]"))
	if end < 0 {
		return nil
	}
	inner := string(line[2:end])
	if strings.TrimSpace(inner) == "" || strings.ContainsAny(inner, "[]\n") {
		return nil
	}
	block.Advance(end + 2)
	return &wikilinkNode{target: inner}
}

type wikilinkRenderer struct{ res resolver }

func (wr wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikilink, wr.render)
}

func (wr wikilinkRenderer) render(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	l := wr.res.resolve(n.(*wikilinkNode).target)
	text := html.EscapeString(l.Text)
	if l.Href == "" {
		_, _ = w.WriteString(`<span class="unresolved">` + text + `</span>`)
	} else {
		_, _ = w.WriteString(`<a href="` + html.EscapeString(l.Href) + `">` + text + `</a>`)
	}
	return ast.WalkContinue, nil
}

// rawHTMLRenderer writes raw HTML source as escaped text, so prose like
// `<type>.<id>` stays visible instead of vanishing the way goldmark's default
// "raw HTML omitted" comment would hide it.
type rawHTMLRenderer struct{}

func (rawHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, renderRawHTML)
	reg.Register(ast.KindHTMLBlock, renderHTMLBlock)
}

func renderRawHTML(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	segs := n.(*ast.RawHTML).Segments
	for i := range segs.Len() {
		seg := segs.At(i)
		_, _ = w.WriteString(html.EscapeString(string(seg.Value(src))))
	}
	return ast.WalkSkipChildren, nil
}

func renderHTMLBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	b := n.(*ast.HTMLBlock)
	lines := b.Lines()
	_, _ = w.WriteString("<p>")
	for i := range lines.Len() {
		seg := lines.At(i)
		_, _ = w.WriteString(html.EscapeString(string(seg.Value(src))))
	}
	if b.HasClosure() {
		seg := b.ClosureLine
		_, _ = w.WriteString(html.EscapeString(string(seg.Value(src))))
	}
	_, _ = w.WriteString("</p>\n")
	return ast.WalkSkipChildren, nil
}

// markdown renders vault bodies. Raw HTML stays off (no html.WithUnsafe) and
// renders escaped, so a `<script>` in a body shows as text.
type markdown struct{ md goldmark.Markdown }

func newMarkdown(res resolver) markdown {
	return markdown{md: goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(wikilinkParser{}, 199))),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wikilinkRenderer{res}, 500), util.Prioritized(rawHTMLRenderer{}, 100))),
	)}
}

// render's output is safe to mark as template.HTML: goldmark escapes raw HTML
// and the wikilink renderer escapes its own text.
func (m markdown) render(body string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := m.md.Convert([]byte(body), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // raw HTML is off; see doc comment
}
