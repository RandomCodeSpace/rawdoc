package main

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// killTags are tags whose entire subtree should be removed.
var killTags = []string{
	"script", "style", "noscript", "iframe", "svg", "canvas", "video", "audio",
	"source", "embed", "object", "form", "input", "button", "select", "textarea",
	"option", "label", "fieldset",
}

// landmarkTags are structural tags that should be removed as noise.
var landmarkTags = []string{"nav", "footer", "aside"}

// noisePattern matches class/id values that indicate non-content elements.
var noisePattern = regexp.MustCompile(`(?i)(cookie|consent|gdpr|banner|popup|modal|overlay|newsletter|subscribe|social|share|sidebar|advertisement|ad-|promo|related-posts)`)

// removeNode detaches a node from its parent.
func removeNode(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

// stripNoise removes non-content elements from the document in-place.
func stripNoise(doc *html.Node) {
	// Build a set for fast lookup of kill tags
	killSet := make(map[string]bool, len(killTags))
	for _, tag := range killTags {
		killSet[tag] = true
	}
	landmarkSet := make(map[string]bool, len(landmarkTags))
	for _, tag := range landmarkTags {
		landmarkSet[tag] = true
	}

	// 1 & 2. Kill tags and landmark tags — collect then remove
	var toRemove []*html.Node
	var collectKill func(*html.Node)
	collectKill = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				if killSet[c.Data] || landmarkSet[c.Data] {
					toRemove = append(toRemove, c)
					continue // don't recurse into removed subtrees
				}
			}
			collectKill(c)
		}
	}
	collectKill(doc)
	for _, n := range toRemove {
		removeNode(n)
	}

	// 3. Kill elements with noisy class/id
	toRemove = toRemove[:0]
	var collectNoisy func(*html.Node)
	collectNoisy = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				class := getAttr(c, "class")
				id := getAttr(c, "id")
				if noisePattern.MatchString(class) || noisePattern.MatchString(id) {
					toRemove = append(toRemove, c)
					continue
				}
			}
			collectNoisy(c)
		}
	}
	collectNoisy(doc)
	for _, n := range toRemove {
		removeNode(n)
	}

	// 4. Kill header elements that do NOT contain an h1
	toRemove = toRemove[:0]
	var collectHeaders func(*html.Node)
	collectHeaders = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "header" {
				if findFirst(c, "h1") == nil {
					toRemove = append(toRemove, c)
					continue
				}
			}
			collectHeaders(c)
		}
	}
	collectHeaders(doc)
	for _, n := range toRemove {
		removeNode(n)
	}

	// 5. Kill hidden elements (aria-hidden=true and display:none)
	toRemove = toRemove[:0]
	var collectHidden func(*html.Node)
	collectHidden = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				if hasAttr(c, "aria-hidden", "true") {
					toRemove = append(toRemove, c)
					continue
				}
				style := getAttr(c, "style")
				if style != "" {
					normalized := strings.ReplaceAll(style, " ", "")
					if strings.Contains(normalized, "display:none") {
						toRemove = append(toRemove, c)
						continue
					}
				}
			}
			collectHidden(c)
		}
	}
	collectHidden(doc)
	for _, n := range toRemove {
		removeNode(n)
	}

	// 6. Remove HTML comments
	var removeComments func(*html.Node)
	removeComments = func(n *html.Node) {
		var next *html.Node
		for c := n.FirstChild; c != nil; c = next {
			next = c.NextSibling
			if c.Type == html.CommentNode {
				n.RemoveChild(c)
			} else {
				removeComments(c)
			}
		}
	}
	removeComments(doc)
}

// extractContent picks the main content node from the document.
func extractContent(doc *html.Node, domain string) *html.Node {
	// 1. Site-specific selector
	if sel, ok := siteSelector(domain); ok {
		if found := findBySelector(doc, sel); found != nil {
			return found
		}
	}

	// 2. Semantic tags
	for _, tag := range []string{"main", "article"} {
		if found := findFirst(doc, tag); found != nil {
			return found
		}
	}
	// Check [role="main"]
	if found := findByAttr(doc, "role", "main"); found != nil {
		return found
	}

	// 3. Readability scoring fallback
	var bestNode *html.Node
	bestScore := -1
	var scoreDivs func(*html.Node)
	scoreDivs = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "div" || c.Data == "section") {
				score := readabilityScore(c)
				if score > bestScore {
					bestScore = score
					bestNode = c
				}
			}
			scoreDivs(c)
		}
	}
	scoreDivs(doc)
	if bestNode != nil && bestScore > 0 {
		return bestNode
	}

	// 4. Fallback: body
	if body := findFirst(doc, "body"); body != nil {
		return body
	}
	return doc
}

// readabilityScore scores a node based on text length, structural elements, and noise signals.
func readabilityScore(n *html.Node) int {
	class := getAttr(n, "class")
	id := getAttr(n, "id")
	if noisePattern.MatchString(class) || noisePattern.MatchString(id) {
		return 0
	}

	text := strings.TrimSpace(textContent(n))
	score := len(text)

	// Bonus for structural elements
	score += len(findAll(n, "p")) * 50
	score += len(findAll(n, "pre")) * 100

	// Penalty: link-text-ratio > 0.5
	if len(text) > 0 {
		var linkTextBuf strings.Builder
		for _, a := range findAll(n, "a") {
			linkTextBuf.WriteString(textContent(a))
		}
		linkText := strings.TrimSpace(linkTextBuf.String())
		ratio := float64(len(linkText)) / float64(len(text))
		if ratio > 0.5 {
			score /= 3
		}
	}

	return score
}

// --- Simple CSS selector matching ---

// matchesSelector checks if a node matches a simple CSS selector.
// Supports: tagname, .classname, #idname, [attr="val"], tag.class, comma-separated.
func matchesSelector(n *html.Node, sel string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	// Handle comma-separated selectors
	if strings.Contains(sel, ",") {
		parts := strings.Split(sel, ",")
		for _, part := range parts {
			if matchesSingleSelector(n, strings.TrimSpace(part)) {
				return true
			}
		}
		return false
	}
	return matchesSingleSelector(n, sel)
}

func matchesSingleSelector(n *html.Node, sel string) bool {
	if sel == "" {
		return false
	}
	// [attr="val"]
	if strings.HasPrefix(sel, "[") && strings.HasSuffix(sel, "]") {
		inner := sel[1 : len(sel)-1]
		if eqIdx := strings.Index(inner, "="); eqIdx >= 0 {
			attrKey := inner[:eqIdx]
			attrVal := strings.Trim(inner[eqIdx+1:], `"'`)
			return getAttr(n, attrKey) == attrVal
		}
		// Just [attr] — check existence
		for _, a := range n.Attr {
			if a.Key == inner {
				return true
			}
		}
		return false
	}
	// #idname
	if strings.HasPrefix(sel, "#") {
		return getAttr(n, "id") == sel[1:]
	}
	// .classname (possibly tag.classname)
	if dotIdx := strings.Index(sel, "."); dotIdx >= 0 {
		tagPart := sel[:dotIdx]
		classPart := sel[dotIdx+1:]
		if tagPart != "" && n.Data != tagPart {
			return false
		}
		return hasClass(n, classPart)
	}
	// Plain tag name
	return n.Data == sel
}

// hasClass checks if the element has the given class (space-separated).
func hasClass(n *html.Node, className string) bool {
	class := getAttr(n, "class")
	for _, cls := range strings.Fields(class) {
		if cls == className {
			return true
		}
	}
	return false
}

// findBySelector finds the first element matching a CSS selector string.
func findBySelector(n *html.Node, sel string) *html.Node {
	var result *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if result != nil {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if result != nil {
				return
			}
			if c.Type == html.ElementNode && matchesSelector(c, sel) {
				result = c
				return
			}
			walk(c)
		}
	}
	walk(n)
	return result
}

// findByAttr finds the first element with the given attribute key=value.
func findByAttr(n *html.Node, key, val string) *html.Node {
	var result *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if result != nil {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if result != nil {
				return
			}
			if c.Type == html.ElementNode && getAttr(c, key) == val {
				result = c
				return
			}
			walk(c)
		}
	}
	walk(n)
	return result
}
