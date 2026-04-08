package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTier1FetchSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "Go-http-client/1.1" || ua == "" {
			t.Errorf("expected browser-like User-Agent, got %q", ua)
		}
		accept := r.Header.Get("Accept")
		if accept == "" {
			t.Error("expected Accept header to be sent")
		}
		fmt.Fprint(w, "<html><body>Hello</body></html>")
	}))
	defer ts.Close()

	opts := &fetchOptions{
		timeout:    5 * time.Second,
		maxRetries: 3,
	}
	result, err := fetchTier1(ts.URL, opts)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.tier != 1 {
		t.Errorf("expected tier 1, got %d", result.tier)
	}
	if result.html == "" {
		t.Error("expected non-empty html")
	}
}

func TestTier1FetchRetryOn5xx(t *testing.T) {
	var attempts int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "<html><body>OK</body></html>")
	}))
	defer ts.Close()

	opts := &fetchOptions{
		timeout:    5 * time.Second,
		maxRetries: 3,
	}
	result, err := fetchTier1(ts.URL, opts)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if result.tier != 1 {
		t.Errorf("expected tier 1, got %d", result.tier)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestTier1FetchTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		fmt.Fprint(w, "<html><body>Late</body></html>")
	}))
	defer ts.Close()

	opts := &fetchOptions{
		timeout:    100 * time.Millisecond,
		maxRetries: 1,
	}
	_, err := fetchTier1(ts.URL, opts)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetchCustomHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Header.Get("X-Custom")
		if val != "test-value" {
			t.Errorf("expected X-Custom=test-value, got %q", val)
		}
		fmt.Fprint(w, "<html><body>OK</body></html>")
	}))
	defer ts.Close()

	opts := &fetchOptions{
		timeout:    5 * time.Second,
		maxRetries: 3,
		headers:    []string{"X-Custom=test-value"},
	}
	_, err := fetchTier1(ts.URL, opts)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestTierEscalationOn403(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	opts := &fetchOptions{
		timeout:    5 * time.Second,
		maxRetries: 3,
		noTLSSpoof: true,
		noHeadless: true,
	}
	_, err := fetch(ts.URL, opts)
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
}

func TestTier3SkipsWhenNoChrome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer server.Close()

	opts := &fetchOptions{
		timeout:    5 * time.Second,
		maxRetries: 1,
		noTLSSpoof: true,
		noHeadless: false, // allow tier3, but Chrome likely not found in test env
	}
	_, err := fetch(server.URL, opts)
	if err == nil {
		t.Error("expected error")
	}
}
