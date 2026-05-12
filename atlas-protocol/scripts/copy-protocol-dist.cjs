"use strict";

const fs = require("fs");
const path = require("path");

function copyDirRecursive(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const from = path.join(src, entry.name);
    const to = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDirRecursive(from, to);
    } else {
      fs.copyFileSync(from, to);
    }
  }
}

const atlasProtocolRoot = path.resolve(__dirname, "..");
const distProtocol = path.join(atlasProtocolRoot, "dist", "protocol");
const sourceRoot = path.join(atlasProtocolRoot, "source");
const examplesDir = path.join(atlasProtocolRoot, "examples");

if (!fs.existsSync(examplesDir)) {
  console.error(`[atlas-protocol] copy-protocol-dist: run sync-examples first; missing ${examplesDir}`);
  process.exit(1);
}

fs.rmSync(distProtocol, { recursive: true, force: true });
fs.mkdirSync(path.join(distProtocol, "source"), { recursive: true });

for (const sub of ["schemas", "manifests", "goldens"]) {
  copyDirRecursive(path.join(sourceRoot, sub), path.join(distProtocol, "source", sub));
}

const distExamples = path.join(distProtocol, "examples");
fs.mkdirSync(distExamples, { recursive: true });
for (const name of fs.readdirSync(examplesDir)) {
  if (name.endsWith(".json")) {
    fs.copyFileSync(path.join(examplesDir, name), path.join(distExamples, name));
  }
}

console.log(`[atlas-protocol] wrote standalone bundle to ${distProtocol}`);
