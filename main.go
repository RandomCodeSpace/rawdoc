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

	"golang.org/x/net/html"
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
	flag.StringVar(&cfg.format, "f", "markdown", "Output format: markdown|text|json|yaml")
	flag.StringVar(&cfg.format, "format", "markdown", "Output format: markdown|text|json|yaml")
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

	// Reorder os.Args so flags work regardless of position relative to URL.
	// Go's flag package stops parsing at the first non-flag argument.
	reorderArgs()

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
	case "markdown", "text", "json", "yaml":
	default:
		fmt.Fprintf(os.Stderr, "error: invalid format %q (use markdown, text, json, or yaml)\n", cfg.format)
		os.Exit(2)
	}

	if err := run(cfg, parsed); err != nil {
		if !cfg.quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

// reorderArgs moves the positional URL argument to the end of os.Args
// so that Go's flag package parses all flags regardless of position.
func reorderArgs() {
	var flags []string
	var positional []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Check if this flag takes a value (next arg doesn't start with -)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.Contains(arg, "=") {
				// Could be a boolean flag or a value flag — check if it's a known bool flag
				name := strings.TrimLeft(arg, "-")
				if !isBoolFlag(name) {
					i++
					flags = append(flags, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
	}
	os.Args = append([]string{os.Args[0]}, append(flags, positional...)...)
}

func isBoolFlag(name string) bool {
	boolFlags := map[string]bool{
		"code-only": true, "no-links": true, "sitemap": true,
		"v": true, "verbose": true, "q": true, "quiet": true,
		"version": true,
	}
	return boolFlags[name]
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
		headers:    []string(cfg.headers),
	}

	result, err := fetch(u.String(), opts)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	doc, err := html.Parse(strings.NewReader(result.html))
	if err != nil {
		return fmt.Errorf("parse HTML: %w", err)
	}

	rawHTMLSize := len(result.html)

	stripNoise(doc)

	content := extractContent(doc, u.Host)
	markdown := convertToMarkdown(content)
	markdown = optimizeMarkdown(markdown)

	title := ""
	if titleNode := findFirst(doc, "title"); titleNode != nil {
		title = strings.TrimSpace(textContent(titleNode))
	}
	description := ""
	var findMeta func(*html.Node)
	findMeta = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "meta" && getAttr(c, "name") == "description" {
				description = strings.TrimSpace(getAttr(c, "content"))
				return
			}
			findMeta(c)
		}
	}
	findMeta(doc)

	outputSize, err := writeOutput(cfg, result.url, title, description, markdown, result)
	if err != nil {
		return err
	}

	if cfg.verbose {
		rawTokens := estimateTokens(rawHTMLSize)
		outTokens := estimateTokens(outputSize)
		savings := 0
		if rawTokens > 0 {
			savings = 100 - (outTokens*100)/rawTokens
		}
		fmt.Fprintf(stderr, "[stats] input: %s (%d tokens) → output: %s (%d tokens) | %d%% saved\n",
			humanSize(rawHTMLSize), rawTokens, humanSize(outputSize), outTokens, savings)
		// Machine-parseable stats line for test scripts
		fmt.Fprintf(stderr, "[data] raw_bytes=%d out_bytes=%d raw_tokens=%d out_tokens=%d saved_pct=%d\n",
			rawHTMLSize, outputSize, rawTokens, outTokens, savings)
		if cfg.output != "" {
			fmt.Fprintf(stderr, "[output] wrote %s to %s\n", cfg.format, cfg.output)
		}
	}

	return nil
}

func writeOutput(cfg *config, pageURL, title, description, markdown string, result *fetchResult) (int, error) {
	var buf strings.Builder

	if cfg.codeOnly {
		if err := writeCodeOnly(&buf, markdown); err != nil {
			return 0, err
		}
	} else {
		switch cfg.format {
		case "json":
			if err := writeJSON(&buf, pageURL, title, description, markdown, result); err != nil {
				return 0, err
			}
		case "yaml":
			if err := writeYAML(&buf, pageURL, title, description, markdown, result); err != nil {
				return 0, err
			}
		case "text":
			if err := writeText(&buf, markdown); err != nil {
				return 0, err
			}
		default:
			if err := writeMarkdown(&buf, markdown); err != nil {
				return 0, err
			}
		}
	}

	output := buf.String()
	outputSize := len(output)

	var w io.Writer = os.Stdout
	if cfg.output != "" && cfg.depth == 0 {
		f, err := os.Create(cfg.output)
		if err != nil {
			return 0, fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	_, err := io.WriteString(w, output)
	return outputSize, err
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

func writeYAML(w io.Writer, pageURL, title, description, markdown string, result *fetchResult) error {
	codeBlocks := extractCodeBlocks(markdown)

	// Write YAML manually — avoids adding a yaml dependency for simple output
	fmt.Fprintf(w, "url: %s\n", yamlQuote(pageURL))
	fmt.Fprintf(w, "title: %s\n", yamlQuote(title))
	fmt.Fprintf(w, "description: %s\n", yamlQuote(description))
	fmt.Fprintf(w, "fetch_tier: %d\n", result.tier)
	fmt.Fprintf(w, "fetched_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(w, "content_length: %d\n", len(markdown))
	fmt.Fprintf(w, "content: |\n")
	for _, line := range strings.Split(markdown, "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if len(codeBlocks) > 0 {
		fmt.Fprintf(w, "code_blocks:\n")
		for _, b := range codeBlocks {
			fmt.Fprintf(w, "  - lang: %s\n", yamlQuote(b.Lang))
			fmt.Fprintf(w, "    code: |\n")
			for _, line := range strings.Split(b.Code, "\n") {
				fmt.Fprintf(w, "      %s\n", line)
			}
		}
	}
	return nil
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	// Quote if it contains special YAML chars
	for _, c := range s {
		if c == ':' || c == '#' || c == '\'' || c == '"' || c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == '&' || c == '*' || c == '!' || c == '|' || c == '>' || c == '%' || c == '@' || c == '`' {
			return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
		}
	}
	return s
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

func humanSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1fKB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.1fMB", mb)
}

