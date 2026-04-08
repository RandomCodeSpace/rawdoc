package main

import (
	"strings"
	"testing"
)

func TestStripScriptAndStyle(t *testing.T) {
	h := `<html><body>
		<script>alert("evil")</script>
		<style>body { color: red; }</style>
		<p>Keep this paragraph.</p>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	body := findFirst(doc, "body")
	bodyText := textContent(body)
	if strings.Contains(bodyText, "alert") {
		t.Error("script content should be removed")
	}
	if strings.Contains(bodyText, "color: red") {
		t.Error("style content should be removed")
	}
	if !strings.Contains(bodyText, "Keep this paragraph.") {
		t.Error("paragraph content should be kept")
	}
}

func TestStripNavFooter(t *testing.T) {
	h := `<html><body>
		<nav>Navigation links here</nav>
		<main><p>Main content here.</p></main>
		<footer>Footer text here</footer>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	if findFirst(doc, "nav") != nil {
		t.Error("nav should be removed")
	}
	if findFirst(doc, "footer") != nil {
		t.Error("footer should be removed")
	}
	mainNode := findFirst(doc, "main")
	if mainNode == nil {
		t.Error("main content should be kept")
	}
	if !strings.Contains(textContent(mainNode), "Main content here.") {
		t.Error("main content text should be kept")
	}
}

func TestStripNoiseByClass(t *testing.T) {
	h := `<html><body>
		<div class="cookie-consent">Accept cookies</div>
		<div class="sidebar">Sidebar links</div>
		<div class="newsletter">Subscribe now!</div>
		<article class="post">Real article content.</article>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	body := findFirst(doc, "body")
	bodyText := textContent(body)
	if strings.Contains(bodyText, "Accept cookies") {
		t.Error("cookie-consent element should be removed")
	}
	if strings.Contains(bodyText, "Sidebar links") {
		t.Error("sidebar element should be removed")
	}
	if strings.Contains(bodyText, "Subscribe now!") {
		t.Error("newsletter element should be removed")
	}
	if !strings.Contains(bodyText, "Real article content.") {
		t.Error("article content should be kept")
	}
}

func TestStripHiddenElements(t *testing.T) {
	h := `<html><body>
		<div style="display:none">Hidden div</div>
		<div style="display: none">Also hidden</div>
		<div aria-hidden="true">Aria hidden</div>
		<p>Visible paragraph.</p>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	body := findFirst(doc, "body")
	bodyText := textContent(body)
	if strings.Contains(bodyText, "Hidden div") {
		t.Error("display:none element should be removed")
	}
	if strings.Contains(bodyText, "Also hidden") {
		t.Error("display: none element should be removed")
	}
	if strings.Contains(bodyText, "Aria hidden") {
		t.Error("aria-hidden element should be removed")
	}
	if !strings.Contains(bodyText, "Visible paragraph.") {
		t.Error("visible paragraph should be kept")
	}
}

func TestStripHeaderWithoutH1(t *testing.T) {
	h := `<html><body>
		<header><nav>Site nav</nav></header>
		<main><p>Content here.</p></main>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	if findFirst(doc, "header") != nil {
		t.Error("header without h1 should be removed")
	}
	mainNode := findFirst(doc, "main")
	if !strings.Contains(textContent(mainNode), "Content here.") {
		t.Error("main content should be kept")
	}
}

func TestKeepHeaderWithH1(t *testing.T) {
	h := `<html><body>
		<header><h1>Page Title</h1></header>
		<main><p>Content here.</p></main>
	</body></html>`
	doc := parseHTML(h)
	stripNoise(doc)

	header := findFirst(doc, "header")
	if header == nil {
		t.Error("header with h1 should be kept")
	}
	if !strings.Contains(textContent(header), "Page Title") {
		t.Error("h1 in header should be kept")
	}
}

func TestExtractMainContent(t *testing.T) {
	h := `<html><body>
		<aside>Sidebar content</aside>
		<main><article><p>Main article content.</p></article></main>
	</body></html>`
	doc := parseHTML(h)
	sel := extractContent(doc, "example.com")

	text := strings.TrimSpace(textContent(sel))
	if !strings.Contains(text, "Main article content.") {
		t.Errorf("expected main article content, got: %s", text)
	}
	if strings.Contains(text, "Sidebar content") {
		t.Error("sidebar content should not be selected")
	}
}

func TestExtractArticleTag(t *testing.T) {
	h := `<html><body>
		<div class="wrapper">
			<article><p>Article body text.</p></article>
		</div>
	</body></html>`
	doc := parseHTML(h)
	sel := extractContent(doc, "example.com")

	text := textContent(sel)
	if !strings.Contains(text, "Article body text.") {
		t.Errorf("expected article text in selection, got: %s", text)
	}
}

func TestExtractFallsBackToBody(t *testing.T) {
	h := `<html><body>
		<div><p>Some content without semantic tags.</p></div>
	</body></html>`
	doc := parseHTML(h)

	// Use a domain with no site selector and HTML with no semantic tags
	sel := extractContent(doc, "no-semantic-site.example.com")

	if sel == nil {
		t.Fatal("expected a node to be returned")
	}
	if !strings.Contains(textContent(sel), "Some content without semantic tags.") {
		t.Errorf("expected body content, got: %s", textContent(sel))
	}
}
