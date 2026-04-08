package main

import "strings"

// siteSelectors maps domain substrings to CSS selectors for site-specific content extraction.
var siteSelectors = map[string]string{
	"baeldung.com":          ".post-content, article.baeldung-article",
	"docusaurus.io":         ".theme-doc-markdown, article[role=\"main\"]",
	"gitbook.io":            ".page-body .page-inner",
	"readthedocs.io":        "[role=\"main\"], .rst-content",
	"readthedocs.org":       "[role=\"main\"], .rst-content",
	"mkdocs":                ".md-content",
	"spring.io":             ".content, main",
	"github.com":            ".markdown-body",
	"developer.mozilla.org": ".main-page-content, article",
	"pkg.go.dev":            ".Documentation",
	"stackoverflow.com":     ".answercell .js-post-body",
	"medium.com":            "article section",
	"dev.to":                ".crayons-article__body",
	"confluence":            ".wiki-content, #main-content",
	"notion.so":             ".notion-page-content",
}

// siteSelector returns the CSS selector for the given domain if a matching key is found.
func siteSelector(domain string) (string, bool) {
	for key, selector := range siteSelectors {
		if strings.Contains(domain, key) {
			return selector, true
		}
	}
	return "", false
}
