package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// MCP JSON-RPC types

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    mcpCaps    `json:"capabilities"`
	ServerInfo      mcpSrvInfo `json:"serverInfo"`
}

type mcpCaps struct {
	Tools *struct{} `json:"tools,omitempty"`
}

type mcpSrvInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Tool argument types

type fetchArgs struct {
	URL      string `json:"url"`
	Format   string `json:"format,omitempty"`
	CodeOnly bool   `json:"code_only,omitempty"`
}

type crawlArgs struct {
	URL         string `json:"url"`
	Depth       int    `json:"depth,omitempty"`
	MaxPages    int    `json:"max_pages,omitempty"`
	Include     string `json:"include,omitempty"`
	Exclude     string `json:"exclude,omitempty"`
	Concurrency int    `json:"concurrency,omitempty"`
}

var fetchSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "URL to fetch"},
    "format": {"type": "string", "enum": ["markdown", "text", "json", "yaml"], "default": "markdown"},
    "code_only": {"type": "boolean", "default": false, "description": "Extract only code blocks"}
  },
  "required": ["url"]
}`)

var crawlSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "Seed URL to crawl"},
    "depth": {"type": "integer", "default": 1, "description": "Crawl depth"},
    "max_pages": {"type": "integer", "default": 20, "description": "Max pages to fetch"},
    "include": {"type": "string", "description": "URL path glob to include"},
    "exclude": {"type": "string", "description": "URL path glob to exclude"},
    "concurrency": {"type": "integer", "default": 3}
  },
  "required": ["url"]
}`)

var mcpTools = []toolDef{
	{
		Name:        "rawdoc_fetch",
		Description: "Fetch a web page and return clean markdown. Strips nav, ads, scripts. 95%+ token reduction vs raw HTML.",
		InputSchema: fetchSchema,
	},
	{
		Name:        "rawdoc_crawl",
		Description: "Crawl linked pages from a URL by depth. Returns concatenated markdown for all pages.",
		InputSchema: crawlSchema,
	},
}

// serveMCP runs the MCP stdio server loop.
func serveMCP() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			mcpWrite(jsonRPCResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}})
			continue
		}

		switch req.Method {
		case "initialize":
			mcpWrite(jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: initResult{
					ProtocolVersion: "2024-11-05",
					Capabilities:    mcpCaps{Tools: &struct{}{}},
					ServerInfo:      mcpSrvInfo{Name: "rawdoc", Version: version},
				},
			})

		case "notifications/initialized":
			// No response for notifications

		case "tools/list":
			mcpWrite(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: toolsListResult{Tools: mcpTools}})

		case "tools/call":
			var params toolCallParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				mcpWrite(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "Invalid params"}})
				continue
			}
			mcpWrite(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: handleToolCall(params)})

		case "ping":
			mcpWrite(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})

		default:
			if req.ID != nil {
				mcpWrite(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "Method not found"}})
			}
		}
	}
}

func mcpWrite(resp jsonRPCResponse) {
	data, _ := json.Marshal(resp)
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

func handleToolCall(params toolCallParams) toolCallResult {
	switch params.Name {
	case "rawdoc_fetch":
		return handleFetch(params.Arguments)
	case "rawdoc_crawl":
		return handleCrawl(params.Arguments)
	default:
		return errResult("Unknown tool: " + params.Name)
	}
}

func handleFetch(raw json.RawMessage) toolCallResult {
	var args fetchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return errResult("Invalid arguments: " + err.Error())
	}
	if args.URL == "" {
		return errResult("url is required")
	}
	if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
		args.URL = "https://" + args.URL
	}
	parsed, err := url.Parse(args.URL)
	if err != nil || parsed.Host == "" {
		return errResult("Invalid URL: " + args.URL)
	}

	opts := &fetchOptions{timeout: 15 * time.Second, maxRetries: 3, quiet: true}
	result, err := fetch(parsed.String(), opts)
	if err != nil {
		return errResult("Fetch failed: " + err.Error())
	}

	doc, err := html.Parse(strings.NewReader(result.html))
	if err != nil {
		return errResult("Parse failed: " + err.Error())
	}

	stripNoise(doc)
	content := extractContent(doc, parsed.Host)
	markdown := optimizeMarkdown(convertToMarkdown(content))

	if args.CodeOnly {
		blocks := extractCodeBlocks(markdown)
		var buf strings.Builder
		for i, b := range blocks {
			if i > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString("```" + b.Lang + "\n" + b.Code + "```\n")
		}
		return toolCallResult{Content: []contentItem{{Type: "text", Text: buf.String()}}}
	}

	if args.Format == "json" {
		title := ""
		if tn := findFirst(doc, "title"); tn != nil {
			title = strings.TrimSpace(textContent(tn))
		}
		out := struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Content string `json:"content"`
			Tokens  int    `json:"estimated_tokens"`
		}{parsed.String(), title, markdown, estimateTokens(len(markdown))}
		data, _ := json.Marshal(out)
		return toolCallResult{Content: []contentItem{{Type: "text", Text: string(data)}}}
	}

	return toolCallResult{Content: []contentItem{{Type: "text", Text: markdown}}}
}

func handleCrawl(raw json.RawMessage) toolCallResult {
	var args crawlArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return errResult("Invalid arguments: " + err.Error())
	}
	if args.URL == "" {
		return errResult("url is required")
	}
	if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
		args.URL = "https://" + args.URL
	}
	parsed, err := url.Parse(args.URL)
	if err != nil || parsed.Host == "" {
		return errResult("Invalid URL: " + args.URL)
	}

	if args.Depth <= 0 {
		args.Depth = 1
	}
	if args.MaxPages <= 0 {
		args.MaxPages = 20
	}
	if args.Concurrency <= 0 {
		args.Concurrency = 3
	}

	cfg := &config{
		depth: args.Depth, maxPages: args.MaxPages, concurrency: args.Concurrency,
		delay: 1 * time.Second, include: args.Include, exclude: args.Exclude,
		timeout: 15 * time.Second, maxTime: 5 * time.Minute, maxRetries: 3, quiet: true,
	}

	results := crawlToResults(cfg, parsed)

	var buf strings.Builder
	pageCount := 0
	for _, r := range results {
		if r.err != nil {
			continue
		}
		pageCount++
		buf.WriteString("## " + r.title + "\n> " + r.url + "\n\n")
		buf.WriteString(r.markdown)
		buf.WriteString("\n\n---\n\n")
	}

	summary := fmt.Sprintf("[crawl] %d pages from %s (depth=%d)\n\n", pageCount, parsed.Host, args.Depth)
	return toolCallResult{Content: []contentItem{{Type: "text", Text: summary + buf.String()}}}
}

func errResult(msg string) toolCallResult {
	return toolCallResult{Content: []contentItem{{Type: "text", Text: msg}}, IsError: true}
}
