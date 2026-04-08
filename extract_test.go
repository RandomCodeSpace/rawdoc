package main

import (
	"strings"
	"testing"
)

func TestStripScriptAndStyle(t *testing.T) {
	html := `<html><body>
		<script>alert("evil")</script>
		<style>body { color: red; }</style>
		<p>Keep this paragraph.</p>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	body := doc.Find("body").Text()
	if strings.Contains(body, "alert") {
		t.Error("script content should be removed")
	}
	if strings.Contains(body, "color: red") {
		t.Error("style content should be removed")
	}
	if !strings.Contains(body, "Keep this paragraph.") {
		t.Error("paragraph content should be kept")
	}
}

func TestStripNavFooter(t *testing.T) {
	html := `<html><body>
		<nav>Navigation links here</nav>
		<main><p>Main content here.</p></main>
		<footer>Footer text here</footer>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	if doc.Find("nav").Length() != 0 {
		t.Error("nav should be removed")
	}
	if doc.Find("footer").Length() != 0 {
		t.Error("footer should be removed")
	}
	if doc.Find("main").Length() == 0 {
		t.Error("main content should be kept")
	}
	if !strings.Contains(doc.Find("main").Text(), "Main content here.") {
		t.Error("main content text should be kept")
	}
}

func TestStripNoiseByClass(t *testing.T) {
	html := `<html><body>
		<div class="cookie-consent">Accept cookies</div>
		<div class="sidebar">Sidebar links</div>
		<div class="newsletter">Subscribe now!</div>
		<article class="post">Real article content.</article>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	body := doc.Find("body").Text()
	if strings.Contains(body, "Accept cookies") {
		t.Error("cookie-consent element should be removed")
	}
	if strings.Contains(body, "Sidebar links") {
		t.Error("sidebar element should be removed")
	}
	if strings.Contains(body, "Subscribe now!") {
		t.Error("newsletter element should be removed")
	}
	if !strings.Contains(body, "Real article content.") {
		t.Error("article content should be kept")
	}
}

func TestStripHiddenElements(t *testing.T) {
	html := `<html><body>
		<div style="display:none">Hidden div</div>
		<div style="display: none">Also hidden</div>
		<div aria-hidden="true">Aria hidden</div>
		<p>Visible paragraph.</p>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	body := doc.Find("body").Text()
	if strings.Contains(body, "Hidden div") {
		t.Error("display:none element should be removed")
	}
	if strings.Contains(body, "Also hidden") {
		t.Error("display: none element should be removed")
	}
	if strings.Contains(body, "Aria hidden") {
		t.Error("aria-hidden element should be removed")
	}
	if !strings.Contains(body, "Visible paragraph.") {
		t.Error("visible paragraph should be kept")
	}
}

func TestStripHeaderWithoutH1(t *testing.T) {
	html := `<html><body>
		<header><nav>Site nav</nav></header>
		<main><p>Content here.</p></main>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	if doc.Find("header").Length() != 0 {
		t.Error("header without h1 should be removed")
	}
	if !strings.Contains(doc.Find("main").Text(), "Content here.") {
		t.Error("main content should be kept")
	}
}

func TestKeepHeaderWithH1(t *testing.T) {
	html := `<html><body>
		<header><h1>Page Title</h1></header>
		<main><p>Content here.</p></main>
	</body></html>`
	doc := docFromHTML(html)
	stripNoise(doc)

	if doc.Find("header").Length() == 0 {
		t.Error("header with h1 should be kept")
	}
	if !strings.Contains(doc.Find("header").Text(), "Page Title") {
		t.Error("h1 in header should be kept")
	}
}

func TestExtractMainContent(t *testing.T) {
	html := `<html><body>
		<aside>Sidebar content</aside>
		<main><article><p>Main article content.</p></article></main>
	</body></html>`
	doc := docFromHTML(html)
	sel := extractContent(doc, "example.com")

	text := strings.TrimSpace(sel.Text())
	if !strings.Contains(text, "Main article content.") {
		t.Errorf("expected main article content, got: %s", text)
	}
	if strings.Contains(text, "Sidebar content") {
		t.Error("sidebar content should not be selected")
	}
}

func TestExtractArticleTag(t *testing.T) {
	html := `<html><body>
		<div class="wrapper">
			<article><p>Article body text.</p></article>
		</div>
	</body></html>`
	doc := docFromHTML(html)
	sel := extractContent(doc, "example.com")

	if sel.Is("article") || sel.Find("article").Length() > 0 || strings.Contains(strings.TrimSpace(sel.Text()), "Article body text.") {
		// pass: article tag was selected or found in selection
	} else {
		t.Errorf("expected article content to be selected, got node: %s, text: %s", sel.Nodes[0].Data, sel.Text())
	}

	if !strings.Contains(sel.Text(), "Article body text.") {
		t.Errorf("expected article text in selection, got: %s", sel.Text())
	}
}

func TestExtractFallsBackToBody(t *testing.T) {
	html := `<html><body>
		<div><p>Some content without semantic tags.</p></div>
	</body></html>`
	doc := docFromHTML(html)

	// Use a domain with no site selector and HTML with no semantic tags
	sel := extractContent(doc, "no-semantic-site.example.com")

	if sel == nil || sel.Length() == 0 {
		t.Fatal("expected a selection to be returned")
	}
	if !strings.Contains(sel.Text(), "Some content without semantic tags.") {
		t.Errorf("expected body content, got: %s", sel.Text())
	}
}
