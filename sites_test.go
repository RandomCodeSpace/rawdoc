package main

import "testing"

func TestSiteSelector(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"www.baeldung.com", true},
		{"baeldung.com", true},
		{"developer.mozilla.org", true},
		{"pkg.go.dev", true},
		{"random-unknown-site.com", false},
	}

	for _, tt := range tests {
		_, got := siteSelector(tt.domain)
		if got != tt.want {
			t.Errorf("siteSelector(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}
