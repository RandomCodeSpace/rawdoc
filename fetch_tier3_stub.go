//go:build !tier3 && !all

package main

func fetchTier3(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	logVerbose(opts, "[tier3] disabled (build without -tags tier3)")
	return nil, &httpError{statusCode: 0, message: "tier3 disabled (build with: go build -tags tier3)"}
}
