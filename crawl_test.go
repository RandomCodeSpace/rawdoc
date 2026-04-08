package main

import (
	"net/url"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			"https://example.com/foo#bar",
			"https://example.com/foo",
		},
		{
			"https://example.com/foo/",
			"https://example.com/foo",
		},
		{
			"https://example.com/foo?b=2&a=1",
			"https://example.com/foo?a=1&b=2",
		},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsSameDomain(t *testing.T) {
	base, _ := url.Parse("https://www.baeldung.com/spring-boot")
	tests := []struct {
		href string
		want bool
	}{
		{"https://www.baeldung.com/spring-kafka", true},
		{"https://baeldung.com/spring-kafka", true},
		{"/spring-kafka", true},
		{"https://google.com", false},
	}
	for _, tt := range tests {
		got := isSameDomain(tt.href, base)
		if got != tt.want {
			t.Errorf("isSameDomain(%q, baeldung) = %v, want %v", tt.href, got, tt.want)
		}
	}
}

func TestPathGlob(t *testing.T) {
	tests := []struct {
		urlPath string
		pattern string
		want    bool
	}{
		{"/spring-kafka", "/spring-*", true},
		{"/spring-boot-start", "/spring-*", true},
		{"/java-collections", "/spring-*", false},
		{"/docs/api/v2", "/docs/*", true},
	}
	for _, tt := range tests {
		got := pathMatchesGlob(tt.urlPath, tt.pattern)
		if got != tt.want {
			t.Errorf("pathMatchesGlob(%q, %q) = %v, want %v", tt.urlPath, tt.pattern, got, tt.want)
		}
	}
}

func TestDedup(t *testing.T) {
	seen := newURLSet()

	if !seen.add("https://example.com/page1") {
		t.Error("first add should return true")
	}
	if seen.add("https://example.com/page1") {
		t.Error("duplicate add should return false")
	}
	if !seen.add("https://example.com/page2") {
		t.Error("different URL should return true")
	}
}

func TestExtractLinks(t *testing.T) {
	h := `<html><body>
		<a href="/page1">Page 1</a>
		<a href="https://example.com/page2">Page 2</a>
		<a href="https://other.com/nope">Other site</a>
		<a href="#fragment">Fragment only</a>
		<a href="javascript:void(0)">JS link</a>
	</body></html>`

	base, _ := url.Parse("https://example.com/start")
	doc := parseHTML(h)

	links := extractLinks(doc, base)

	// Build a set for easy lookup
	linkSet := make(map[string]bool)
	for _, l := range links {
		linkSet[l] = true
	}

	// Should include same-domain pages
	if !linkSet["https://example.com/page1"] {
		t.Errorf("expected https://example.com/page1 in links, got: %v", links)
	}
	if !linkSet["https://example.com/page2"] {
		t.Errorf("expected https://example.com/page2 in links, got: %v", links)
	}

	// Should exclude other domains
	for _, l := range links {
		u, _ := url.Parse(l)
		if u.Hostname() == "other.com" {
			t.Errorf("other.com link should be excluded, got: %s", l)
		}
	}

	// Should exclude fragment-only and javascript links
	for _, l := range links {
		if l == "#fragment" || l == "javascript:void(0)" {
			t.Errorf("fragment/javascript link should be excluded: %s", l)
		}
	}
}
