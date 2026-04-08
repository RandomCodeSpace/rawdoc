//go:build tier2 || all

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	tls "github.com/refraction-networking/utls"
)

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
		Timeout:   timeout,
		Jar:       jar,
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
			return &fetchResult{html: body, tier: 2, url: rawURL}, nil

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
