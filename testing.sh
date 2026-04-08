#!/bin/bash
# rawdoc automated test suite
# Focuses on: performance timing, tier escalation, failure handling
# All content goes to files — console shows only metrics and status

set -o pipefail

# ─── Config ───────────────────────────────────────────────────────────────────

RAWDOC="./rawdoc"          # path to binary — adjust if needed
OUT_DIR="/tmp/rawdoc-tests"
LOG="$OUT_DIR/test-results.log"
PASS=0
FAIL=0
SKIP=0
TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
DIM='\033[0;90m'
BOLD='\033[1m'
NC='\033[0m'

# ─── Build ────────────────────────────────────────────────────────────────────

echo -e "${CYAN}Building rawdoc...${NC}"
cd "$(dirname "$0")" || exit 1
if ! go build -ldflags="-s -w" -o rawdoc . 2>&1; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi
echo -e "${GREEN}Built $(./rawdoc --version)${NC}"
RAWDOC="./rawdoc"
echo ""

# ─── Setup ────────────────────────────────────────────────────────────────────

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/tier1" "$OUT_DIR/tier2" "$OUT_DIR/tier3" \
         "$OUT_DIR/formats" "$OUT_DIR/crawl" "$OUT_DIR/failures" \
         "$OUT_DIR/edge" "$OUT_DIR/selectors" "$OUT_DIR/perf"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BOLD} rawdoc test suite${NC}"
echo -e "${DIM} $(date '+%Y-%m-%d %H:%M:%S')${NC}"
echo -e "${DIM} output: $OUT_DIR${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Log header
echo "rawdoc test results — $(date '+%Y-%m-%d %H:%M:%S')" > "$LOG"
echo "=================================================" >> "$LOG"
echo "" >> "$LOG"

# ─── Test Runner ──────────────────────────────────────────────────────────────

run_test() {
    local name="$1"
    local outfile="$2"
    shift 2
    local cmd=("$@")

    TOTAL=$((TOTAL + 1))

    # Time the execution
    local start_ms=$(($(date +%s%N)/1000000))
    local stderr_file="$OUT_DIR/.stderr_tmp"

    "${cmd[@]}" > "$outfile" 2>"$stderr_file"
    local exit_code=$?

    local end_ms=$(($(date +%s%N)/1000000))
    local duration_ms=$((end_ms - start_ms))

    # Get output size
    local size=0
    if [ -f "$outfile" ]; then
        size=$(wc -c < "$outfile" | tr -d ' ')
    fi

    # Format duration
    local duration_str
    if [ $duration_ms -ge 1000 ]; then
        duration_str="$(echo "scale=1; $duration_ms/1000" | bc)s"
    else
        duration_str="${duration_ms}ms"
    fi

    # Format size
    local size_str
    if [ $size -ge 1024 ]; then
        size_str="$(echo "scale=1; $size/1024" | bc)KB"
    else
        size_str="${size}B"
    fi

    # Determine status
    local status_icon status_color
    if [ $exit_code -eq 0 ] && [ $size -gt 100 ]; then
        status_icon="✓"
        status_color="$GREEN"
        PASS=$((PASS + 1))
    elif [ $exit_code -eq 0 ] && [ $size -le 100 ]; then
        status_icon="⚠"
        status_color="$YELLOW"
        SKIP=$((SKIP + 1))
    else
        status_icon="✗"
        status_color="$RED"
        FAIL=$((FAIL + 1))
    fi

    # Save stderr for stats extraction before any cleanup
    cp "$stderr_file" "$stderr_file.saved" 2>/dev/null

    # Stderr tier info (first line only)
    local tier_info=""
    if [ -f "$stderr_file" ] && [ -s "$stderr_file" ]; then
        tier_info=$(head -1 "$stderr_file" | sed 's/\x1b\[[0-9;]*m//g' | cut -c1-60)
    fi

    # Extract token stats from stderr
    local tokens_str=""
    if [ -f "$stderr_file.saved" ] && [ -s "$stderr_file.saved" ]; then
        local stats_line
        stats_line=$(grep '\[stats\]' "$stderr_file.saved" | head -1 | sed 's/\x1b\[[0-9;]*m//g')
        if [ -n "$stats_line" ]; then
            # Extract "N tokens) | XX% saved" → "N tok ↓XX%"
            local in_tok out_tok pct
            in_tok=$(echo "$stats_line" | grep -oP '\((\d+) tokens\)' | head -1 | grep -oP '\d+')
            pct=$(echo "$stats_line" | grep -oP '\d+% saved' | grep -oP '\d+')
            if [ -n "$in_tok" ] && [ -n "$pct" ]; then
                tokens_str="${in_tok}→↓${pct}%"
            fi
        fi
    fi

    # Preview — first non-empty content line
    local preview=""
    if [ -f "$outfile" ] && [ $size -gt 0 ]; then
        preview=$(grep -v '^\s*$' "$outfile" 2>/dev/null | head -1 | cut -c1-30)
    fi

    # Console output — single tabular line
    printf "${status_color}%s${NC}  %-35s %7s %8s  %-18s  ${DIM}%-30s${NC}\n" \
        "$status_icon" "$name" "$duration_str" "$size_str" "$tokens_str" "$preview"

    # Log output
    printf "%-2s %-45s %8s %8s exit=%d %s\n" \
        "$status_icon" "$name" "$duration_str" "$size_str" "$exit_code" "$tier_info" >> "$LOG"

    # Save stderr alongside output
    if [ -s "$stderr_file" ]; then
        cp "$stderr_file" "${outfile%.md}.stderr"
    fi
    rm -f "$stderr_file" "$stderr_file.saved"
}

run_test_expect_fail() {
    local name="$1"
    local outfile="$2"
    shift 2
    local cmd=("$@")

    TOTAL=$((TOTAL + 1))

    local start_ms=$(($(date +%s%N)/1000000))
    local stderr_file="$OUT_DIR/.stderr_tmp"

    "${cmd[@]}" > "$outfile" 2>"$stderr_file"
    local exit_code=$?

    local end_ms=$(($(date +%s%N)/1000000))
    local duration_ms=$((end_ms - start_ms))

    local duration_str
    if [ $duration_ms -ge 1000 ]; then
        duration_str="$(echo "scale=1; $duration_ms/1000" | bc)s"
    else
        duration_str="${duration_ms}ms"
    fi

    # For expected failures, non-zero exit = pass
    local status_icon status_color
    if [ $exit_code -ne 0 ]; then
        status_icon="✓"
        status_color="$GREEN"
        PASS=$((PASS + 1))
    else
        status_icon="✗"
        status_color="$RED"
        FAIL=$((FAIL + 1))
    fi

    local err_msg=""
    if [ -f "$stderr_file" ] && [ -s "$stderr_file" ]; then
        err_msg=$(head -1 "$stderr_file" | sed 's/\x1b\[[0-9;]*m//g' | cut -c1-60)
    fi

    printf "${status_color}%s${NC}  %-45s  %8s  exit=%-3d ${DIM}(expected≠0)${NC}" \
        "$status_icon" "$name" "$duration_str" "$exit_code"
    if [ -n "$err_msg" ]; then
        printf "  ${DIM}%s${NC}" "$err_msg"
    fi
    echo ""

    printf "%-2s %-45s %8s exit=%d (expected≠0) %s\n" \
        "$status_icon" "$name" "$duration_str" "$exit_code" "$err_msg" >> "$LOG"

    if [ -s "$stderr_file" ]; then
        cp "$stderr_file" "${outfile%.md}.stderr"
    fi
    rm -f "$stderr_file"
}

run_crawl_test() {
    local name="$1"
    local outdir="$2"
    shift 2
    local cmd=("$@")

    TOTAL=$((TOTAL + 1))

    local start_ms=$(($(date +%s%N)/1000000))
    local stderr_file="$OUT_DIR/.stderr_tmp"

    "${cmd[@]}" 2>"$stderr_file"
    local exit_code=$?

    local end_ms=$(($(date +%s%N)/1000000))
    local duration_ms=$((end_ms - start_ms))

    local duration_str
    if [ $duration_ms -ge 1000 ]; then
        duration_str="$(echo "scale=1; $duration_ms/1000" | bc)s"
    else
        duration_str="${duration_ms}ms"
    fi

    # Count files
    local file_count=0
    local total_size=0
    if [ -d "$outdir" ]; then
        file_count=$(find "$outdir" -name "*.md" | wc -l | tr -d ' ')
        total_size=$(du -sb "$outdir" 2>/dev/null | cut -f1)
    fi

    local size_str
    if [ $total_size -ge 1048576 ]; then
        size_str="$(echo "scale=1; $total_size/1048576" | bc)MB"
    elif [ $total_size -ge 1024 ]; then
        size_str="$(echo "scale=1; $total_size/1024" | bc)KB"
    else
        size_str="${total_size}B"
    fi

    local status_icon status_color
    if [ $exit_code -eq 0 ] && [ $file_count -gt 0 ]; then
        status_icon="✓"
        status_color="$GREEN"
        PASS=$((PASS + 1))
    else
        status_icon="✗"
        status_color="$RED"
        FAIL=$((FAIL + 1))
    fi

    printf "${status_color}%s${NC}  %-45s  %8s  %4d files  %8s  exit=%d\n" \
        "$status_icon" "$name" "$duration_str" "$file_count" "$size_str" "$exit_code"

    printf "%-2s %-45s %8s %d files %8s exit=%d\n" \
        "$status_icon" "$name" "$duration_str" "$file_count" "$size_str" "$exit_code" >> "$LOG"

    if [ -s "$stderr_file" ]; then
        cp "$stderr_file" "$outdir/.stderr"
    fi
    rm -f "$stderr_file"
}

# ─── Section Header ──────────────────────────────────────────────────────────

section() {
    echo ""
    echo -e "${CYAN}── $1 ──${NC}"
    echo "" >> "$LOG"
    echo "── $1 ──" >> "$LOG"
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  TESTS
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# ── Tier 1: Plain HTTP ────────────────────────────────────────────────────────

section "TIER 1 — Plain HTTP (no protection)"

run_test "k8s/pods" \
    "$OUT_DIR/tier1/k8s-pods.md" \
    $RAWDOC https://kubernetes.io/docs/concepts/workloads/pods/ -v

run_test "k8s/ingress" \
    "$OUT_DIR/tier1/k8s-ingress.md" \
    $RAWDOC https://kubernetes.io/docs/concepts/services-networking/ingress/ -v

run_test "go-pkg/net-http" \
    "$OUT_DIR/tier1/go-net-http.md" \
    $RAWDOC https://pkg.go.dev/net/http -v

run_test "go/effective-go" \
    "$OUT_DIR/tier1/effective-go.md" \
    $RAWDOC https://go.dev/doc/effective_go -v

run_test "spring.io/blog" \
    "$OUT_DIR/tier1/spring-blog.md" \
    $RAWDOC https://spring.io/blog -v

run_test "mdn/http-status" \
    "$OUT_DIR/tier1/mdn-http-status.md" \
    $RAWDOC https://developer.mozilla.org/en-US/docs/Web/HTTP/Status -v

run_test "postgresql/select" \
    "$OUT_DIR/tier1/pg-select.md" \
    $RAWDOC https://www.postgresql.org/docs/current/sql-select.html -v

run_test "istio/traffic-mgmt" \
    "$OUT_DIR/tier1/istio-traffic.md" \
    $RAWDOC https://istio.io/latest/docs/concepts/traffic-management/ -v

run_test "helm/chart-templates" \
    "$OUT_DIR/tier1/helm-templates.md" \
    $RAWDOC https://helm.sh/docs/chart_template_guide/ -v

run_test "python/asyncio" \
    "$OUT_DIR/tier1/python-asyncio.md" \
    $RAWDOC https://docs.python.org/3/library/asyncio.html -v

run_test "gobyexample/goroutines" \
    "$OUT_DIR/tier1/gobyexample-goroutines.md" \
    $RAWDOC https://gobyexample.com/goroutines -v

run_test "redis/set-command" \
    "$OUT_DIR/tier1/redis-set.md" \
    $RAWDOC https://redis.io/docs/latest/commands/set/ -v

# ── Tier 2: Cloudflare / TLS Spoofing ────────────────────────────────────────

section "TIER 2 — Cloudflare / utls spoofing"

run_test "baeldung/spring-kafka" \
    "$OUT_DIR/tier2/baeldung-spring-kafka.md" \
    $RAWDOC https://www.baeldung.com/spring-kafka -v

run_test "baeldung/jpa-query" \
    "$OUT_DIR/tier2/baeldung-jpa-query.md" \
    $RAWDOC https://www.baeldung.com/spring-data-jpa-query -v

run_test "baeldung/spring-boot-start" \
    "$OUT_DIR/tier2/baeldung-boot-start.md" \
    $RAWDOC https://www.baeldung.com/spring-boot-start -v

run_test "baeldung/spring-security" \
    "$OUT_DIR/tier2/baeldung-security.md" \
    $RAWDOC https://www.baeldung.com/spring-security-login -v

run_test "baeldung/java-streams" \
    "$OUT_DIR/tier2/baeldung-streams.md" \
    $RAWDOC https://www.baeldung.com/java-streams -v

run_test "digitalocean/nginx-tutorial" \
    "$OUT_DIR/tier2/do-nginx.md" \
    $RAWDOC https://www.digitalocean.com/community/tutorials/how-to-install-nginx-on-ubuntu-22-04 -v

# ── Tier 3: JS Rendered ──────────────────────────────────────────────────────

section "TIER 3 — JS rendered (headless Chrome)"

run_test "react.dev/learn" \
    "$OUT_DIR/tier3/react-learn.md" \
    $RAWDOC https://react.dev/learn -v

run_test "github-docs/actions" \
    "$OUT_DIR/tier3/github-actions.md" \
    $RAWDOC https://docs.github.com/en/actions -v

run_test "nextjs/getting-started" \
    "$OUT_DIR/tier3/nextjs-start.md" \
    $RAWDOC https://nextjs.org/docs/getting-started/installation -v

run_test "azure/aks-intro" \
    "$OUT_DIR/tier3/azure-aks.md" \
    $RAWDOC https://learn.microsoft.com/en-us/azure/aks/intro-kubernetes -v

# ── Output Formats ────────────────────────────────────────────────────────────

section "OUTPUT FORMATS — same URL, different formats"

TEST_URL="https://kubernetes.io/docs/concepts/workloads/pods/"

run_test "format/markdown" \
    "$OUT_DIR/formats/markdown.md" \
    $RAWDOC "$TEST_URL" -f markdown

run_test "format/text" \
    "$OUT_DIR/formats/text.txt" \
    $RAWDOC "$TEST_URL" -f text

run_test "format/json" \
    "$OUT_DIR/formats/json.json" \
    $RAWDOC "$TEST_URL" -f json

run_test "format/yaml" \
    "$OUT_DIR/formats/yaml.yaml" \
    $RAWDOC "$TEST_URL" -f yaml

run_test "format/code-only" \
    "$OUT_DIR/formats/code-only.md" \
    $RAWDOC "$TEST_URL" --code-only

run_test "format/no-links" \
    "$OUT_DIR/formats/no-links.md" \
    $RAWDOC "$TEST_URL" --no-links

# ── Crawl Mode ────────────────────────────────────────────────────────────────

section "CRAWL MODE — multi-page"

run_crawl_test "crawl/k8s-workloads (depth=1, max=10)" \
    "$OUT_DIR/crawl/k8s" \
    $RAWDOC https://kubernetes.io/docs/concepts/workloads/ -d 1 --max-pages 10 -o "$OUT_DIR/crawl/k8s" -v

run_crawl_test "crawl/gobyexample (depth=1, max=10)" \
    "$OUT_DIR/crawl/gobyexample" \
    $RAWDOC https://gobyexample.com/ -d 1 --max-pages 10 -o "$OUT_DIR/crawl/gobyexample" -v

run_crawl_test "crawl/helm (depth=1, max=8)" \
    "$OUT_DIR/crawl/helm" \
    $RAWDOC https://helm.sh/docs/ -d 1 --max-pages 8 -o "$OUT_DIR/crawl/helm" -v

run_crawl_test "crawl/baeldung-spring (depth=1, max=5)" \
    "$OUT_DIR/crawl/baeldung" \
    $RAWDOC https://www.baeldung.com/spring-boot -d 1 --max-pages 5 --include "/spring-*" -o "$OUT_DIR/crawl/baeldung" -v

# ── Failure Handling ──────────────────────────────────────────────────────────

section "FAILURE HANDLING — expected failures"

run_test_expect_fail "fail/invalid-url" \
    "$OUT_DIR/failures/invalid-url.md" \
    $RAWDOC not-a-valid-url

run_test_expect_fail "fail/nonexistent-domain" \
    "$OUT_DIR/failures/nonexistent.md" \
    $RAWDOC https://this-domain-does-not-exist-xyz123.com/page

run_test_expect_fail "fail/http-403" \
    "$OUT_DIR/failures/http-403.md" \
    $RAWDOC https://httpstat.us/403 -v

run_test_expect_fail "fail/http-500" \
    "$OUT_DIR/failures/http-500.md" \
    $RAWDOC https://httpstat.us/500 -v

run_test_expect_fail "fail/http-429" \
    "$OUT_DIR/failures/http-429.md" \
    $RAWDOC https://httpstat.us/429 -v

run_test_expect_fail "fail/empty-url" \
    "$OUT_DIR/failures/empty-url.md" \
    $RAWDOC ""

# ── Edge Cases ────────────────────────────────────────────────────────────────

section "EDGE CASES"

run_test "edge/example.com (minimal)" \
    "$OUT_DIR/edge/example-com.md" \
    $RAWDOC https://example.com -v

run_test "edge/redirect-golang.org" \
    "$OUT_DIR/edge/redirect-golang.md" \
    $RAWDOC https://golang.org/ -v

run_test "edge/redirect-reactjs.org" \
    "$OUT_DIR/edge/redirect-reactjs.md" \
    $RAWDOC https://reactjs.org/ -v

run_test "edge/hacker-news (table layout)" \
    "$OUT_DIR/edge/hackernews.md" \
    $RAWDOC https://news.ycombinator.com/ -v

run_test "edge/gobyexample-hello (minimal)" \
    "$OUT_DIR/edge/gobyexample-hello.md" \
    $RAWDOC https://gobyexample.com/hello-world -v

# ── Flag Combinations ────────────────────────────────────────────────────────

section "FLAG COMBINATIONS"

run_test "flags/code-only+baeldung" \
    "$OUT_DIR/edge/baeldung-code-only.md" \
    $RAWDOC https://www.baeldung.com/spring-kafka --code-only -v

run_test "flags/json+baeldung" \
    "$OUT_DIR/edge/baeldung-json.json" \
    $RAWDOC https://www.baeldung.com/spring-kafka -f json -v

run_test "flags/no-links+mdn" \
    "$OUT_DIR/edge/mdn-no-links.md" \
    $RAWDOC https://developer.mozilla.org/en-US/docs/Web/HTTP/Status --no-links -v

run_test "flags/no-tls-spoof+baeldung" \
    "$OUT_DIR/edge/baeldung-no-spoof.md" \
    $RAWDOC https://www.baeldung.com/spring-kafka --no-tls-spoof -v

run_test "flags/no-headless+react" \
    "$OUT_DIR/edge/react-no-headless.md" \
    $RAWDOC https://react.dev/learn --no-headless -v

run_test "flags/timeout-short" \
    "$OUT_DIR/edge/timeout-short.md" \
    $RAWDOC https://kubernetes.io/docs/concepts/workloads/pods/ --timeout 2s -v

run_test "flags/custom-header" \
    "$OUT_DIR/edge/custom-header.md" \
    $RAWDOC https://kubernetes.io/docs/concepts/workloads/pods/ -header "X-Test=rawdoc" -v

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  SUMMARY
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BOLD} RESULTS${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "  Total:    ${BOLD}$TOTAL${NC}"
echo -e "  Passed:   ${GREEN}$PASS${NC}"
echo -e "  Failed:   ${RED}$FAIL${NC}"
echo -e "  Warning:  ${YELLOW}$SKIP${NC}"
echo ""
echo -e "  Output:   ${DIM}$OUT_DIR${NC}"
echo -e "  Log:      ${DIM}$LOG${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Summary to log
echo "" >> "$LOG"
echo "=================================================" >> "$LOG"
echo "TOTAL: $TOTAL  PASS: $PASS  FAIL: $FAIL  WARN: $SKIP" >> "$LOG"

# Exit with failure if any test failed
if [ $FAIL -gt 0 ]; then
    exit 1
fi
exit 0
