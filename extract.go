package main

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
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

// stripNoise removes non-content elements from the document in-place.
func stripNoise(doc *goquery.Document) {
	// 1. Kill tags entirely (tag + contents)
	for _, tag := range killTags {
		doc.Find(tag).Remove()
	}

	// 2. Kill landmark tags
	for _, tag := range landmarkTags {
		doc.Find(tag).Remove()
	}

	// 3. Kill elements with noisy class/id
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		class, _ := s.Attr("class")
		id, _ := s.Attr("id")
		if noisePattern.MatchString(class) || noisePattern.MatchString(id) {
			s.Remove()
		}
	})

	// 4. Kill header elements that do NOT contain an h1
	doc.Find("header").Each(func(_ int, s *goquery.Selection) {
		if s.Find("h1").Length() == 0 {
			s.Remove()
		}
	})

	// 5. Kill hidden elements
	doc.Find("[aria-hidden='true']").Remove()
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		style, exists := s.Attr("style")
		if exists {
			normalized := strings.ReplaceAll(style, " ", "")
			if strings.Contains(normalized, "display:none") {
				s.Remove()
			}
		}
	})

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
	removeComments(doc.Get(0))
}

// extractContent picks the main content selection from the document.
func extractContent(doc *goquery.Document, domain string) *goquery.Selection {
	// 1. Site-specific selector
	if sel, ok := siteSelector(domain); ok {
		found := doc.Find(sel)
		if found.Length() > 0 {
			return found.First()
		}
	}

	// 2. Semantic tags
	for _, tag := range []string{"main", "article", `[role="main"]`} {
		found := doc.Find(tag)
		if found.Length() > 0 {
			return found.First()
		}
	}

	// 3. Readability scoring fallback
	var bestSel *goquery.Selection
	bestScore := -1
	doc.Find("div, section").Each(func(_ int, s *goquery.Selection) {
		score := readabilityScore(s)
		if score > bestScore {
			bestScore = score
			bestSel = s
		}
	})
	if bestSel != nil && bestScore > 0 {
		return bestSel
	}

	// 4. Fallback: body
	return doc.Find("body")
}

// readabilityScore scores a selection based on text length, structural elements, and noise signals.
func readabilityScore(s *goquery.Selection) int {
	// Noise check: score = 0 if element matches noise pattern
	class, _ := s.Attr("class")
	id, _ := s.Attr("id")
	if noisePattern.MatchString(class) || noisePattern.MatchString(id) {
		return 0
	}

	text := strings.TrimSpace(s.Text())
	score := len(text)

	// Bonus for structural elements
	score += s.Find("p").Length() * 50
	score += s.Find("pre").Length() * 100

	// Penalty: link-text-ratio > 0.5
	if len(text) > 0 {
		linkText := strings.TrimSpace(s.Find("a").Text())
		ratio := float64(len(linkText)) / float64(len(text))
		if ratio > 0.5 {
			score /= 3
		}
	}

	return score
}
