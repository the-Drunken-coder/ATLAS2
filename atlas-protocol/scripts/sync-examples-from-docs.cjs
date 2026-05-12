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
for (const name of fs.readdirSync(srcDir)) {
  if (!name.endsWith(".json")) {
    continue;
  }
  fs.copyFileSync(path.join(srcDir, name), path.join(destDir, name));
}
console.log(`[atlas-protocol] synced examples from docs to ${destDir}`);
