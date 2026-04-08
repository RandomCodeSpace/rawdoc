# rawdoc automated test suite — PowerShell
# Mirrors testing.sh: performance timing, tier escalation, failure handling
# All content goes to files — console shows only metrics and status

#Requires -Version 5.1

# ─── Config ───────────────────────────────────────────────────────────────────

$RAWDOC  = ".\rawdoc.exe"
$OUT_DIR = "$env:TEMP\rawdoc-tests"
$LOG     = "$OUT_DIR\test-results.log"
$PASS    = 0
$FAIL    = 0
$WARN    = 0
$TOTAL   = 0

# ─── Build ────────────────────────────────────────────────────────────────────

Write-Host "Building rawdoc..." -ForegroundColor Cyan

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$buildOutput = & go build -o rawdoc.exe . 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed:" -ForegroundColor Red
    Write-Host $buildOutput
    exit 1
}

$version = & .\rawdoc.exe --version 2>&1
Write-Host "Built $version" -ForegroundColor Green
Write-Host ""

# ─── Setup ────────────────────────────────────────────────────────────────────

if (Test-Path $OUT_DIR) { Remove-Item $OUT_DIR -Recurse -Force }
foreach ($sub in @("tier1","tier2","tier3","formats","crawl","failures","edge","selectors","perf")) {
    New-Item -ItemType Directory -Path "$OUT_DIR\$sub" -Force | Out-Null
}

$sep = "━" * 54
Write-Host $sep
Write-Host " rawdoc test suite" -ForegroundColor White
Write-Host " $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor DarkGray
Write-Host " output: $OUT_DIR"  -ForegroundColor DarkGray
Write-Host $sep
Write-Host ""

# Log header
"rawdoc test results — $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" | Out-File $LOG -Encoding utf8
"=================================================" | Out-File $LOG -Append -Encoding utf8
""                                                  | Out-File $LOG -Append -Encoding utf8

# ─── Helpers ──────────────────────────────────────────────────────────────────

function Format-Duration([long]$ms) {
    if ($ms -ge 1000) { return "{0:F1}s" -f ($ms / 1000.0) }
    return "${ms}ms"
}

function Format-Size([long]$bytes) {
    if ($bytes -ge 1048576) { return "{0:F1}MB" -f ($bytes / 1048576.0) }
    if ($bytes -ge 1024)    { return "{0:F1}KB" -f ($bytes / 1024.0) }
    return "${bytes}B"
}

function Strip-Ansi([string]$s) {
    return [regex]::Replace($s, '\x1b\[[0-9;]*m', '')
}

# ─── Test Runner ──────────────────────────────────────────────────────────────

function Run-Test {
    param(
        [string]   $Name,
        [string]   $OutFile,
        [string[]] $CmdArgs
    )

    $script:TOTAL++

    $stderrFile = "$OUT_DIR\.stderr_tmp"
    $startMs    = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds

    # Run command, capture stdout to file, stderr to temp file
    $exe  = $CmdArgs[0]
    $args = @()
    if ($CmdArgs.Count -gt 1) { $args = $CmdArgs[1..($CmdArgs.Count-1)] }
    & $exe @args > $OutFile 2> $stderrFile
    $exitCode = $LASTEXITCODE

    $endMs      = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds
    $durationMs = $endMs - $startMs

    $size = 0
    if (Test-Path $OutFile) { $size = (Get-Item $OutFile).Length }

    $durationStr = Format-Duration $durationMs
    $sizeStr     = Format-Size $size

    # Status
    if ($exitCode -eq 0 -and $size -gt 100) {
        $icon  = [char]0x2713   # ✓
        $color = "Green"
        $script:PASS++
    } elseif ($exitCode -eq 0 -and $size -le 100) {
        $icon  = [char]0x26A0   # ⚠
        $color = "Yellow"
        $script:WARN++
    } else {
        $icon  = [char]0x2717   # ✗
        $color = "Red"
        $script:FAIL++
    }

    # Extract token stats from stderr  ([stats] line)
    $tokensStr = ""
    if ((Test-Path $stderrFile) -and (Get-Item $stderrFile).Length -gt 0) {
        $stderrLines = Get-Content $stderrFile
        $statsLine   = $stderrLines | Where-Object { (Strip-Ansi $_) -match '\[stats\]' } | Select-Object -First 1
        if ($statsLine) {
            $clean = Strip-Ansi $statsLine
            if ($clean -match '\((\d+)\s+tokens\)' -and $clean -match '(\d+)%\s+saved') {
                $tokensStr = "$($Matches[1])→↓$($Matches[1])%"
                # Re-match for pct separately
                if ($clean -match '\((\d+)\s+tokens\)') { $inTok = $Matches[1] }
                if ($clean -match '(\d+)%\s+saved')     { $pct   = $Matches[1] }
                $tokensStr = "${inTok}→↓${pct}%"
            }
        }
    }

    # Preview — first non-empty line, max 30 chars
    $preview = ""
    if ((Test-Path $OutFile) -and $size -gt 0) {
        $preview = (Get-Content $OutFile | Where-Object { $_ -match '\S' } | Select-Object -First 1)
        if ($preview.Length -gt 30) { $preview = $preview.Substring(0, 30) }
    }

    # Console — tabular single line
    Write-Host ("{0}  {1,-35} {2,7} {3,8}  {4,-18}  {5,-30}" -f `
        $icon, $Name, $durationStr, $sizeStr, $tokensStr, $preview) -ForegroundColor $color

    # Log
    "{0,-2} {1,-45} {2,8} {3,8} exit={4}" -f $icon, $Name, $durationStr, $sizeStr, $exitCode |
        Out-File $LOG -Append -Encoding utf8

    # Save stderr alongside output
    if ((Test-Path $stderrFile) -and (Get-Item $stderrFile).Length -gt 0) {
        $base = [System.IO.Path]::ChangeExtension($OutFile, $null).TrimEnd('.')
        Copy-Item $stderrFile "${base}.stderr" -Force
    }
    Remove-Item $stderrFile -ErrorAction SilentlyContinue
}

function Run-Test-ExpectFail {
    param(
        [string]   $Name,
        [string]   $OutFile,
        [string[]] $CmdArgs
    )

    $script:TOTAL++

    $stderrFile = "$OUT_DIR\.stderr_tmp"
    $startMs    = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds

    $exe  = $CmdArgs[0]
    $args = @()
    if ($CmdArgs.Count -gt 1) { $args = $CmdArgs[1..($CmdArgs.Count-1)] }
    & $exe @args > $OutFile 2> $stderrFile
    $exitCode = $LASTEXITCODE

    $endMs      = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds
    $durationMs = $endMs - $startMs
    $durationStr = Format-Duration $durationMs

    # Non-zero exit = pass for expected-failure tests
    if ($exitCode -ne 0) {
        $icon  = [char]0x2713   # ✓
        $color = "Green"
        $script:PASS++
    } else {
        $icon  = [char]0x2717   # ✗
        $color = "Red"
        $script:FAIL++
    }

    $errMsg = ""
    if ((Test-Path $stderrFile) -and (Get-Item $stderrFile).Length -gt 0) {
        $errMsg = Strip-Ansi (Get-Content $stderrFile | Select-Object -First 1)
        if ($errMsg.Length -gt 60) { $errMsg = $errMsg.Substring(0, 60) }
    }

    $line = "{0}  {1,-45}  {2,8}  exit={3,-3} (expected≠0)" -f $icon, $Name, $durationStr, $exitCode
    if ($errMsg) { $line += "  $errMsg" }
    Write-Host $line -ForegroundColor $color

    "{0,-2} {1,-45} {2,8} exit={3} (expected≠0) {4}" -f $icon, $Name, $durationStr, $exitCode, $errMsg |
        Out-File $LOG -Append -Encoding utf8

    if ((Test-Path $stderrFile) -and (Get-Item $stderrFile).Length -gt 0) {
        $base = [System.IO.Path]::ChangeExtension($OutFile, $null).TrimEnd('.')
        Copy-Item $stderrFile "${base}.stderr" -Force
    }
    Remove-Item $stderrFile -ErrorAction SilentlyContinue
}

function Run-Crawl-Test {
    param(
        [string]   $Name,
        [string]   $CrawlOutDir,
        [string[]] $CmdArgs
    )

    $script:TOTAL++

    $stderrFile = "$OUT_DIR\.stderr_tmp"
    $startMs    = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds

    # Crawl writes to outdir directly — discard stdout
    $exe  = $CmdArgs[0]
    $args = @()
    if ($CmdArgs.Count -gt 1) { $args = $CmdArgs[1..($CmdArgs.Count-1)] }
    & $exe @args > $null 2> $stderrFile
    $exitCode = $LASTEXITCODE

    $endMs      = [long]([datetime]::UtcNow - [datetime]::new(1970,1,1)).TotalMilliseconds
    $durationMs = $endMs - $startMs
    $durationStr = Format-Duration $durationMs

    $fileCount  = 0
    $totalBytes = 0
    if (Test-Path $CrawlOutDir) {
        $mdFiles    = Get-ChildItem $CrawlOutDir -Filter "*.md" -Recurse -ErrorAction SilentlyContinue
        $fileCount  = $mdFiles.Count
        $totalBytes = ($mdFiles | Measure-Object -Property Length -Sum).Sum
        if (-not $totalBytes) { $totalBytes = 0 }
    }
    $sizeStr = Format-Size $totalBytes

    if ($exitCode -eq 0 -and $fileCount -gt 0) {
        $icon  = [char]0x2713   # ✓
        $color = "Green"
        $script:PASS++
    } else {
        $icon  = [char]0x2717   # ✗
        $color = "Red"
        $script:FAIL++
    }

    Write-Host ("{0}  {1,-45}  {2,8}  {3,4} files  {4,8}  exit={5}" -f `
        $icon, $Name, $durationStr, $fileCount, $sizeStr, $exitCode) -ForegroundColor $color

    "{0,-2} {1,-45} {2,8} {3} files {4,8} exit={5}" -f $icon, $Name, $durationStr, $fileCount, $sizeStr, $exitCode |
        Out-File $LOG -Append -Encoding utf8

    if ((Test-Path $stderrFile) -and (Get-Item $stderrFile).Length -gt 0) {
        Copy-Item $stderrFile "$CrawlOutDir\.stderr" -Force
    }
    Remove-Item $stderrFile -ErrorAction SilentlyContinue
}

function Section([string]$title) {
    Write-Host ""
    Write-Host "── $title ──" -ForegroundColor Cyan
    ""            | Out-File $LOG -Append -Encoding utf8
    "── $title ──" | Out-File $LOG -Append -Encoding utf8
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  TESTS
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# ── Tier 1: Plain HTTP ────────────────────────────────────────────────────────

Section "TIER 1 — Plain HTTP (no protection)"

Run-Test "k8s/pods"          "$OUT_DIR\tier1\k8s-pods.md"             @($RAWDOC, "https://kubernetes.io/docs/concepts/workloads/pods/", "-v")
Run-Test "k8s/ingress"       "$OUT_DIR\tier1\k8s-ingress.md"          @($RAWDOC, "https://kubernetes.io/docs/concepts/services-networking/ingress/", "-v")
Run-Test "go-pkg/net-http"   "$OUT_DIR\tier1\go-net-http.md"          @($RAWDOC, "https://pkg.go.dev/net/http", "-v")
Run-Test "go/effective-go"   "$OUT_DIR\tier1\effective-go.md"         @($RAWDOC, "https://go.dev/doc/effective_go", "-v")
Run-Test "spring.io/blog"    "$OUT_DIR\tier1\spring-blog.md"          @($RAWDOC, "https://spring.io/blog", "-v")
Run-Test "mdn/http-status"   "$OUT_DIR\tier1\mdn-http-status.md"      @($RAWDOC, "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status", "-v")
Run-Test "postgresql/select" "$OUT_DIR\tier1\pg-select.md"            @($RAWDOC, "https://www.postgresql.org/docs/current/sql-select.html", "-v")
Run-Test "istio/traffic-mgmt" "$OUT_DIR\tier1\istio-traffic.md"        @($RAWDOC, "https://istio.io/latest/docs/concepts/traffic-management/", "-v")
Run-Test "helm/chart-templates" "$OUT_DIR\tier1\helm-templates.md"    @($RAWDOC, "https://helm.sh/docs/chart_template_guide/", "-v")
Run-Test "python/asyncio"    "$OUT_DIR\tier1\python-asyncio.md"       @($RAWDOC, "https://docs.python.org/3/library/asyncio.html", "-v")
Run-Test "gobyexample/goroutines" "$OUT_DIR\tier1\gobyexample-goroutines.md" @($RAWDOC, "https://gobyexample.com/goroutines", "-v")
Run-Test "redis/set-command" "$OUT_DIR\tier1\redis-set.md"            @($RAWDOC, "https://redis.io/docs/latest/commands/set/", "-v")

# ── Tier 2: Cloudflare / TLS Spoofing ────────────────────────────────────────

Section "TIER 2 — Cloudflare / utls spoofing"

Run-Test "baeldung/spring-kafka"    "$OUT_DIR\tier2\baeldung-spring-kafka.md"  @($RAWDOC, "https://www.baeldung.com/spring-kafka", "-v")
Run-Test "baeldung/jpa-query"       "$OUT_DIR\tier2\baeldung-jpa-query.md"     @($RAWDOC, "https://www.baeldung.com/spring-data-jpa-query", "-v")
Run-Test "baeldung/spring-boot-start" "$OUT_DIR\tier2\baeldung-boot-start.md" @($RAWDOC, "https://www.baeldung.com/spring-boot-start", "-v")
Run-Test "baeldung/spring-security" "$OUT_DIR\tier2\baeldung-security.md"     @($RAWDOC, "https://www.baeldung.com/spring-security-login", "-v")
Run-Test "baeldung/java-streams"    "$OUT_DIR\tier2\baeldung-streams.md"       @($RAWDOC, "https://www.baeldung.com/java-streams", "-v")
Run-Test "digitalocean/nginx-tutorial" "$OUT_DIR\tier2\do-nginx.md"           @($RAWDOC, "https://www.digitalocean.com/community/tutorials/how-to-install-nginx-on-ubuntu-22-04", "-v")

# ── Tier 3: JS Rendered ──────────────────────────────────────────────────────

Section "TIER 3 — JS rendered (headless Chrome)"

Run-Test "react.dev/learn"        "$OUT_DIR\tier3\react-learn.md"    @($RAWDOC, "https://react.dev/learn", "-v")
Run-Test "github-docs/actions"    "$OUT_DIR\tier3\github-actions.md" @($RAWDOC, "https://docs.github.com/en/actions", "-v")
Run-Test "nextjs/getting-started" "$OUT_DIR\tier3\nextjs-start.md"   @($RAWDOC, "https://nextjs.org/docs/getting-started/installation", "-v")
Run-Test "azure/aks-intro"        "$OUT_DIR\tier3\azure-aks.md"      @($RAWDOC, "https://learn.microsoft.com/en-us/azure/aks/intro-kubernetes", "-v")

# ── Output Formats ────────────────────────────────────────────────────────────

Section "OUTPUT FORMATS — same URL, different formats"

$TEST_URL = "https://kubernetes.io/docs/concepts/workloads/pods/"

Run-Test "format/markdown"  "$OUT_DIR\formats\markdown.md"    @($RAWDOC, $TEST_URL, "-f", "markdown")
Run-Test "format/text"      "$OUT_DIR\formats\text.txt"       @($RAWDOC, $TEST_URL, "-f", "text")
Run-Test "format/json"      "$OUT_DIR\formats\json.json"      @($RAWDOC, $TEST_URL, "-f", "json")
Run-Test "format/yaml"      "$OUT_DIR\formats\yaml.yaml"      @($RAWDOC, $TEST_URL, "-f", "yaml")
Run-Test "format/code-only" "$OUT_DIR\formats\code-only.md"   @($RAWDOC, $TEST_URL, "--code-only")
Run-Test "format/no-links"  "$OUT_DIR\formats\no-links.md"    @($RAWDOC, $TEST_URL, "--no-links")

# ── Crawl Mode ────────────────────────────────────────────────────────────────

Section "CRAWL MODE — multi-page"

Run-Crawl-Test "crawl/k8s-workloads (depth=1, max=10)" "$OUT_DIR\crawl\k8s" `
    @($RAWDOC, "https://kubernetes.io/docs/concepts/workloads/", "-d", "1", "--max-pages", "10", "-o", "$OUT_DIR\crawl\k8s", "-v")

Run-Crawl-Test "crawl/gobyexample (depth=1, max=10)" "$OUT_DIR\crawl\gobyexample" `
    @($RAWDOC, "https://gobyexample.com/", "-d", "1", "--max-pages", "10", "-o", "$OUT_DIR\crawl\gobyexample", "-v")

Run-Crawl-Test "crawl/helm (depth=1, max=8)" "$OUT_DIR\crawl\helm" `
    @($RAWDOC, "https://helm.sh/docs/", "-d", "1", "--max-pages", "8", "-o", "$OUT_DIR\crawl\helm", "-v")

Run-Crawl-Test "crawl/baeldung-spring (depth=1, max=5)" "$OUT_DIR\crawl\baeldung" `
    @($RAWDOC, "https://www.baeldung.com/spring-boot", "-d", "1", "--max-pages", "5", "--include", "/spring-*", "-o", "$OUT_DIR\crawl\baeldung", "-v")

# ── Failure Handling ──────────────────────────────────────────────────────────

Section "FAILURE HANDLING — expected failures"

Run-Test-ExpectFail "fail/invalid-url"       "$OUT_DIR\failures\invalid-url.md"   @($RAWDOC, "not-a-valid-url")
Run-Test-ExpectFail "fail/nonexistent-domain" "$OUT_DIR\failures\nonexistent.md"   @($RAWDOC, "https://this-domain-does-not-exist-xyz123.com/page")
Run-Test-ExpectFail "fail/http-403"          "$OUT_DIR\failures\http-403.md"      @($RAWDOC, "https://httpstat.us/403", "-v")
Run-Test-ExpectFail "fail/http-500"          "$OUT_DIR\failures\http-500.md"      @($RAWDOC, "https://httpstat.us/500", "-v")
Run-Test-ExpectFail "fail/http-429"          "$OUT_DIR\failures\http-429.md"      @($RAWDOC, "https://httpstat.us/429", "-v")
Run-Test-ExpectFail "fail/empty-url"         "$OUT_DIR\failures\empty-url.md"     @($RAWDOC, "")

# ── Edge Cases ────────────────────────────────────────────────────────────────

Section "EDGE CASES"

Run-Test "edge/example.com (minimal)"       "$OUT_DIR\edge\example-com.md"       @($RAWDOC, "https://example.com", "-v")
Run-Test "edge/redirect-golang.org"         "$OUT_DIR\edge\redirect-golang.md"   @($RAWDOC, "https://golang.org/", "-v")
Run-Test "edge/redirect-reactjs.org"        "$OUT_DIR\edge\redirect-reactjs.md"  @($RAWDOC, "https://reactjs.org/", "-v")
Run-Test "edge/hacker-news (table layout)"  "$OUT_DIR\edge\hackernews.md"        @($RAWDOC, "https://news.ycombinator.com/", "-v")
Run-Test "edge/gobyexample-hello (minimal)" "$OUT_DIR\edge\gobyexample-hello.md" @($RAWDOC, "https://gobyexample.com/hello-world", "-v")

# ── Flag Combinations ────────────────────────────────────────────────────────

Section "FLAG COMBINATIONS"

Run-Test "flags/code-only+baeldung"   "$OUT_DIR\edge\baeldung-code-only.md" @($RAWDOC, "https://www.baeldung.com/spring-kafka", "--code-only", "-v")
Run-Test "flags/json+baeldung"        "$OUT_DIR\edge\baeldung-json.json"    @($RAWDOC, "https://www.baeldung.com/spring-kafka", "-f", "json", "-v")
Run-Test "flags/no-links+mdn"         "$OUT_DIR\edge\mdn-no-links.md"       @($RAWDOC, "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status", "--no-links", "-v")
Run-Test "flags/no-tls-spoof+baeldung" "$OUT_DIR\edge\baeldung-no-spoof.md"  @($RAWDOC, "https://www.baeldung.com/spring-kafka", "--no-tls-spoof", "-v")
Run-Test "flags/no-headless+react"    "$OUT_DIR\edge\react-no-headless.md"  @($RAWDOC, "https://react.dev/learn", "--no-headless", "-v")
Run-Test "flags/timeout-short"        "$OUT_DIR\edge\timeout-short.md"      @($RAWDOC, "https://kubernetes.io/docs/concepts/workloads/pods/", "--timeout", "2s", "-v")
Run-Test "flags/custom-header"        "$OUT_DIR\edge\custom-header.md"      @($RAWDOC, "https://kubernetes.io/docs/concepts/workloads/pods/", "-header", "X-Test=rawdoc", "-v")

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  SUMMARY
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Write-Host ""
Write-Host $sep
Write-Host " RESULTS" -ForegroundColor White
Write-Host $sep
Write-Host ("  Total:    {0}" -f $TOTAL)
Write-Host ("  Passed:   {0}" -f $PASS)  -ForegroundColor Green
Write-Host ("  Failed:   {0}" -f $FAIL)  -ForegroundColor Red
Write-Host ("  Warning:  {0}" -f $WARN)  -ForegroundColor Yellow
Write-Host ""
Write-Host ("  Output:   {0}" -f $OUT_DIR) -ForegroundColor DarkGray
Write-Host ("  Log:      {0}" -f $LOG)     -ForegroundColor DarkGray
Write-Host $sep

# Summary to log
""                                                                    | Out-File $LOG -Append -Encoding utf8
"================================================="                   | Out-File $LOG -Append -Encoding utf8
"TOTAL: $TOTAL  PASS: $PASS  FAIL: $FAIL  WARN: $WARN"               | Out-File $LOG -Append -Encoding utf8

# Exit with failure if any test failed
if ($FAIL -gt 0) { exit 1 }
exit 0
