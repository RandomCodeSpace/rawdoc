package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
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
	// TODO: dispatch to single-page fetch or crawl based on cfg.depth
	fmt.Fprintf(os.Stderr, "rawdoc: fetching %s\n", u.String())
	return fmt.Errorf("not implemented")
}
