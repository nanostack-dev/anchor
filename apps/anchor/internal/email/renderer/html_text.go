package renderer

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// skippedElements hold no text a reader sees: document metadata, CSS, code.
//
//nolint:gochecknoglobals // read-only lookup set
var skippedElements = map[string]bool{
	"head": true, "title": true, "style": true, "script": true,
	"noscript": true, "template": true,
}

// blockElements start and end a line in the text fallback.
//
//nolint:gochecknoglobals // read-only lookup set
var blockElements = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"center": true, "dd": true, "div": true, "dl": true, "dt": true,
	"figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hr": true, "li": true, "main": true, "nav": true,
	"ol": true, "p": true, "pre": true, "section": true, "table": true,
	"tbody": true, "thead": true, "tfoot": true, "tr": true, "ul": true,
}

// htmlToText renders an HTML body as the plain-text part a reader sees when
// a template has no body_text. It parses the HTML the way a browser does, so
// an omitted </li> or </p> still ends its item. It drops head, title, style
// and script content; puts each block and list item on its own line, with a
// "- " or "1. " marker; separates table cells with a space; keeps a link's
// URL after its text; and collapses runs of whitespace, keeping at most one
// blank line in a row.
func htmlToText(rendered string) string {
	root, err := html.Parse(strings.NewReader(rendered))
	if err != nil {
		return ""
	}
	w := &textWriter{}
	w.walk(root)
	w.endLine()

	out := make([]string, 0, len(w.lines))
	blank := false
	for _, line := range w.lines {
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

type textWriter struct {
	lines []string
	line  strings.Builder
	// space records whitespace seen since the last word on this line.
	space bool
}

// endLine closes the current line if it holds text.
func (w *textWriter) endLine() {
	if w.line.Len() > 0 {
		w.lines = append(w.lines, w.line.String())
		w.line.Reset()
	}
	w.space = false
}

// breakLine is a <br>: it ends the line even when empty, so <br><br> leaves
// a blank line.
func (w *textWriter) breakLine() {
	w.lines = append(w.lines, w.line.String())
	w.line.Reset()
	w.space = false
}

// text appends a run of text, collapsing HTML whitespace (not U+00A0, which
// an author writes to keep words together).
func (w *textWriter) text(s string) {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f':
			w.space = true
		default:
			if w.space && w.line.Len() > 0 {
				w.line.WriteByte(' ')
			}
			w.space = false
			w.line.WriteRune(r)
		}
	}
}

func (w *textWriter) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.text(n.Data)
		return
	case html.ElementNode:
		if skippedElements[n.Data] {
			return
		}
		switch n.Data {
		case "br":
			w.breakLine()
			return
		case "td", "th":
			w.space = true
		case "li":
			w.endLine()
			w.line.WriteString(listMarker(n))
		case "a":
			w.children(n)
			w.linkURL(n)
			return
		default:
			if blockElements[n.Data] {
				w.endLine()
			}
		}
		w.children(n)
		if blockElements[n.Data] {
			w.endLine()
		}
		return
	case html.ErrorNode, html.DocumentNode, html.CommentNode, html.DoctypeNode, html.RawNode:
	}
	w.children(n)
}

func (w *textWriter) children(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		w.walk(child)
	}
}

// linkURL writes a web link's URL after its text, so a reader of the text part
// can still follow it. A link whose text already is the URL, or a mailto, is
// left as written.
func (w *textWriter) linkURL(a *html.Node) {
	href := strings.TrimSpace(attr(a, "href"))
	if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
		return
	}
	if strings.Contains(w.line.String(), href) {
		return
	}
	w.text(" (" + href + ")")
}

// listMarker is "1. " for the nth item of an ordered list, "- " otherwise.
func listMarker(li *html.Node) string {
	parent := li.Parent
	if parent == nil || parent.Type != html.ElementNode || parent.Data != "ol" {
		return "- "
	}
	index := 1
	if start, err := strconv.Atoi(attr(parent, "start")); err == nil {
		index = start
	}
	for sibling := parent.FirstChild; sibling != nil && sibling != li; sibling = sibling.NextSibling {
		if sibling.Type == html.ElementNode && sibling.Data == "li" {
			index++
		}
	}
	return strconv.Itoa(index) + ". "
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
