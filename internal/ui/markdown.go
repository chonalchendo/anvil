package ui

import (
	"bytes"
	"html"
	"html/template"
	"regexp"
	"strconv"
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
		_, _ = w.WriteString(`<a href="` + html.EscapeString(l.Href) + `"` + hueAttr(l) + `>` + text + `</a>`)
	}
	return ast.WalkContinue, nil
}

// hueAttr is the class attribute that tints a link's underline by its target's status.
func hueAttr(l link) string {
	if l.Hue == "" {
		return ""
	}
	return ` class="to-` + html.EscapeString(l.Hue) + `"`
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
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wikilinkRenderer{res}, 500), util.Prioritized(rawHTMLRenderer{}, 100), util.Prioritized(sectionRenderer{}, 500))),
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

var (
	kindSection = ast.NewNodeKind("Section")
	kindSummary = ast.NewNodeKind("SectionSummary")
)

// sectionNode groups an H2 and the blocks up to the next H2 or H1 as a fold.
// id anchors the outline; title and lines feed it.
type sectionNode struct {
	ast.BaseBlock
	id, title string
	lines     int
}

func (n *sectionNode) Kind() ast.NodeKind         { return kindSection }
func (n *sectionNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// summaryNode holds the H2 and the item count of its section.
type summaryNode struct {
	ast.BaseBlock
	items int
}

func (n *summaryNode) Kind() ast.NodeKind         { return kindSummary }
func (n *summaryNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

type sectionRenderer struct{}

func (sectionRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindSection, renderSection)
	reg.Register(kindSummary, renderSummary)
}

func renderSection(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<details class=\"section\" id=\"" + n.(*sectionNode).id + "\" open>\n")
	} else {
		_, _ = w.WriteString("</details>\n")
	}
	return ast.WalkContinue, nil
}

func renderSummary(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<summary>")
		return ast.WalkContinue, nil
	}
	if items := n.(*summaryNode).items; items > 0 {
		_, _ = w.WriteString(`<span class="count">` + strconv.Itoa(items) + "</span></summary>\n")
	} else {
		_, _ = w.WriteString("</summary>\n")
	}
	return ast.WalkContinue, nil
}

// foldSections moves each H2 and the blocks after it, up to the next H2 or H1,
// into a sectionNode. The count is the section's top-level list items.
func foldSections(doc ast.Node, src []byte) {
	var sec *sectionNode
	var sum *summaryNode
	var secs []*sectionNode
	var starts []int
	ids := map[string]int{}
	for c := doc.FirstChild(); c != nil; {
		next := c.NextSibling()
		if h, ok := c.(*ast.Heading); ok && h.Level == 1 {
			sec = nil
		} else if ok && h.Level == 2 {
			seg := h.Lines().At(0)
			title := strings.ReplaceAll(string(seg.Value(src)), "`", "")
			sec, sum = &sectionNode{title: title, id: slug(title, ids)}, &summaryNode{}
			secs, starts = append(secs, sec), append(starts, seg.Start)
			doc.InsertBefore(doc, c, sec)
			doc.RemoveChild(doc, c)
			sum.AppendChild(sum, c)
			sec.AppendChild(sec, sum)
		} else if sec != nil {
			if l, ok := c.(*ast.List); ok {
				sum.items += l.ChildCount()
			}
			doc.RemoveChild(doc, c)
			sec.AppendChild(sec, c)
		}
		c = next
	}
	for i, sec := range secs {
		end := len(src)
		if i+1 < len(secs) {
			end = bytes.LastIndexByte(src[:starts[i+1]], '\n') + 1
		}
		for l := range bytes.SplitSeq(src[starts[i]:end], []byte("\n")) {
			if len(bytes.TrimSpace(l)) > 0 {
				sec.lines++
			}
		}
		sec.lines-- // the heading line
	}
}

// slug makes a heading anchor, suffixing -2, -3 … when a title repeats.
func slug(title string, seen map[string]int) string {
	id := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if id == "" {
		id = "section"
	}
	seen[id]++
	if n := seen[id]; n > 1 {
		id += "-" + strconv.Itoa(n)
	}
	return id
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// renderSections is render with each H2 section folded as a details element.
func (m markdown) renderSections(body string) (template.HTML, error) {
	src := []byte(body)
	doc := m.md.Parser().Parse(text.NewReader(src))
	foldSections(doc, src)
	return m.renderDoc(doc, src)
}

func (m markdown) renderDoc(doc ast.Node, src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := m.md.Renderer().Render(&buf, src, doc); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // raw HTML is off; see render
}

// outlineItem is one H2 of the contents column.
type outlineItem struct {
	N        int
	Title    string
	ID, Size string
}

// pageBody is an artifact body split for the page: the sections, their
// outline, and the `## Links` section as one sentence.
type pageBody struct {
	HTML    template.HTML
	Outline []outlineItem
	Links   template.HTML
}

// renderPage folds the sections, lifting a `## Links` section that holds
// wikilinks out of the body into a sentence.
func (m markdown) renderPage(body string, res resolver) (pageBody, error) {
	src := []byte(body)
	doc := m.md.Parser().Parse(text.NewReader(src))
	foldSections(doc, src)
	var pb pageBody
	for c := doc.FirstChild(); c != nil; {
		next := c.NextSibling()
		if sec, ok := c.(*sectionNode); ok {
			if sent := linksSentence(res, sec); sec.title == "Links" && sent != "" {
				pb.Links = sent
				doc.RemoveChild(doc, c)
			} else {
				pb.Outline = append(pb.Outline, outlineItem{N: len(pb.Outline) + 1, Title: sec.title, ID: sec.id, Size: sizeOf(sec.lines)})
			}
		}
		c = next
	}
	var err error
	pb.HTML, err = m.renderDoc(doc, src)
	return pb, err
}

func sizeOf(lines int) string {
	if lines == 1 {
		return "1 line"
	}
	return strconv.Itoa(lines) + " lines"
}
