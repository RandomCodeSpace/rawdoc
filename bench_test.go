package main

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func BenchmarkHTMLParse(b *testing.B) {
	data, _ := os.ReadFile("/tmp/bench-sample.html")
	s := string(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, _ := html.Parse(strings.NewReader(s))
		stripNoise(doc)
		content := extractContent(doc, "pkg.go.dev")
		md := convertToMarkdown(content)
		_ = optimizeMarkdown(md)
	}
}
