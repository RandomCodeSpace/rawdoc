---
description: Crawl linked pages from a URL by depth
argument-hint: <url> [depth]
---

# rawdoc-crawl Command

Crawl a documentation site starting from the given URL. Use the `rawdoc_crawl` MCP tool.

## Instructions

1. Parse `$ARGUMENTS` — first word is URL, optional second word is depth (default: 1)
2. Call `rawdoc_crawl` with `{"url": "<url>", "depth": <depth>}`
3. Return the concatenated markdown for all crawled pages
4. If the crawl fails, report the error clearly

## Examples

- `/rawdoc-crawl https://kubernetes.io/docs/concepts/workloads/` — crawl depth 1
- `/rawdoc-crawl https://helm.sh/docs/ 2` — crawl depth 2
