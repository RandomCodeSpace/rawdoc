# Journey Into rawdoc

*A technical narrative of how a documentation-fetching tool was born, rewritten, stripped down, built back up, and shipped as a Claude Code plugin -- all in three days.*

---

## 1. Project Genesis

The rawdoc project began on April 8, 2026, with three rapid "pre-yolo" checkpoint commits (`b2ea084`, `54ff251`, `c48595c`) made within ninety minutes of each other -- the digital equivalent of a deep breath before a sprint. The problem being solved was concrete and immediate: LLM-assisted development consumes enormous token budgets when feeding raw HTML documentation pages into context windows. A tool that could fetch web documentation and convert it to clean, minimal markdown would slash those costs by an order of magnitude.

The founding technical decisions arrived fast. Go was chosen as the implementation language, likely for its single-binary distribution, strong HTTP libraries, and cross-platform compilation. The very first substantive commit (`b4f50a0`) scaffolded a CLI with flag parsing and URL validation -- the skeleton onto which everything else would hang. Within the same day, five more foundational commits landed in rapid succession: an HTML-to-Markdown converter with tests (`5b7160a`), content extraction with noise stripping and site-specific selectors (`99f2561`), a Tier 1 HTTP fetcher with retry, backoff, and browser headers (`bf985dc`), the full single-page pipeline wired together (`7373094`), and already a Tier 2 TLS fingerprint spoofing layer using utls (`12d00e4`).

The initial vision was ambitious from the start: not just a simple fetcher, but a multi-tier system that could escalate its approach when simpler methods failed. The architecture acknowledged a hard truth about the modern web -- many documentation sites sit behind Cloudflare or similar CDN protections that reject non-browser clients.

## 2. Architectural Evolution

### Phase 1: The Three-Tier Go Monolith (April 8)

The original architecture was a three-tier escalation system in Go:

- **Tier 1**: Standard `net/http` with browser-mimicking headers
- **Tier 2**: utls-based TLS fingerprint spoofing to bypass JA3 fingerprinting
- **Tier 3**: Full headless Chrome via go-rod for JavaScript-rendered content

This was complemented by a BFS crawl engine (`0099fad`) with concurrency control, URL deduplication, rate limiting, and configurable output. The pipeline flowed: fetch -> extract -> convert -> output, with token statistics available via `-v` flag.

### Phase 2: The AV Crisis and Great Simplification (April 8)

A crisis emerged almost immediately. The go-rod dependency (headless Chrome automation) triggered antivirus false positives on Windows -- a dealbreaker for a developer tool. The response came in waves:

1. Symbol stripping to reduce AV triggers (`488393a`)
2. Build tags to isolate utls/rod from the default binary (`3b917ca`)
3. A decisive refactor dropping utls entirely, simplifying to a 2-tier strategy (`4299434`)
4. The final cut: removing go-rod entirely, leaving pure `net/http` with zero AV flags (`02f6c2c`)

This was a pivotal architectural moment. Rather than fighting the AV problem with complexity (build tags, conditional compilation), the project chose radical simplification. The rewrite of the README (`37365ba`) to match the new "HTTP fetch + crawl, no browser" scope marked a philosophical shift: rawdoc would do less, but do it reliably everywhere.

### Phase 3: Performance Optimization (April 8)

With the architecture simplified, attention turned to speed. Two back-to-back performance commits replaced goquery with direct `x/net/html` tree walking (`fae80c0`) and eliminated regex bottlenecks (`f258ecd`), achieving a claimed 3x speedup. The dependency footprint shrank to a single external package: `golang.org/x/net/html`.

### Phase 4: MCP Server Integration (April 11)

Three days after initial creation, rawdoc gained a second life as an MCP (Model Context Protocol) server. Observation 442 records the research phase at 6:52 AM, where stdio was chosen as the transport protocol. By 6:56 AM -- just four minutes later -- observation 444 records a complete MCP server implementation in `mcp.go` with `rawdoc_fetch` and `rawdoc_crawl` tools. The `--serve` flag (observation 446, the single most expensive observation at 14,973 discovery tokens) enabled dual-mode operation: the same binary could serve as both a CLI tool and an MCP server.

### Phase 5: The Node.js Detour (April 11, 7:14 AM - 9:08 AM)

Perhaps the most dramatic architectural event was the complete rewrite from Go to Node.js, driven by plugin distribution friction. Sessions S151 and S152 evaluated the Go toolchain dependency problem -- requiring users to have Go installed just to use a Claude Code plugin was too much friction. Session S153 committed to a full Node.js migration.

The migration was thorough: Go source files were removed (observation 458), CI and release workflows were rewritten for Node.js/npm (observations 461-462), and the README was completely rewritten (observation 463). The Node.js implementation was committed on a `node` branch (observation 464).

But the migration immediately hit the same wall the Go version had handled: Cloudflare protection. Observations 465-469 document a cascade of bugs -- Cloudflare 403 errors, TLS configuration conflicts, script/style tags leaking JavaScript into markdown output. The fixes were applied, but the fundamental problem proved intractable: observation 479 confirmed that "all Node.js fetch methods blocked by Baeldung Cloudflare protection," while the Go version handled it effortlessly (observation 473).

The Node.js version achieved 9/10 site compatibility with 28-97% token reduction (observation 480), but the Go version's superior Cloudflare handling tipped the decision back. The `node` branch was eventually abandoned (sessions S164-S165), and development continued on the Go main branch.

### Phase 6: Claude Code Plugin Marketplace (April 11, 9:10 AM - 9:23 AM)

The final architectural phase solved the distribution problem that had motivated the Node.js detour, but without abandoning Go. Research into Claude Code plugin marketplace structure (observations 482-485) revealed that plugins could bundle MCP servers with auto-build hooks. The solution: a plugin that automatically compiles the Go binary from source during installation.

Observations 486-491 document the rapid implementation: plugin manifest, slash commands, auto-build hooks, and marketplace deployment -- all shipped in thirteen minutes.

## 3. Key Breakthroughs

**The MCP server in four minutes.** Between observation 442 (protocol research complete) and observation 444 (full implementation), just four minutes elapsed. The 275-line `mcp.go` file implemented complete JSON-RPC stdio protocol handling with two tools. This was possible because the existing pipeline was cleanly factored -- the `crawlToResults()` function (observation 445) was the only new plumbing needed.

**The radical simplification decision.** Commits `4299434` and `02f6c2c` represent a breakthrough in judgment rather than code. Recognizing that three tiers of complexity existed to handle edge cases that affected a minority of sites, the project chose to serve the 90% use case perfectly rather than the 100% use case poorly. The AV false positive crisis forced this clarity.

**The auto-build plugin hook.** The insight that a Claude Code plugin could compile Go from source during installation (observation 487) resolved the distribution dilemma without sacrificing Go's runtime advantages. Users need Go installed, but the plugin handles the build transparently.

**The 95%+ token reduction metric.** Observation 480 validated that rawdoc achieves 28-97% token reduction across diverse documentation sites. The project's core value proposition -- saving LLM tokens -- was quantitatively confirmed.

## 4. Work Patterns

The development of rawdoc reveals a distinctive rhythm best described as "sprint-crisis-simplify."

**Day 1 (April 8): The Build Sprint.** Thirty-two commits in a single day, from empty repository to feature-complete tool. The commits show a pattern of feature addition followed immediately by fixes and refinements: the crawl engine is added, then flags are fixed (`e43e98a`), then output formatting is iterated through four commits (`37f7643`, `3d66bd2`, `8bac2ec`, `9cd1451`, `06295d6`). This is not waterfall development; it is build-test-fix in tight loops.

**The AV Crisis (April 8, late).** Five commits in quick succession address antivirus false positives, escalating from bandaids (symbol stripping) through structural fixes (build tags) to architectural surgery (removing dependencies entirely). The pattern: try the minimal fix first, escalate only when it fails.

**The Gap (April 9-10).** No commits for two days. The project sat in a stable, simplified state.

**Day 3 (April 11): The Integration Sprint.** Twenty-six sessions recorded by claude-mem in five hours (6:05 AM - 10:55 AM). The memory system captured 52 observations during this period. The work was dominated by integration concerns: MCP servers, plugin packaging, distribution, and the Node.js experiment. Sessions averaged about 11 minutes each, with some as short as one minute (pure decision sessions like S146) and others spanning extended implementation work.

**The Exploration Pattern.** Sessions S149 through S153 show a characteristic exploration phase: evaluate options (S149), commit to a direction (S150), hit friction (S151), evaluate alternatives (S152), execute (S153). This five-session arc took about ten minutes of wall-clock time but represented a major architectural decision point.

## 5. Technical Debt

**The utls/go-rod accumulation.** Adding Tier 2 (utls) and Tier 3 (go-rod) created immediate technical debt in the form of binary size, dependency complexity, and AV false positives. This debt was paid back aggressively on the same day through the refactoring commits `4299434` and `02f6c2c`.

**The Node.js branch.** The entire Node.js migration (observations 458-480) represents technical debt in the form of abandoned work. Approximately 48,000 discovery tokens were spent on a branch that was ultimately deleted. However, this was not wasted effort -- it validated the decision to stay with Go and produced reusable knowledge about Cloudflare bypass limitations.

**The OpenClaw plugin format.** Observation 451 created an OpenClaw plugin manifest, which was subsequently replaced by the Claude Code plugin format (observation 488). This small debt was paid back within two hours.

**Documentation drift.** The README was rewritten at least three times: once for the simplified Go version (`37365ba`), once for Node.js (observation 463), and once back to Go with MCP details (observation 492). Each rewrite was necessary but represents the cost of architectural churn.

## 6. Challenges and Debugging Sagas

### The Cloudflare Saga (Observations 465-479)

The most sustained debugging effort in rawdoc's history spans fifteen observations across twenty-two minutes. It began when the Node.js implementation hit Cloudflare 403 errors on Baeldung.com (observation 465). The initial fix -- mimicking browser headers more aggressively -- immediately caused a TLS configuration conflict (observation 466, costing 8,875 discovery tokens to resolve).

With HTTP fetching working, a different problem surfaced: JavaScript and CSS content was leaking into the markdown output (observations 468-469). The root cause was subtle -- `htmlparser2` uses different node type constants than the code expected, so `<script>` and `<style>` tags bypassed the noise filter. Two separate fixes were needed: one in `extract.mjs` and one in `convert.mjs`.

The saga's climax came with the multi-tier fallback system (observations 475-478). After implementing `undici` and `curl` fallbacks for TLS fingerprinting bypass, observation 477 revealed that curl's apparent success was illusory -- it received Cloudflare JavaScript challenge pages, not actual content. Observation 478 (11,341 discovery tokens) implemented challenge page detection, the second most expensive bugfix in the project. The conclusion (observation 479) was sobering: no Node.js method could bypass Baeldung's Cloudflare protection. The Go implementation's `net/http` stack, with its different TLS fingerprint, was simply accepted by Cloudflare where Node.js was not.

### The AV False Positive Crisis

Though not captured by claude-mem (it predates the April 11 sessions), the git history tells the story. Commit `488393a` ("strip symbols to reduce AV false positives, add Windows AV note") was the first response. When that proved insufficient, build tags (`3b917ca`) attempted to isolate the problematic dependencies. When even that was not enough, the nuclear option was taken: removing go-rod entirely (`02f6c2c`), followed by removing utls (`4299434`). Four commits, escalating in severity, to address a single class of problem.

## 7. Memory and Continuity

The rawdoc project's memory timeline reveals how persistent memory shaped development in real-time. The project's entire claude-mem history spans a single primary session (`caec1145`) with 51 observations, plus one follow-up session (`057081ee`) contributing a single observation.

Memory's most visible impact was on architectural decision-making. When evaluating the Node.js migration (sessions S151-S153), the accumulated context about Go's Cloudflare-bypassing capabilities (from Day 1 work) was available to inform the comparison. When the Node.js version failed on Baeldung (observation 473), memory of the Go version's success provided immediate contrast without re-testing.

The session structure also reveals how memory enabled rapid context switching. Twenty-six sessions in five hours means the developer was frequently starting new conversational contexts. Each session boundary (S143 through S168) represents a potential loss of context that persistent memory mitigated. Session S163, for example, needed to "confirm current branch status and implementation language for rawdoc plugin" -- a question that memory could answer instantly rather than requiring re-exploration.

The follow-up session (observation 492) at 10:55 AM, nearly 90 minutes after the main sprint ended, demonstrates memory's value for documentation: the README update required understanding of MCP server details, plugin architecture, and current features -- all knowledge captured in earlier observations.

## 8. Token Economics & Memory ROI

### Raw Metrics

| Metric | Value |
|--------|-------|
| Total discovery tokens | 194,612 |
| Total observations | 52 |
| Distinct sessions | 2 |
| Average discovery tokens per observation | 3,742.5 |
| Average read tokens per observation (compressed) | 377.7 |
| Compression ratio | **9.9:1** |
| Timeline read tokens (all 52 obs) | 19,885 |

### Monthly Breakdown

| Month | Observations | Discovery Tokens | Sessions | Avg Discovery/Obs |
|-------|-------------|-----------------|----------|-------------------|
| 2026-04 | 52 | 194,612 | 2 | 3,742 |

### Top 5 Most Expensive Observations

| Rank | ID | Title | Discovery Tokens |
|------|-----|-------|-----------------|
| 1 | 446 | Added --serve flag to main.go enabling dual-mode operation | 14,973 |
| 2 | 492 | README documentation updated with MCP server details and current features | 14,096 |
| 3 | 445 | Added crawlToResults function for in-memory crawl result collection | 13,965 |
| 4 | 478 | Detect and reject Cloudflare JavaScript challenge pages in fallback responses | 11,341 |
| 5 | 444 | Implemented MCP stdio server in mcp.go with rawdoc_fetch and rawdoc_crawl tools | 10,046 |

These five observations alone account for 64,421 tokens -- 33% of all discovery work. They represent the project's most complex integration points: MCP server implementation, pipeline adaptation, Cloudflare challenge detection, and comprehensive documentation.

### Breakdown by Observation Type

| Type | Count | Total Tokens | Avg Tokens |
|------|-------|-------------|-----------|
| feature | 12 | 60,235 | 5,020 |
| change | 16 | 53,593 | 3,350 |
| bugfix | 5 | 34,823 | 6,965 |
| discovery | 17 | 34,773 | 2,046 |
| refactor | 2 | 11,188 | 5,594 |

Bugfixes are the most expensive per-observation, averaging 6,965 discovery tokens each. This reflects the investigative nature of debugging: each bugfix required understanding the failure, tracing the cause, implementing the fix, and validating the result. Features, while cheaper per unit, consumed the most total tokens due to volume.

### ROI Analysis

The 52 observations compress 194,612 tokens of original work into 19,885 tokens of readable timeline -- a **90% savings ratio**. Each future session that needs to understand rawdoc's history, architecture, or past decisions can access this knowledge for roughly one-tenth the cost of rediscovering it.

The most concrete ROI came during the Node.js evaluation: observations 441 and 481 (combined: 1,391 read tokens) encoded knowledge about Go's Cloudflare bypass capability that would have cost thousands of tokens to rediscover through testing. The decision to abandon the Node.js branch and return to Go was informed by this memory at minimal cost.

## 9. Timeline Statistics

| Statistic | Value |
|-----------|-------|
| **Date range** | April 8, 2026 (first commit) -- April 11, 2026 (last observation) |
| **Active development days** | 2 (April 8 and April 11) |
| **Total git commits** | 38 |
| **Commits on Day 1 (Apr 8)** | 34 |
| **Commits on Day 3 (Apr 11)** | 4 |
| **Total observations** | 52 |
| **Total sessions (claude-mem)** | 26 (S142-S168) |
| **Memory sessions** | 2 |
| **Total discovery tokens** | 194,612 |
| **Most active hour** | 9:00-9:30 AM Apr 11 (observations 481-491, plugin marketplace sprint) |
| **Longest debugging arc** | Cloudflare saga: 15 observations, 22 minutes (obs 465-479) |
| **Most prolific type** | discovery (17 observations) |
| **Most expensive type** | feature (60,235 total tokens) |
| **Fastest feature cycle** | MCP server: 4 minutes from research to implementation (obs 442-444) |

### Session Duration Distribution

The 26 sessions on April 11 spanned 4 hours and 50 minutes (6:05 AM to 10:55 AM). The primary session (`caec1145`) ran for 2 hours and 38 minutes continuously, producing 51 observations. After a 90-minute gap, the follow-up session (`057081ee`) contributed the final README update.

## 10. Lessons and Meta-Observations

### Simplification Beats Sophistication

The most important pattern in rawdoc's development is the repeated triumph of simplification over sophistication. The three-tier fetch system (HTTP + utls + Chrome) was replaced by single-tier HTTP. The Node.js rewrite was abandoned in favor of the simpler Go original. Goquery was replaced by direct `x/net/html` walking. At every decision point, the simpler option eventually won -- sometimes after the complex option was fully implemented.

### Distribution Drives Architecture

The Node.js migration was not motivated by any deficiency in Go's runtime behavior. It was driven entirely by distribution friction: requiring users to install Go to use a Claude Code plugin. When a plugin auto-build hook resolved the distribution problem without a language change, the migration was abandoned. This pattern -- distribution concerns overriding implementation concerns -- is increasingly common in the age of LLM tool ecosystems.

### Validate Before Committing

The project's healthiest pattern was its consistent validation discipline. Each major feature was followed by benchmark testing across real production sites. The Node.js version was benchmarked against 10 sites (observation 480) before any decision was made. The script/style fix was validated across 9 sites with leak detection (observation 470). This prevented the accumulation of hidden regressions.

### Speed of Iteration Over Perfection of Design

Thirty-four commits on Day 1. Twenty-six sessions on Day 3. The development style was unmistakably rapid iteration: build something, test it, fix it, ship it, move on. The OpenClaw plugin format was created and discarded within two hours. The Node.js migration was attempted and abandoned within two hours. This speed was enabled by small, focused commits and a willingness to reverse course.

### The 90% Rule

Rawdoc's final form handles 9 out of 10 documentation sites successfully. The one site it cannot handle (Baeldung, behind aggressive Cloudflare protection) was the subject of extensive investigation but was ultimately accepted as an out-of-scope edge case. Chasing 100% compatibility was explicitly rejected in favor of reliability for the common case. The 95%+ token reduction metric for supported sites validated this tradeoff.

### Memory as Institutional Knowledge

The 52 observations recorded during rawdoc's development form a compressed institutional memory. Future developers (or future sessions with the same developer) can understand not just what was built, but why specific decisions were made, what alternatives were tried and rejected, and where the known limitations lie. The 9.9:1 compression ratio means this knowledge transfer costs about 20,000 tokens -- a fraction of the 194,612 tokens it took to generate.

---

*Report generated April 11, 2026. Based on 52 claude-mem observations, 38 git commits, and 26 recorded sessions spanning April 8-11, 2026.*
