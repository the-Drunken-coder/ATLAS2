"use strict";

const fs = require("fs");
const path = require("path");

const atlasProtocolRoot = path.resolve(__dirname, "..");
const repoRoot = path.resolve(atlasProtocolRoot, "..");
const srcDir = path.join(repoRoot, "docs", "atlas-protocol", "examples");
const destDir = path.join(atlasProtocolRoot, "examples");

if (!fs.existsSync(srcDir)) {
  console.error(`[atlas-protocol] sync-examples: missing ${srcDir}`);
  process.exit(1);
}

fs.mkdirSync(destDir, { recursive: true });
const sourceJsonFiles = new Set(
  fs.readdirSync(srcDir).filter((name) => name.endsWith(".json")),
);
for (const name of fs.readdirSync(destDir)) {
  if (name.endsWith(".json") && !sourceJsonFiles.has(name)) {
    fs.rmSync(path.join(destDir, name), { force: true });
  }
}
for (const name of sourceJsonFiles) {
  if (!name.endsWith(".json")) {
    continue;
  }
  fs.copyFileSync(path.join(srcDir, name), path.join(destDir, name));
}
console.log(`[atlas-protocol] synced examples from docs to ${destDir}`);
