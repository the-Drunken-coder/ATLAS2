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

// Build into a staging directory, then swap into place to shrink the window where
// concurrent builds see a half-written dist/protocol. True parallel multi-writer
// safety still assumes a single writer; this avoids common ENOTEMPTY/partial-tree races.
const staging = path.join(atlasProtocolRoot, "dist", `protocol.staging.${process.pid}`);
const prev = path.join(atlasProtocolRoot, "dist", `protocol.prev.${process.pid}`);

fs.rmSync(staging, { recursive: true, force: true });
fs.rmSync(prev, { recursive: true, force: true });
fs.mkdirSync(path.join(staging, "source"), { recursive: true });

for (const sub of ["schemas", "manifests", "goldens"]) {
  copyDirRecursive(path.join(sourceRoot, sub), path.join(staging, "source", sub));
}

const distExamples = path.join(staging, "examples");
fs.mkdirSync(distExamples, { recursive: true });
for (const name of fs.readdirSync(examplesDir)) {
  if (name.endsWith(".json")) {
    fs.copyFileSync(path.join(examplesDir, name), path.join(distExamples, name));
  }
}

try {
  if (fs.existsSync(distProtocol)) {
    fs.renameSync(distProtocol, prev);
  }
} catch (err) {
  fs.rmSync(staging, { recursive: true, force: true });
  throw err;
}

try {
  fs.renameSync(staging, distProtocol);
} catch (err) {
  try {
    if (fs.existsSync(prev) && !fs.existsSync(distProtocol)) {
      fs.renameSync(prev, distProtocol);
    }
  } catch {
    // best-effort rollback
  }
  throw err;
}

fs.rmSync(prev, { recursive: true, force: true });

console.log(`[atlas-protocol] wrote standalone bundle to ${distProtocol}`);
