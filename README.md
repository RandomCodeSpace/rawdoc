# rawdoc

Fetch web pages as clean markdown for AI coding agents.

[![CI](https://github.com/RandomCodeSpace/rawdoc/actions/workflows/ci.yml/badge.svg)](https://github.com/RandomCodeSpace/rawdoc/actions/workflows/ci.yml)

Single Go binary. Fetches HTML, strips noise, outputs markdown. Supports single-page fetch and multi-page crawling by depth.

---

## Install

```bash
go install github.com/RandomCodeSpace/rawdoc@latest
```

---

## What It Does

1. **Fetches** HTML via plain HTTP with browser-like headers
2. **Strips** noise — scripts, styles, navbars, footers, ads, cookie banners, hidden elements
3. **Extracts** main content using site-specific selectors or readability scoring
4. **Converts** to clean markdown (headings, code blocks, tables, lists)
5. **Crawls** linked pages when given a depth > 0

Works on server-rendered sites. JS-only SPAs (React, Next.js) are not supported.

---

## Usage

```bash
# Single page → stdout
rawdoc https://kubernetes.io/docs/concepts/workloads/pods/

# Just the code blocks
rawdoc https://www.baeldung.com/spring-kafka --code-only

# JSON output with metadata
rawdoc https://pkg.go.dev/fmt -f json

# YAML output
rawdoc https://pkg.go.dev/fmt -f yaml

# Save to file
rawdoc https://example.com -o docs.md

# Crawl docs to a directory (depth=2, max 50 pages)
rawdoc https://kubernetes.io/docs/concepts/workloads/ -d 2 -o ~/docs/k8s/

# Verbose — see fetch decisions and token stats
rawdoc https://www.baeldung.com/spring-kafka -v
```

### Verbose Output

```
[tier1] https://pkg.go.dev/fmt → fetching
[stats] input: 139.2KB (35634 tokens) → output: 43.5KB (11135 tokens) | 69% saved
[output] wrote json to docs.json
```

All verbose output goes to stderr. stdout stays clean for piping.

---

## Flags

### Output

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | stdout | File or directory |
| `-f, --format` | `markdown` | `markdown` `text` `json` `yaml` |
| `--code-only` | — | Extract only code blocks |
| `--no-links` | — | Strip link URLs, keep text only |

### Crawling

| Flag | Default | Description |
|------|---------|-------------|
| `-d, --depth` | `0` | Crawl depth (0 = single page) |
| `-c, --concurrency` | `5` | Parallel fetches |
| `--max-pages` | `50` | Page limit |
| `--delay` | `1s` | Delay between requests |
| `--include` | — | URL path glob to include |
| `--exclude` | — | URL path glob to exclude |
| `--sitemap` | — | Parse sitemap.xml for URL discovery |

### HTTP

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout` | `15s` | Per-request timeout |
| `--max-time` | `10m` | Total runtime ceiling |
| `--max-retries` | `3` | Per-URL retries with exponential backoff |
| `--header K=V` | — | Extra header (repeatable) |

### Info

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Fetch log and token stats to stderr |
| `-q, --quiet` | Suppress all stderr |
| `--version` | Print version |

---

## Crawl Mode

```bash
rawdoc https://kubernetes.io/docs/concepts/workloads/ -d 2 --max-pages 50 -o ~/docs/k8s/
```

Writes one `.md` file per page plus an `index.md`:

```
~/docs/k8s/
├── index.md
├── workloads.md
├── workloads-pods.md
├── workloads-controllers-deployment.md
└── ...
```

Stays on the same domain. Respects `--include`/`--exclude` globs and `--max-pages` limit.

---

## Output Formats

| Format | Description |
|--------|-------------|
| `markdown` | Headings, code blocks, tables, lists (default) |
| `text` | Plain text, no markup |
| `json` | Structured: url, title, content, code_blocks, fetch_tier, token count |
| `yaml` | Same fields as JSON |
| `--code-only` | Only fenced code blocks from the page |

---

## Site-Specific Selectors

Built-in content selectors for: Baeldung, Docusaurus, GitBook, ReadTheDocs, MkDocs, Spring.io, GitHub, MDN, Go pkg.dev, StackOverflow, Medium, Dev.to, Confluence, Notion.

Falls back to readability scoring when no selector matches.

---

## Claude Code MCP Plugin

rawdoc runs as an MCP stdio server with `--serve`. Two tools are exposed:

- **`rawdoc_fetch`** — fetch a single page as markdown/json/text
- **`rawdoc_crawl`** — crawl linked pages by depth

### Install

```bash
# 1. Install the binary
go install github.com/RandomCodeSpace/rawdoc@latest

# 2. Add to Claude Code settings (~/.claude/settings.json)
```

```json
{
  "mcpServers": {
    "rawdoc": {
      "type": "stdio",
      "command": "rawdoc",
      "args": ["--serve"]
    }
  }
}
```

That's it. Restart Claude Code — `rawdoc_fetch` and `rawdoc_crawl` appear as tools.

### CLI Usage (without MCP)

```bash
rawdoc <url>              — fetch docs as markdown
rawdoc <url> --code-only  — code blocks only
rawdoc <url> -f json      — structured output
rawdoc <url> -d 2 -o dir/ — crawl to local directory
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Fetch failure |
| `2` | Usage error (bad flags, invalid URL) |

---

## Building

```bash
git clone https://github.com/RandomCodeSpace/rawdoc.git
cd rawdoc
go build -o rawdoc .
```

Cross-compile:

```bash
GOOS=linux   GOARCH=amd64 go build -o rawdoc-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -o rawdoc.exe .
GOOS=darwin  GOARCH=arm64 go build -o rawdoc-darwin-arm64 .
```

**Requires:** Go 1.24+

---

## Limitations

- **JS-rendered pages** (React SPAs, Next.js CSR, Angular) return empty content — rawdoc uses plain HTTP, not a browser
- **CAPTCHA/login-gated pages** — returns whatever the public page shows
- **Single IP** — not designed for large-scale scraping or proxy rotation
