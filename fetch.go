package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	tls "github.com/refraction-networking/utls"
)

type fetchOptions struct {
	timeout    time.Duration
	maxRetries int
	verbose    bool
	quiet      bool
	noTLSSpoof bool
	noHeadless bool
	headers    []string
}

type fetchResult struct {
	html string
	tier int
	url  string
}

type httpError struct {
	statusCode int
	message    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, e.message)
}

var browserHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.5",
	"DNT":                       "1",
	"Connection":                "keep-alive",
	"Upgrade-Insecure-Requests": "1",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
}

func logVerbose(opts *fetchOptions, format string, args ...any) {
	if opts.verbose && !opts.quiet {
		fmt.Fprintf(stderr, format+"\n", args...)
	}
}

func fetch(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	logVerbose(opts, "[tier1] %s → fetching", rawURL)
	result, err := fetchTier1(rawURL, opts)
	if err == nil {
		return result, nil
	}

	if isEscalatable(err) && !opts.noTLSSpoof {
		logVerbose(opts, "[tier1] %s → %v, escalating to tier2", rawURL, err)
		result, err = fetchTier2(rawURL, opts)
		if err == nil {
			return result, nil
		}
	}

	if isEscalatable(err) && !opts.noHeadless {
		logVerbose(opts, "[tier2] %s → %v, escalating to tier3", rawURL, err)
		result, err = fetchTier3(rawURL, opts)
		if err == nil {
			return result, nil
		}
	}

	return nil, err
}

func isEscalatable(err error) bool {
	if err == nil {
		return false
	}
	if he, ok := err.(*httpError); ok {
		return he.statusCode == 403 || he.statusCode == 503
	}
	return false
}

func fetchTier1(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}

	timeout := opts.timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		Jar:     jar,
	}

	maxRetries := opts.maxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	var lastErr error
	backoff := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			logVerbose(opts, "[tier1] %s → retry %d/%d after %v", rawURL, attempt, maxRetries, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set browser headers
		for k, v := range browserHeaders {
			req.Header.Set(k, v)
		}

		// Set custom headers from opts
		for _, h := range opts.headers {
			parts := strings.SplitN(h, "=", 2)
			if len(parts) == 2 {
				req.Header.Set(parts[0], parts[1])
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// Network/timeout errors: retry if attempts remain
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			body, err := readBody(resp)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("reading body: %w", err)
			}
			return &fetchResult{html: body, tier: 1, url: rawURL}, nil

		case resp.StatusCode == 429:
			// Too Many Requests: retry with backoff
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode >= 500:
			// Server error: retry with backoff
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode == 403 || resp.StatusCode == 503:
			// Escalatable: return immediately, don't retry
			resp.Body.Close()
			return nil, &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}

		default:
			// Other client errors: return immediately
			resp.Body.Close()
			return nil, &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
		}
	}

	if lastErr == nil {
		lastErr = &httpError{statusCode: 0, message: "max retries exceeded"}
	}
	return nil, lastErr
}

func readBody(resp *http.Response) (string, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type utlsDialer struct{}

func (d *utlsDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("utlsDialer: split host/port: %w", err)
	}

	tcpConn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("utlsDialer: tcp dial: %w", err)
	}

	uconn := tls.UClient(tcpConn, &tls.Config{ServerName: host}, tls.HelloChrome_Auto)
	if err := uconn.HandshakeContext(ctx); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("utlsDialer: tls handshake: %w", err)
	}

	return uconn, nil
}

func fetchTier2(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}

	timeout := opts.timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	dialer := &utlsDialer{}
	transport := &http.Transport{
		DialTLSContext: dialer.DialTLSContext,
	}

	client := &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: transport,
	}

	maxRetries := opts.maxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	var lastErr error
	backoff := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			logVerbose(opts, "[tier2] %s → retry %d/%d after %v", rawURL, attempt, maxRetries, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set browser headers
		for k, v := range browserHeaders {
			req.Header.Set(k, v)
		}

		// Set custom headers from opts
		for _, h := range opts.headers {
			parts := strings.SplitN(h, "=", 2)
			if len(parts) == 2 {
				req.Header.Set(parts[0], parts[1])
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// Network/timeout errors: retry if attempts remain
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			body, err := readBody(resp)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("reading body: %w", err)
			}
			return &fetchResult{html: body, tier: 2, url: rawURL}, nil

		case resp.StatusCode == 429:
			// Too Many Requests: retry with backoff
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode >= 500:
			// Server error: retry with backoff
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode == 403 || resp.StatusCode == 503:
			// Escalatable: return immediately, don't retry
			resp.Body.Close()
			return nil, &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}

		default:
			// Other client errors: return immediately
			resp.Body.Close()
			return nil, &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
		}
	}

	if lastErr == nil {
		lastErr = &httpError{statusCode: 0, message: "max retries exceeded"}
	}
	return nil, lastErr
}

func fetchTier3(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	chromePath := findChrome()
	if chromePath == "" {
		logVerbose(opts, "[tier3] Chrome not found at any known path")
		return nil, &httpError{statusCode: 0, message: "Chrome not installed"}
	}

	logVerbose(opts, "[tier3] %s → using Chrome at %s", rawURL, chromePath)

	l := launcher.New().Bin(chromePath).Headless(true)
	controlURL, err := l.Launch()
	if err != nil {
		logVerbose(opts, "[tier3] %s → Chrome launch failed: %v", rawURL, err)
		return nil, fmt.Errorf("chrome launch: %w", err)
	}

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		logVerbose(opts, "[tier3] %s → browser connect failed: %v", rawURL, err)
		return nil, fmt.Errorf("browser connect: %w", err)
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		logVerbose(opts, "[tier3] %s → page open failed: %v", rawURL, err)
		return nil, fmt.Errorf("browser open page: %w", err)
	}

	if err := page.WaitStable(1 * time.Second); err != nil {
		logVerbose(opts, "[tier3] %s → page did not stabilize: %v", rawURL, err)
	}
	page.MustWaitLoad()

	html, err := page.HTML()
	if err != nil {
		logVerbose(opts, "[tier3] %s → failed to get HTML: %v", rawURL, err)
		return nil, fmt.Errorf("page html: %w", err)
	}

	if isCaptchaPage(html) {
		logVerbose(opts, "[tier3] %s → CAPTCHA detected", rawURL)
		return nil, &httpError{statusCode: 0, message: "site requires interactive challenge (CAPTCHA)"}
	}

	logVerbose(opts, "[tier3] %s → 200 (headless Chrome, %d bytes)", rawURL, len(html))
	return &fetchResult{html: html, tier: 3, url: rawURL}, nil
}

func findChrome() string {
	paths := []string{
		// Linux
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/snap/bin/chromium",
		// Mac
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		// Windows (native)
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		// Windows (via WSL)
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
	}

	// Also check CHROME_PATH env var (user override)
	if envPath := os.Getenv("CHROME_PATH"); envPath != "" {
		if fileExists(envPath) {
			return envPath
		}
	}

	for _, p := range paths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isCaptchaPage(html string) bool {
	lower := strings.ToLower(html)
	for _, marker := range []string{"turnstile", "cf-challenge", "captcha", "recaptcha", "hcaptcha", "challenge-platform"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
