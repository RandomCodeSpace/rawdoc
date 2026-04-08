# rawdoc

Fetch web pages as clean markdown for AI coding agents.

[![CI](https://github.com/RandomCodeSpace/rawdoc/actions/workflows/ci.yml/badge.svg)](https://github.com/RandomCodeSpace/rawdoc/actions/workflows/ci.yml)

Single Go binary. No runtime downloads, no external services, no AI, no search.

---

## Install

```bash
go install github.com/RandomCodeSpace/rawdoc@latest
```

That's it — single binary, no runtime deps.

---

## Quick Start

```bash
# Single page
rawdoc https://kubernetes.io/docs/concepts/workloads/pods/

# Just code blocks
rawdoc https://www.baeldung.com/spring-kafka --code-only

# JSON output
rawdoc https://pkg.go.dev/fmt -f json

# YAML output
rawdoc https://pkg.go.dev/fmt -f yaml

# Save to file
rawdoc https://example.com -o docs.md

# Crawl docs to directory
rawdoc https://kubernetes.io/docs/concepts/workloads/ -d 2 -o ~/docs/k8s/

# Verbose — see tier decisions and token stats
rawdoc https://www.baeldung.com/spring-kafka -v
```

---

## How It Works

Three-tier fetch strategy — auto-escalates until a clean response is obtained:

```
Tier 1: Plain HTTP (~50ms)     — works for most doc sites
Tier 2: TLS Spoofing (~100ms)  — bypasses basic Cloudflare
Tier 3: Headless Chrome (~2s)  — JS-rendered pages (needs Chrome installed)
```

Processing pipeline: **Fetch → Strip noise → Extract content → Convert to Markdown**

---

## CLI Reference

```
rawdoc [flags] <url>
```

### Output

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output PATH` | stdout | File or directory |
| `-f, --format string` | `markdown` | `markdown\|text\|json\|yaml` |
| `--code-only` | — | Extract only code blocks |
| `--no-links` | — | Strip link URLs, keep text only |

### Crawling

| Flag | Default | Description |
|------|---------|-------------|
| `-d, --depth int` | `0` | Crawl depth, 0 = single page |
| `-c, --concurrency int` | `5` | Parallel fetches |
| `--max-pages int` | `50` | Page limit |
| `--delay duration` | `1s` | Delay between requests |
| `--include string` | — | URL path glob to include |
| `--exclude string` | — | URL path glob to exclude |
| `--sitemap` | — | Parse sitemap.xml for URL discovery |

### HTTP

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout duration` | `15s` | Request timeout |
| `--max-time duration` | `10m` | Total runtime ceiling |
| `--max-retries int` | `3` | Per-URL retries |
| `--header K=V` | — | Extra header (repeatable) |
| `--no-tls-spoof` | — | Disable utls fingerprint mimicry |
| `--no-headless` | — | Disable Chrome fallback tier |

### Info

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Log fetch/tier decisions and token stats to stderr |
| `-q, --quiet` | Suppress all stderr output |
| `--version` | Print version |

---

## Output Formats

| Format | Description |
|--------|-------------|
| `markdown` | Clean markdown with headings, lists, code blocks (default) |
| `text` | Plain text, no markup |
| `json` | Structured JSON with metadata (url, title, content, stats) |
| `yaml` | Same as JSON but YAML-encoded |
| `--code-only` | Extracts only fenced code blocks from the page |

---

## Verbose Mode & Token Stats

```
[tier1] https://pkg.go.dev/fmt → fetching
[stats] input: 139.2KB (35634 tokens) → output: 43.5KB (11135 tokens) | 69% saved
[output] wrote json to docs.json
```

All verbose output goes to stderr, keeping stdout clean for piping.

---

## Crawl Mode

Set `-d` to a depth greater than 0 to crawl linked pages under the same origin path.

```bash
rawdoc https://kubernetes.io/docs/concepts/workloads/ -d 2 -o ~/docs/k8s/
```

Output directory structure mirrors the URL path:

```
~/docs/k8s/
├── index.md
├── pods/
│   └── index.md
├── deployments/
│   └── index.md
└── replicasets/
    └── index.md
```

Use `--sitemap` to seed the crawl from `sitemap.xml` instead of link-following.

---

## Site-Specific Selectors

rawdoc ships with content-extraction rules for popular doc platforms and sites, so boilerplate (navbars, footers, ads) is stripped automatically:

Baeldung, Docusaurus, GitBook, ReadTheDocs, MkDocs, Hugo, Spring.io, GitHub, MDN, Go pkg.dev, StackOverflow, Medium, Dev.to, Confluence, Notion

---

## Agent Integration

### Claude Code (`CLAUDE.md`)

```markdown
## Fetching Documentation

Use `rawdoc` to fetch external docs as markdown before answering questions about them:

```bash
rawdoc <url>                  # pipe to stdin or save with -o
rawdoc <url> -f json          # structured output with metadata
rawdoc <url> --code-only      # grab only code examples
```
```

### Any agent via shell

```bash
# Pipe directly into your agent
rawdoc https://pkg.go.dev/net/http | your-agent-cli

# Save first, reference later
rawdoc https://docs.example.com/api -o /tmp/api-docs.md
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Fetch failure (network error, all tiers exhausted) |
| `2` | Usage error (bad flags, invalid URL) |

---

## Building from Source

```bash
git clone https://github.com/RandomCodeSpace/rawdoc.git
cd rawdoc
go build -o rawdoc .
```

Cross-compile:

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o rawdoc-linux-amd64 .

# Windows (amd64)
GOOS=windows GOARCH=amd64 go build -o rawdoc-windows-amd64.exe .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o rawdoc-darwin-arm64 .
```

---

## Requirements

| Requirement | Notes |
|-------------|-------|
| Go 1.24+ | Required to build from source |
| Chrome / Chromium | Optional — only needed for Tier 3 (JS-rendered pages) |

## Windows Antivirus Note

Windows Defender may flag `rawdoc.exe` as a false positive. This happens because:

- Go statically links everything into one large binary (triggers heuristic scanners)
- `utls` dependency spoofs TLS fingerprints (anti-detection technique)
- `go-rod` automates headless Chrome (browser automation flagged by behavior analysis)

**The code is fully open source — inspect it yourself.** To work around this:

```powershell
# Option 1: Exclude the build directory
Add-MpPreference -ExclusionPath "D:\Development\rawdoc"

# Option 2: Build with stripped symbols (smaller, fewer flags)
go build -ldflags="-s -w" -o rawdoc.exe .
```
