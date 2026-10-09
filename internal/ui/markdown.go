package ui

import (
	"bytes"
	"cmp"
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
	_, _ = w.WriteString(linkHTML(wr.res.resolve(n.(*wikilinkNode).target)))
	return ast.WalkContinue, nil
}

// linkHTML is the one anchor builder: a resolved link tints its underline by
// the target's status and names that status in a title; an unresolved one is a
// bare span.
func linkHTML(l link) string {
	text := html.EscapeString(l.label())
	if l.Plain {
		return `<span>` + text + `</span>`
	}
	if l.Href == "" {
		return `<span class="unresolved">` + text + `</span>`
	}
	a := `<a href="` + html.EscapeString(l.Href) + `"`
	if l.Hue != "" {
		a += ` class="to-` + html.EscapeString(l.Hue) + `" title="` + html.EscapeString(l.Status) + `"`
	}
	return a + `>` + text + `</a>`
}

// anchorHTML is linkHTML as a template func, so a slot value, a state-line ref
// and a cited-by entry print exactly what a body link prints.
func anchorHTML(l link) template.HTML {
	return template.HTML(linkHTML(l)) //nolint:gosec // linkHTML escapes every interpolated field
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
	titled    bool
}

func (n *sectionNode) Kind() ast.NodeKind         { return kindSection }
func (n *sectionNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// summaryNode holds the H2 of its section.
type summaryNode struct{ ast.BaseBlock }

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

func renderSummary(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<summary>")
	} else {
		_, _ = w.WriteString("</summary>\n")
	}
	return ast.WalkContinue, nil
}

// foldSections moves each H2 and the blocks after it, up to the next H2 or H1,
// into a sectionNode.
func foldSections(doc ast.Node, src []byte) {
	var sec *sectionNode
	var secs []*sectionNode
	var starts []int
	ids := map[string]int{}
	for c := doc.FirstChild(); c != nil; {
		next := c.NextSibling()
		if h, ok := c.(*ast.Heading); ok && h.Level == 1 {
			sec = nil
		} else if ok && h.Level == 2 {
			// An empty `##` has no line segment: it has no title, and its
			// section is counted from the block after it.
			title, start, titled := "", len(src), h.Lines().Len() > 0
			if titled {
				seg := h.Lines().At(0)
				title, start = strings.ReplaceAll(string(seg.Value(src)), "`", ""), seg.Start
			} else if n := firstLine(next); n >= 0 {
				start = n
			}
			sec = &sectionNode{title: title, id: slug(title, ids), titled: titled}
			secs, starts = append(secs, sec), append(starts, start)
			sum := &summaryNode{}
			doc.InsertBefore(doc, c, sec)
			doc.RemoveChild(doc, c)
			sum.AppendChild(sum, c)
			sec.AppendChild(sec, sum)
		} else if sec != nil {
			doc.RemoveChild(doc, c)
			sec.AppendChild(sec, c)
		}
		c = next
	}
	countLines(secs, starts, src)
}

// firstLine is the source offset where n's first line starts, or -1 when n or
// its descendants hold no line.
func firstLine(n ast.Node) int {
	at := -1
	if n != nil {
		_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering || c.Type() != ast.TypeBlock || c.Lines().Len() == 0 {
				return ast.WalkContinue, nil
			}
			at = c.Lines().At(0).Start
			return ast.WalkStop, nil
		})
	}
	return at
}

// countLines sets each section's non-blank line count, heading excluded.
func countLines(secs []*sectionNode, starts []int, src []byte) {
	for i, sec := range secs {
		end := len(src)
		if i+1 < len(secs) {
			end = max(starts[i], bytes.LastIndexByte(src[:starts[i+1]], '\n')+1)
		}
		for l := range bytes.SplitSeq(src[starts[i]:end], []byte("\n")) {
			if len(bytes.TrimSpace(l)) > 0 {
				sec.lines++
			}
		}
		if sec.titled {
			sec.lines-- // the heading line
		}
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
		if sec, ok := c.(*sectionNode); ok && sec.titled {
			if sent := lifted(res, sec); sent != "" {
				pb.Links = sent
				doc.RemoveChild(doc, c)
			} else {
				pb.Outline = append(pb.Outline, outlineItem{N: len(pb.Outline) + 1, Title: sec.title, ID: sec.id, Size: sizeOf(sec.lines, "line")})
			}
		}
		c = next
	}
	var err error
	pb.HTML, err = m.renderDoc(doc, src)
	return pb, err
}

// lifted is the Links sentence when sec is a `## Links` section of bare
// wikilinks, else empty: a section with prose or per-link notes stays in the body.
func lifted(res resolver, sec *sectionNode) template.HTML {
	if sec.title != "Links" {
		return ""
	}
	targets, ok := bareLinks(sec)
	if !ok {
		return ""
	}
	return linksSentence(res, targets)
}

// bareLinks returns sec's wikilink targets when sec holds only list items that
// are each one wikilink (alias allowed) and nothing else.
func bareLinks(sec *sectionNode) ([]string, bool) {
	var targets []string
	for c := sec.FirstChild().NextSibling(); c != nil; c = c.NextSibling() { // past the summary
		list, ok := c.(*ast.List)
		if !ok {
			return nil, false
		}
		for item := list.FirstChild(); item != nil; item = item.NextSibling() {
			blk := item.FirstChild()
			if blk == nil || blk.NextSibling() != nil || blk.ChildCount() != 1 {
				return nil, false
			}
			w, ok := blk.FirstChild().(*wikilinkNode)
			if !ok {
				return nil, false
			}
			targets = append(targets, w.target)
		}
	}
	return targets, len(targets) > 0
}

// linksSentence renders targets as one sentence of typed titles, one entry per
// distinct href: "decision <a>…</a>, issue <a>…</a> and …".
func linksSentence(res resolver, targets []string) template.HTML {
	var parts []string
	seen := map[string]bool{}
	for _, t := range targets {
		l := res.resolve(t)
		if id := cmp.Or(l.Href, l.Text); !seen[id] {
			seen[id] = true
			part := linkHTML(l)
			if l.Href != "" {
				part = l.TypeWord() + " " + part
			}
			parts = append(parts, part)
		}
	}
	last := len(parts) - 1
	if last < 1 {
		return template.HTML(strings.Join(parts, "") + ".") //nolint:gosec // parts are escaped by linkHTML
	}
	return template.HTML(strings.Join(parts[:last], ", ") + " and " + parts[last] + ".") //nolint:gosec // parts are escaped by linkHTML
}

func sizeOf(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
