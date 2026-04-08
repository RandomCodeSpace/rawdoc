package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// convertToMarkdown is the main entry point: takes a goquery Selection and returns markdown.
func convertToMarkdown(sel *goquery.Selection) string {
	var buf strings.Builder
	convertNodes(&buf, sel, 0, false)
	return cleanMarkdown(buf.String())
}

// convertNodes recursively walks the node tree, writing markdown to buf.
func convertNodes(buf *strings.Builder, sel *goquery.Selection, depth int, inBlockquote bool) {
	sel.Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		if node == nil {
			return
		}
		switch node.Type {
		case html.TextNode:
			text := collapseWhitespace(node.Data)
			if text != "" && text != " " || (text == " " && buf.Len() > 0) {
				if inBlockquote {
					buf.WriteString(text)
				} else {
					buf.WriteString(text)
				}
			}
		case html.ElementNode:
			convertElement(buf, s, node.Data, depth, inBlockquote)
		}
	})
}

// convertElement handles a single HTML element.
func convertElement(buf *strings.Builder, s *goquery.Selection, tag string, depth int, inBlockquote bool) {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		prefix := strings.Repeat("#", level)
		buf.WriteString("\n\n")
		buf.WriteString(prefix)
		buf.WriteString(" ")
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("\n\n")

	case "p":
		buf.WriteString("\n\n")
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		text := strings.TrimSpace(inner.String())
		if inBlockquote {
			buf.WriteString("> ")
			buf.WriteString(text)
		} else {
			buf.WriteString(text)
		}
		buf.WriteString("\n\n")

	case "pre":
		// Look for code child
		code := s.Find("code")
		if code.Length() > 0 {
			lang := detectCodeLang(code)
			buf.WriteString("\n\n")
			buf.WriteString("```")
			buf.WriteString(lang)
			buf.WriteString("\n")
			buf.WriteString(code.Text())
			if !strings.HasSuffix(code.Text(), "\n") {
				buf.WriteString("\n")
			}
			buf.WriteString("```")
			buf.WriteString("\n\n")
		} else {
			buf.WriteString("\n\n```\n")
			buf.WriteString(s.Text())
			buf.WriteString("\n```\n\n")
		}

	case "code":
		// Check if parent is pre (handled there)
		parent := s.Parent()
		if parent.Length() > 0 && parent.Get(0).Data == "pre" {
			return
		}
		buf.WriteString("`")
		buf.WriteString(s.Text())
		buf.WriteString("`")

	case "ul", "ol":
		buf.WriteString("\n")
		s.Children().Each(func(i int, li *goquery.Selection) {
			if li.Get(0).Data == "li" {
				convertLI(buf, li, depth, tag == "ol", i+1)
			}
		})
		buf.WriteString("\n")

	case "li":
		// handled via convertLI when called from ul/ol
		convertLI(buf, s, depth, false, 1)

	case "a":
		href, exists := s.Attr("href")
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		text := strings.TrimSpace(inner.String())
		if exists && href != "" {
			buf.WriteString("[")
			buf.WriteString(text)
			buf.WriteString("](")
			buf.WriteString(href)
			buf.WriteString(")")
		} else {
			buf.WriteString(text)
		}

	case "img":
		alt, _ := s.Attr("alt")
		src, _ := s.Attr("src")
		buf.WriteString(fmt.Sprintf("![%s](%s)", alt, src))

	case "strong", "b":
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		text := inner.String()
		buf.WriteString("**")
		buf.WriteString(text)
		buf.WriteString("**")

	case "em", "i":
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		text := inner.String()
		buf.WriteString("*")
		buf.WriteString(text)
		buf.WriteString("*")

	case "table":
		convertTable(buf, s)

	case "blockquote":
		buf.WriteString("\n\n")
		var inner strings.Builder
		convertNodes(&inner, s, depth, true)
		innerText := strings.TrimSpace(inner.String())
		lines := strings.Split(innerText, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "> ") || line == ">" {
				buf.WriteString(line)
			} else {
				buf.WriteString("> ")
				buf.WriteString(line)
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")

	case "hr":
		buf.WriteString("\n\n---\n\n")

	case "br":
		buf.WriteString("\n")

	case "dl":
		convertDefList(buf, s)

	case "dt":
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		buf.WriteString("\n**")
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("**\n")

	case "dd":
		var inner strings.Builder
		convertNodes(&inner, s, depth, inBlockquote)
		buf.WriteString(strings.TrimSpace(inner.String()))
		buf.WriteString("\n")

	case "div", "section", "article", "main", "span", "figure", "figcaption":
		convertNodes(buf, s, depth, inBlockquote)

	case "thead", "tbody", "tfoot":
		// handled by convertTable
		convertNodes(buf, s, depth, inBlockquote)

	case "tr", "th", "td":
		// handled by convertTable
		convertNodes(buf, s, depth, inBlockquote)

	case "html", "head", "body":
		convertNodes(buf, s, depth, inBlockquote)

	case "script", "style", "noscript":
		// skip

	default:
		// For unknown elements, recurse into children
		convertNodes(buf, s, depth, inBlockquote)
	}
}

// convertLI handles a list item (li), supporting nested lists.
func convertLI(buf *strings.Builder, li *goquery.Selection, depth int, ordered bool, index int) {
	indent := strings.Repeat("  ", depth)

	// Collect text content (excluding nested lists)
	var textBuf strings.Builder
	li.Contents().Each(func(i int, child *goquery.Selection) {
		node := child.Get(0)
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			text := collapseWhitespace(node.Data)
			if text != "" {
				textBuf.WriteString(text)
			}
		} else if node.Type == html.ElementNode {
			switch node.Data {
			case "ul", "ol":
				// handled below
			default:
				var inner strings.Builder
				convertElement(&inner, child, node.Data, depth+1, false)
				textBuf.WriteString(inner.String())
			}
		}
	})

	text := strings.TrimSpace(textBuf.String())

	if ordered {
		buf.WriteString(fmt.Sprintf("%s%d. %s\n", indent, index, text))
	} else {
		buf.WriteString(fmt.Sprintf("%s- %s\n", indent, text))
	}

	// Now handle nested lists
	li.Children().Each(func(i int, child *goquery.Selection) {
		node := child.Get(0)
		if node == nil || node.Type != html.ElementNode {
			return
		}
		tag := node.Data
		if tag == "ul" || tag == "ol" {
			child.Children().Each(func(j int, nestedLI *goquery.Selection) {
				if nestedLI.Get(0).Data == "li" {
					convertLI(buf, nestedLI, depth+1, tag == "ol", j+1)
				}
			})
		}
	})
}

// convertTable renders a goquery table selection as a pipe table.
func convertTable(buf *strings.Builder, table *goquery.Selection) {
	buf.WriteString("\n\n")

	// Collect all rows
	var headers []string
	var rows [][]string

	// thead rows
	table.Find("thead tr").Each(func(i int, row *goquery.Selection) {
		var cols []string
		row.Find("th, td").Each(func(j int, cell *goquery.Selection) {
			cols = append(cols, strings.TrimSpace(cell.Text()))
		})
		if i == 0 {
			headers = cols
		}
	})

	// If no thead, use first tr's th/td as headers
	if len(headers) == 0 {
		table.Find("tr").First().Each(func(i int, row *goquery.Selection) {
			row.Find("th, td").Each(func(j int, cell *goquery.Selection) {
				headers = append(headers, strings.TrimSpace(cell.Text()))
			})
		})
	}

	// tbody rows
	table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		var cols []string
		row.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			cols = append(cols, strings.TrimSpace(cell.Text()))
		})
		rows = append(rows, cols)
	})

	// If no tbody, use all rows except first as data rows
	if len(rows) == 0 {
		first := true
		table.Find("tr").Each(func(i int, row *goquery.Selection) {
			if first {
				first = false
				return
			}
			var cols []string
			row.Find("td, th").Each(func(j int, cell *goquery.Selection) {
				cols = append(cols, strings.TrimSpace(cell.Text()))
			})
			rows = append(rows, cols)
		})
	}

	if len(headers) == 0 {
		return
	}

	// Write header row
	buf.WriteString("| ")
	buf.WriteString(strings.Join(headers, " | "))
	buf.WriteString(" |\n")

	// Write separator row
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	buf.WriteString("| ")
	buf.WriteString(strings.Join(seps, " | "))
	buf.WriteString(" |\n")

	// Write data rows
	for _, row := range rows {
		buf.WriteString("| ")
		buf.WriteString(strings.Join(row, " | "))
		buf.WriteString(" |\n")
	}

	buf.WriteString("\n")
}

// convertDefList renders a dl element.
func convertDefList(buf *strings.Builder, dl *goquery.Selection) {
	buf.WriteString("\n\n")
	dl.Children().Each(func(i int, child *goquery.Selection) {
		node := child.Get(0)
		if node == nil {
			return
		}
		switch node.Data {
		case "dt":
			buf.WriteString("\n**")
			buf.WriteString(strings.TrimSpace(child.Text()))
			buf.WriteString("**\n")
		case "dd":
			buf.WriteString(strings.TrimSpace(child.Text()))
			buf.WriteString("\n")
		}
	})
	buf.WriteString("\n")
}

// detectCodeLang extracts the language from a class like "language-go".
func detectCodeLang(code *goquery.Selection) string {
	class, exists := code.Attr("class")
	if !exists {
		return ""
	}
	for _, cls := range strings.Fields(class) {
		if strings.HasPrefix(cls, "language-") {
			return strings.TrimPrefix(cls, "language-")
		}
	}
	return ""
}

var wsRegexp = regexp.MustCompile(`[ \t]+`)

// collapseWhitespace collapses sequences of spaces/tabs into a single space.
func collapseWhitespace(s string) string {
	return wsRegexp.ReplaceAllString(s, " ")
}

var tripleNewlineRegexp = regexp.MustCompile(`\n{3,}`)

// cleanMarkdown collapses 3+ newlines to 2 and trims leading/trailing whitespace.
func cleanMarkdown(s string) string {
	s = tripleNewlineRegexp.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Patterns for output optimization
var (
	// [¶](#section-anchor) — pilcrow anchors used by Go docs, MDN, etc.
	pilcrowPattern = regexp.MustCompile(`\s*\[¶\]\([^)]*\)`)
	// [text](#fragment) — fragment-only self-links, keep just the text
	fragmentLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(#[^)]*\)`)
	// Lines that are only whitespace
	whitespaceLinePattern = regexp.MustCompile(`(?m)^[ \t]+$`)
	// 3+ consecutive blank lines → 2
	excessiveBlanks = regexp.MustCompile(`\n{3,}`)
	// Trailing whitespace on lines
	trailingWhitespace = regexp.MustCompile(`(?m)[ \t]+$`)
)

// optimizeMarkdown post-processes markdown to minimize token usage for AI agents.
func optimizeMarkdown(s string) string {
	// Strip pilcrow anchors: [¶](#...)
	s = pilcrowPattern.ReplaceAllString(s, "")

	// Convert fragment-only links to plain text: [text](#frag) → text
	s = fragmentLinkPattern.ReplaceAllString(s, "$1")

	// Strip trailing whitespace from lines
	s = trailingWhitespace.ReplaceAllString(s, "")

	// Collapse whitespace-only lines to empty lines
	s = whitespaceLinePattern.ReplaceAllString(s, "")

	// Collapse 3+ blank lines to 2
	s = excessiveBlanks.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

// estimateTokens gives a rough token count using the ~4 chars/token heuristic
// typical for GPT/Claude tokenizers on English text.
func estimateTokens(byteCount int) int {
	if byteCount == 0 {
		return 0
	}
	return (byteCount + 3) / 4
}
