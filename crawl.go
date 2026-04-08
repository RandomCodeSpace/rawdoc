package main

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// urlSet is a thread-safe set of visited URLs.
type urlSet struct {
	mu   sync.Mutex
	urls map[string]bool
}

func newURLSet() *urlSet {
	return &urlSet{urls: make(map[string]bool)}
}

// add adds u to the set after normalizing it. Returns true if the URL was new.
func (s *urlSet) add(u string) bool {
	norm := normalizeURL(u)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.urls[norm] {
		return false
	}
	s.urls[norm] = true
	return true
}

// normalizeURL strips the fragment, trailing slashes, and sorts query params.
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Strip fragment
	u.Fragment = ""
	// Sort query params
	if u.RawQuery != "" {
		params := u.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			vals := params[k]
			sort.Strings(vals)
			for _, v := range vals {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		u.RawQuery = strings.Join(parts, "&")
	}
	// Strip trailing slashes from path (but keep root "/" intact)
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}

// isSameDomain returns true if href is on the same domain as base.
// Relative URLs (starting with /) are always considered same domain.
func isSameDomain(href string, base *url.URL) bool {
	if strings.HasPrefix(href, "/") {
		return true
	}
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	stripWWW := func(h string) string {
		return strings.TrimPrefix(h, "www.")
	}
	return stripWWW(u.Hostname()) == stripWWW(base.Hostname())
}

// pathMatchesGlob returns true if urlPath matches the pattern (glob).
func pathMatchesGlob(urlPath, pattern string) bool {
	if matched, _ := path.Match(pattern, urlPath); matched {
		return true
	}
	// Also try matching just the last segment
	base := "/" + path.Base(urlPath)
	if matched, _ := path.Match(pattern, base); matched {
		return true
	}
	// Try matching each prefix segment against the pattern
	// e.g. "/docs/api/v2" should match "/docs/*"
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for i := 1; i <= len(parts); i++ {
		prefix := "/" + strings.Join(parts[:i], "/")
		if matched, _ := path.Match(pattern, prefix); matched {
			return true
		}
	}
	return false
}

// extractLinks finds all same-domain http/https links in the document.
func extractLinks(doc *goquery.Document, base *url.URL) []string {
	var links []string
	seen := make(map[string]bool)

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)
		// Skip special schemes and fragment-only links
		lower := strings.ToLower(href)
		if strings.HasPrefix(lower, "javascript:") ||
			strings.HasPrefix(lower, "mailto:") ||
			strings.HasPrefix(lower, "tel:") ||
			href == "#" ||
			strings.HasPrefix(href, "#") {
			return
		}

		resolved, err := base.Parse(href)
		if err != nil {
			return
		}

		// Only http/https
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}

		// Same domain only
		if !isSameDomain(href, base) {
			return
		}

		norm := normalizeURL(resolved.String())
		if !seen[norm] {
			seen[norm] = true
			links = append(links, resolved.String())
		}
	})
	return links
}

// crawlResult holds the result of fetching and converting a single page.
type crawlResult struct {
	url      string
	title    string
	markdown string
	err      error
}

type crawlItem struct {
	url   string
	depth int
}

// runCrawl performs a BFS crawl starting from seedURL.
func runCrawl(cfg *config, seedURL *url.URL) error {
	deadline := time.Now().Add(cfg.maxTime)

	seen := newURLSet()
	seen.add(seedURL.String())

	queue := []crawlItem{{url: seedURL.String(), depth: 0}}
	var results []crawlResult

	// Semaphore for concurrency control
	sem := make(chan struct{}, cfg.concurrency)

	var mu sync.Mutex
	failureCount := 0
	const maxFailures = 5

	opts := &fetchOptions{
		timeout:    cfg.timeout,
		maxRetries: cfg.maxRetries,
		verbose:    cfg.verbose,
		quiet:      cfg.quiet,
		headers:    []string(cfg.headers),
	}

	for len(queue) > 0 {
		// Check max-time deadline
		if time.Now().After(deadline) {
			if !cfg.quiet {
				fmt.Fprintf(stderr, "rawdoc: max-time reached, stopping crawl\n")
			}
			break
		}

		// Check max-pages limit
		mu.Lock()
		pageCount := len(results)
		mu.Unlock()
		if cfg.maxPages > 0 && pageCount >= cfg.maxPages {
			if !cfg.quiet {
				fmt.Fprintf(stderr, "rawdoc: max-pages (%d) reached, stopping crawl\n", cfg.maxPages)
			}
			break
		}

		// Check failure budget
		mu.Lock()
		failures := failureCount
		mu.Unlock()
		if failures >= maxFailures {
			if !cfg.quiet {
				fmt.Fprintf(stderr, "rawdoc: too many failures (%d), stopping crawl\n", failures)
			}
			break
		}

		item := queue[0]
		queue = queue[1:]

		// Apply delay between requests
		if cfg.delay > 0 && len(results) > 0 {
			time.Sleep(cfg.delay)
		}

		sem <- struct{}{}
		// Process this URL
		func() {
			defer func() { <-sem }()

			result, newLinks := fetchPage(item.url, opts, cfg)

			mu.Lock()
			defer mu.Unlock()

			if result.err != nil {
				failureCount++
				if !cfg.quiet {
					fmt.Fprintf(stderr, "rawdoc: error fetching %s: %v\n", item.url, result.err)
				}
				return
			}

			results = append(results, result)

			// Enqueue discovered links if within depth
			if cfg.depth == 0 || item.depth < cfg.depth {
				for _, link := range newLinks {
					if seen.add(link) {
						// Apply include/exclude filters
						u, err := url.Parse(link)
						if err != nil {
							continue
						}
						if cfg.include != "" && !pathMatchesGlob(u.Path, cfg.include) {
							continue
						}
						if cfg.exclude != "" && pathMatchesGlob(u.Path, cfg.exclude) {
							continue
						}
						queue = append(queue, crawlItem{url: link, depth: item.depth + 1})
					}
				}
			}
		}()
	}

	if len(results) == 0 {
		return fmt.Errorf("no pages crawled successfully")
	}

	if cfg.output != "" {
		return writeCrawlOutput(cfg, results)
	}

	// Write to stdout with separators
	for i, r := range results {
		if i > 0 {
			fmt.Print("\n---\n\n")
		}
		if r.title != "" {
			fmt.Printf("# %s\n\n", r.title)
		}
		fmt.Printf("<!-- URL: %s -->\n\n", r.url)
		fmt.Println(r.markdown)
	}
	return nil
}

// fetchPage fetches a URL, parses it, and returns a crawlResult and discovered links.
func fetchPage(rawURL string, opts *fetchOptions, cfg *config) (crawlResult, []string) {
	result, err := fetch(rawURL, opts)
	if err != nil {
		return crawlResult{url: rawURL, err: err}, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.html))
	if err != nil {
		return crawlResult{url: rawURL, err: fmt.Errorf("parse HTML: %w", err)}, nil
	}

	base, err := url.Parse(rawURL)
	if err != nil {
		base, _ = url.Parse(result.url)
	}

	links := extractLinks(doc, base)

	stripNoise(doc)
	content := extractContent(doc, base.Host)
	markdown := optimizeMarkdown(convertToMarkdown(content))
	title := strings.TrimSpace(doc.Find("title").Text())

	return crawlResult{
		url:      result.url,
		title:    title,
		markdown: markdown,
	}, links
}

// writeCrawlOutput writes crawl results to a directory.
func writeCrawlOutput(cfg *config, results []crawlResult) error {
	if err := os.MkdirAll(cfg.output, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	var indexLines []string
	indexLines = append(indexLines, "# Crawl Index\n")

	for _, r := range results {
		filename := urlToFilename(r.url) + ".md"
		filepath := cfg.output + "/" + filename

		var content strings.Builder
		if r.title != "" {
			content.WriteString("# " + r.title + "\n\n")
		}
		content.WriteString("<!-- URL: " + r.url + " -->\n\n")
		content.WriteString(r.markdown)
		content.WriteString("\n")

		if err := os.WriteFile(filepath, []byte(content.String()), 0644); err != nil {
			fmt.Fprintf(stderr, "rawdoc: error writing %s: %v\n", filepath, err)
			continue
		}

		indexLines = append(indexLines, fmt.Sprintf("- [%s](%s) — %s", r.title, filename, r.url))
	}

	indexContent := strings.Join(indexLines, "\n") + "\n"
	indexPath := cfg.output + "/index.md"
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("write index.md: %w", err)
	}

	if !cfg.quiet {
		fmt.Fprintf(stderr, "rawdoc: crawled %d pages → %s/\n", len(results), cfg.output)
	}
	return nil
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9\-_]+`)

// urlToFilename converts a URL to a safe filename (without extension).
func urlToFilename(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "index"
	}
	p := u.Path
	p = strings.Trim(p, "/")
	if p == "" {
		return "index"
	}
	p = strings.ToLower(p)
	p = strings.ReplaceAll(p, "/", "-")
	p = nonAlphanumRe.ReplaceAllString(p, "")
	p = strings.Trim(p, "-")
	if p == "" {
		return "index"
	}
	return p
}
