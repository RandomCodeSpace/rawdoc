#!/usr/bin/env node
// postinstall: build rawdoc binary if Go is available, otherwise check PATH
import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { platform } from "node:os";

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = resolve(__dirname, "..");
const ext = platform() === "win32" ? ".exe" : "";
const binary = resolve(root, `rawdoc${ext}`);

if (existsSync(binary)) {
  console.log(`[rawdoc] Binary already exists: ${binary}`);
  process.exit(0);
}

// Try building from source
try {
  console.log("[rawdoc] Building from source...");
  execFileSync("go", ["build", "-ldflags=-s -w", "-o", binary, "."], {
    cwd: root,
    stdio: "inherit",
  });
  console.log(`[rawdoc] Built: ${binary}`);
  process.exit(0);
} catch {
  // Go not available — check if rawdoc is already in PATH
  try {
    const which = platform() === "win32" ? "where" : "which";
    const found = execFileSync(which, [`rawdoc${ext}`], { encoding: "utf8" }).trim();
    console.log(`[rawdoc] Found in PATH: ${found}`);
    process.exit(0);
  } catch {
    console.error("[rawdoc] Warning: Go not available and rawdoc not in PATH.");
    console.error("[rawdoc] Install manually: go install github.com/RandomCodeSpace/rawdoc@latest");
    // Don't fail postinstall — let start.mjs give the error at runtime
    process.exit(0);
  }
}
