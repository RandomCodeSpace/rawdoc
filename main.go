package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var version = "0.1.0"

type config struct {
	// Output
	output   string
	format   string
	codeOnly bool
	noLinks  bool

	// Crawling
	depth       int
	concurrency int
	maxPages    int
	delay       time.Duration
	include     string
	exclude     string
	sitemap     bool

	// HTTP
	timeout    time.Duration
	maxTime    time.Duration
	maxRetries int
	headers    headerFlags
	noTLSSpoof bool
	noHeadless bool

	// Info
	verbose bool
	quiet   bool
}

type headerFlags []string

func (h *headerFlags) String() string { return strings.Join(*h, ", ") }
func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

var stderr = os.Stderr

func main() {
	cfg := &config{}

	flag.StringVar(&cfg.output, "o", "", "Output file or directory")
	flag.StringVar(&cfg.output, "output", "", "Output file or directory")
	flag.StringVar(&cfg.format, "f", "markdown", "Output format: markdown|text|json")
	flag.StringVar(&cfg.format, "format", "markdown", "Output format: markdown|text|json")
	flag.BoolVar(&cfg.codeOnly, "code-only", false, "Extract only code blocks")
	flag.BoolVar(&cfg.noLinks, "no-links", false, "Strip link URLs, keep text only")

	flag.IntVar(&cfg.depth, "d", 0, "Crawl depth, 0 = single page")
	flag.IntVar(&cfg.depth, "depth", 0, "Crawl depth, 0 = single page")
	flag.IntVar(&cfg.concurrency, "c", 5, "Parallel fetches")
	flag.IntVar(&cfg.concurrency, "concurrency", 5, "Parallel fetches")
	flag.IntVar(&cfg.maxPages, "max-pages", 50, "Page limit for crawling")
	flag.DurationVar(&cfg.delay, "delay", 1*time.Second, "Delay between requests")
	flag.StringVar(&cfg.include, "include", "", "URL path glob to include")
	flag.StringVar(&cfg.exclude, "exclude", "", "URL path glob to exclude")
	flag.BoolVar(&cfg.sitemap, "sitemap", false, "Parse sitemap.xml for URL discovery")

	flag.DurationVar(&cfg.timeout, "timeout", 15*time.Second, "Request timeout")
	flag.DurationVar(&cfg.maxTime, "max-time", 10*time.Minute, "Total runtime ceiling")
	flag.IntVar(&cfg.maxRetries, "max-retries", 3, "Per-URL retries")
	flag.Var(&cfg.headers, "header", "Extra header K=V (repeatable)")
	flag.BoolVar(&cfg.noTLSSpoof, "no-tls-spoof", false, "Disable utls fingerprint mimicry")
	flag.BoolVar(&cfg.noHeadless, "no-headless", false, "Disable Chrome fallback tier")

	flag.BoolVar(&cfg.verbose, "v", false, "Log fetch/tier decisions to stderr")
	flag.BoolVar(&cfg.verbose, "verbose", false, "Log fetch/tier decisions to stderr")
	flag.BoolVar(&cfg.quiet, "q", false, "Suppress all stderr output")
	flag.BoolVar(&cfg.quiet, "quiet", false, "Suppress all stderr output")

	showVersion := flag.Bool("version", false, "Print version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rawdoc <url> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Fetches web pages and converts them to clean markdown.\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Println("rawdoc " + version)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: URL required")
		flag.Usage()
		os.Exit(2)
	}

	rawURL := flag.Arg(0)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		fmt.Fprintf(os.Stderr, "error: invalid URL %q\n", flag.Arg(0))
		os.Exit(2)
	}

	// Validate format
	switch cfg.format {
	case "markdown", "text", "json":
	default:
		fmt.Fprintf(os.Stderr, "error: invalid format %q (use markdown, text, or json)\n", cfg.format)
		os.Exit(2)
	}

	if err := run(cfg, parsed); err != nil {
		if !cfg.quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(cfg *config, u *url.URL) error {
	if cfg.depth > 0 {
		return runCrawl(cfg, u)
	}
	return runSingle(cfg, u)
}

func runSingle(cfg *config, u *url.URL) error {
	opts := &fetchOptions{
		timeout:    cfg.timeout,
		maxRetries: cfg.maxRetries,
		verbose:    cfg.verbose,
		quiet:      cfg.quiet,
		noTLSSpoof: cfg.noTLSSpoof,
		noHeadless: cfg.noHeadless,
		headers:    []string(cfg.headers),
	}

	result, err := fetch(u.String(), opts)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.html))
	if err != nil {
		return fmt.Errorf("parse HTML: %w", err)
	}

	stripNoise(doc)

	content := extractContent(doc, u.Host)
	markdown := convertToMarkdown(content)

	title := strings.TrimSpace(doc.Find("title").Text())
	description, _ := doc.Find(`meta[name="description"]`).Attr("content")
	description = strings.TrimSpace(description)

	return writeOutput(cfg, result.url, title, description, markdown, result)
}

func writeOutput(cfg *config, pageURL, title, description, markdown string, result *fetchResult) error {
	var w io.Writer = os.Stdout

	if cfg.output != "" && cfg.depth == 0 {
		f, err := os.Create(cfg.output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	if cfg.codeOnly {
		return writeCodeOnly(w, markdown)
	}

	switch cfg.format {
	case "json":
		return writeJSON(w, pageURL, title, description, markdown, result)
	case "text":
		return writeText(w, markdown)
	default:
		return writeMarkdown(w, markdown)
	}
}

func writeMarkdown(w io.Writer, markdown string) error {
	_, err := fmt.Fprintln(w, markdown)
	return err
}

func writeText(w io.Writer, markdown string) error {
	// Strip bold/italic markers (** and *)
	text := strings.ReplaceAll(markdown, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	_, err := fmt.Fprintln(w, text)
	return err
}

type codeBlock struct {
	Lang string `json:"lang"`
	Code string `json:"code"`
}

func extractCodeBlocks(markdown string) []codeBlock {
	var blocks []codeBlock
	lines := strings.Split(markdown, "\n")
	var inBlock bool
	var lang string
	var buf strings.Builder

	for _, line := range lines {
		if !inBlock {
			if strings.HasPrefix(line, "```") {
				inBlock = true
				lang = strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				buf.Reset()
			}
		} else {
			if strings.HasPrefix(line, "```") {
				blocks = append(blocks, codeBlock{Lang: lang, Code: buf.String()})
				inBlock = false
				lang = ""
				buf.Reset()
			} else {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
	}
	return blocks
}

func writeJSON(w io.Writer, pageURL, title, description, markdown string, result *fetchResult) error {
	codeBlocks := extractCodeBlocks(markdown)

	type output struct {
		URL           string      `json:"url"`
		Title         string      `json:"title"`
		Description   string      `json:"description"`
		Content       string      `json:"content"`
		CodeBlocks    []codeBlock `json:"code_blocks"`
		FetchTier     int         `json:"fetch_tier"`
		FetchedAt     string      `json:"fetched_at"`
		ContentLength int         `json:"content_length"`
	}

	out := output{
		URL:           pageURL,
		Title:         title,
		Description:   description,
		Content:       markdown,
		CodeBlocks:    codeBlocks,
		FetchTier:     result.tier,
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
		ContentLength: len(markdown),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeCodeOnly(w io.Writer, markdown string) error {
	blocks := extractCodeBlocks(markdown)
	for i, b := range blocks {
		if i > 0 {
			fmt.Fprintln(w)
		}
		lang := b.Lang
		if lang == "" {
			lang = ""
		}
		fmt.Fprintf(w, "```%s\n%s```\n", lang, b.Code)
	}
	return nil
}

func runCrawl(cfg *config, u *url.URL) error {
	return fmt.Errorf("crawl mode not implemented yet")
}
