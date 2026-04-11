#!/usr/bin/env node
import { execFileSync, spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { platform } from "node:os";

const __dirname = dirname(fileURLToPath(import.meta.url));

const os = platform();
const ext = os === "win32" ? ".exe" : "";
const binaryName = `rawdoc${ext}`;

// Check for binary in plugin directory first, then PATH
let binaryPath = resolve(__dirname, binaryName);

if (!existsSync(binaryPath)) {
  // Try to find in PATH
  try {
    const which = os === "win32" ? "where" : "which";
    binaryPath = execFileSync(which, [binaryName], { encoding: "utf8" }).trim().split("\n")[0];
  } catch {
    // Try to build from source if Go is available
    try {
      console.error("[rawdoc] Binary not found, building from source...");
      execFileSync("go", ["build", "-o", resolve(__dirname, binaryName), "."], {
        cwd: __dirname,
        stdio: "inherit",
      });
      binaryPath = resolve(__dirname, binaryName);
    } catch {
      console.error("[rawdoc] Error: rawdoc binary not found and Go is not available to build it.");
      console.error("[rawdoc] Install with: go install github.com/RandomCodeSpace/rawdoc@latest");
      process.exit(1);
    }
  }
}

// Spawn rawdoc in MCP server mode, piping stdio
const child = spawn(binaryPath, ["--serve"], {
  stdio: ["inherit", "inherit", "inherit"],
});

child.on("error", (err) => {
  console.error(`[rawdoc] Failed to start: ${err.message}`);
  process.exit(1);
});

child.on("exit", (code) => {
  process.exit(code ?? 0);
});
