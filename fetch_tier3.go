//go:build tier3 || all

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func fetchTier3(rawURL string, opts *fetchOptions) (*fetchResult, error) {
	chromePath := findChrome()
	if chromePath == "" {
		logVerbose(opts, "[tier3] Chrome not found at any known path")
		return nil, &httpError{statusCode: 0, message: "Chrome not installed"}
	}

	logVerbose(opts, "[tier3] %s → using Chrome at %s", rawURL, chromePath)

	l := launcher.New().Bin(chromePath).Headless(true)
	controlURL, err := l.Launch()
	if err != nil {
		logVerbose(opts, "[tier3] %s → Chrome launch failed: %v", rawURL, err)
		return nil, fmt.Errorf("chrome launch: %w", err)
	}

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		logVerbose(opts, "[tier3] %s → browser connect failed: %v", rawURL, err)
		return nil, fmt.Errorf("browser connect: %w", err)
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		logVerbose(opts, "[tier3] %s → page open failed: %v", rawURL, err)
		return nil, fmt.Errorf("browser open page: %w", err)
	}

	if err := page.WaitStable(1 * time.Second); err != nil {
		logVerbose(opts, "[tier3] %s → page did not stabilize: %v", rawURL, err)
	}
	page.MustWaitLoad()

	html, err := page.HTML()
	if err != nil {
		logVerbose(opts, "[tier3] %s → failed to get HTML: %v", rawURL, err)
		return nil, fmt.Errorf("page html: %w", err)
	}

	if isCaptchaPage(html) {
		logVerbose(opts, "[tier3] %s → CAPTCHA detected", rawURL)
		return nil, &httpError{statusCode: 0, message: "site requires interactive challenge (CAPTCHA)"}
	}

	logVerbose(opts, "[tier3] %s → 200 (headless Chrome, %d bytes)", rawURL, len(html))
	return &fetchResult{html: html, tier: 3, url: rawURL}, nil
}

func findChrome() string {
	paths := []string{
		// Linux
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/snap/bin/chromium",
		// Mac
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		// Windows (native)
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		// Windows (via WSL)
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
	}

	if envPath := os.Getenv("CHROME_PATH"); envPath != "" {
		if fileExists(envPath) {
			return envPath
		}
	}

	for _, p := range paths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isCaptchaPage(html string) bool {
	lower := strings.ToLower(html)
	for _, marker := range []string{"turnstile", "cf-challenge", "captcha", "recaptcha", "hcaptcha", "challenge-platform"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
