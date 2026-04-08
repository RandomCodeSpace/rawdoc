package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

type fetchOptions struct {
	timeout    time.Duration
	maxRetries int
	verbose    bool
	quiet      bool
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

	if isEscalatable(err) && !opts.noHeadless {
		logVerbose(opts, "[tier1] %s → %v, escalating to tier2 (Chrome)", rawURL, err)
		result, err = fetchTier2(rawURL, opts)
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

		for k, v := range browserHeaders {
			req.Header.Set(k, v)
		}

		for _, h := range opts.headers {
			parts := strings.SplitN(h, "=", 2)
			if len(parts) == 2 {
				req.Header.Set(parts[0], parts[1])
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
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
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}
			continue

		case resp.StatusCode == 403 || resp.StatusCode == 503:
			resp.Body.Close()
			return nil, &httpError{statusCode: resp.StatusCode, message: http.StatusText(resp.StatusCode)}

		default:
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
