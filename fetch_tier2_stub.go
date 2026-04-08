//go:build !tier2 && !all

package main

func fetchTier2(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	logVerbose(opts, "[tier2] disabled (build without -tags tier2)")
	return nil, &httpError{statusCode: 403, message: "tier2 disabled (build with: go build -tags tier2)"}
}
