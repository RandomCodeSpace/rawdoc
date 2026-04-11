---
description: Fetch a web page as clean markdown
argument-hint: <url>
---

# rawdoc Command

Fetch the given URL and return clean markdown content. Use the `rawdoc_fetch` MCP tool.

## Instructions

1. Take the URL from `$ARGUMENTS`
2. Call the `rawdoc_fetch` tool with `{"url": "$ARGUMENTS"}`
3. Return the markdown content directly — do not summarize or modify it
4. If the fetch fails, report the error clearly

## Examples

- `/rawdoc https://kubernetes.io/docs/concepts/workloads/pods/` — fetch K8s pods docs
- `/rawdoc https://pkg.go.dev/fmt` — fetch Go fmt package docs
- `/rawdoc https://www.baeldung.com/spring-kafka` — fetch Baeldung article
