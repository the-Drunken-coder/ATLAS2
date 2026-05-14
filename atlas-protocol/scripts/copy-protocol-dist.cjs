"use strict";

const fs = require("fs");
const path = require("path");

const LOCK_WAIT_TIMEOUT_MS = 30_000;
const LOCK_WAIT_INTERVAL_MS = 100;
const STALE_LOCK_AGE_MS = 10 * 60 * 1000;

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

function sleepMs(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function ownerMetadataPath(lockDir) {
  return path.join(lockDir, "owner.json");
}

function readLockOwner(lockDir) {
  const ownerPath = ownerMetadataPath(lockDir);
  if (!fs.existsSync(ownerPath)) {
    return null;
  }
  try {
    return JSON.parse(fs.readFileSync(ownerPath, "utf8"));
  } catch {
    return null;
  }
}

function writeLockOwner(lockDir) {
  const owner = {
    pid: process.pid,
    acquiredAtMs: Date.now(),
  };
  fs.writeFileSync(ownerMetadataPath(lockDir), JSON.stringify(owner, null, 2));
}

function isPidRunning(pid) {
  if (!Number.isInteger(pid) || pid <= 0) {
    return false;
  }
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    if (err && (err.code === "ESRCH" || err.code === "EPERM")) {
      return err.code === "EPERM";
    }
    return false;
  }
}

function lockAgeMs(lockDir) {
  try {
    return Date.now() - fs.statSync(lockDir).mtimeMs;
  } catch {
    return Number.NaN;
  }
}

function shouldClearStaleLock(lockDir, owner) {
  if (!owner || typeof owner !== "object") {
    return lockAgeMs(lockDir) > STALE_LOCK_AGE_MS;
  }
  const pid = Number(owner.pid);
  const acquiredAtMs = Number(owner.acquiredAtMs);
  if (Number.isInteger(pid) && pid > 0 && isPidRunning(pid)) {
    return false;
  }
  const ageMs = Number.isFinite(acquiredAtMs) ? Date.now() - acquiredAtMs : lockAgeMs(lockDir);
  return Number.isFinite(ageMs) && ageMs > STALE_LOCK_AGE_MS;
}

function acquireLock(lockDir) {
  const deadline = Date.now() + LOCK_WAIT_TIMEOUT_MS;
  while (true) {
    try {
      fs.mkdirSync(lockDir);
      writeLockOwner(lockDir);
      return;
    } catch (err) {
      if (!err || err.code !== "EEXIST") {
        throw err;
      }
      const owner = readLockOwner(lockDir);
      if (shouldClearStaleLock(lockDir, owner)) {
        try {
          fs.rmSync(lockDir, { recursive: true, force: true });
        } catch {
          // another waiter may clear it first
        }
        continue;
      }
      if (Date.now() >= deadline) {
        const ownerJson = owner ? JSON.stringify(owner) : "unknown";
        throw new Error(
          `timed out waiting for lock ${lockDir}; owner=${ownerJson}`,
        );
      }
      sleepMs(LOCK_WAIT_INTERVAL_MS);
    }
  }
}

function releaseLock(lockDir) {
  try {
    fs.rmSync(lockDir, { recursive: true, force: true });
  } catch {
    // best-effort cleanup
  }
}

function cleanupStaleTempDirs(distRoot, activePaths) {
  if (!fs.existsSync(distRoot)) {
    return;
  }
  const active = new Set(activePaths);
  const entries = fs.readdirSync(distRoot, { withFileTypes: true });
  for (const entry of entries) {
    if (!entry.isDirectory()) {
      continue;
    }
    const name = entry.name;
    if (!name.startsWith("protocol.staging.") && !name.startsWith("protocol.prev.")) {
      continue;
    }
    const fullPath = path.join(distRoot, name);
    if (active.has(fullPath)) {
      continue;
    }
    try {
      fs.rmSync(fullPath, { recursive: true, force: true });
    } catch {
      // best-effort cleanup
    }
  }
}

function main() {
  const atlasProtocolRoot = path.resolve(__dirname, "..");
  const distRoot = path.join(atlasProtocolRoot, "dist");
  const lockDir = path.join(distRoot, "protocol.lock");
  const distProtocol = path.join(distRoot, "protocol");
  const sourceRoot = path.join(atlasProtocolRoot, "source");
  const examplesDir = path.join(atlasProtocolRoot, "examples");
  const staging = path.join(distRoot, `protocol.staging.${process.pid}`);
  const prev = path.join(distRoot, `protocol.prev.${process.pid}`);

  if (!fs.existsSync(examplesDir)) {
    console.error(`[atlas-protocol] copy-protocol-dist: run sync-examples first; missing ${examplesDir}`);
    process.exit(1);
  }
  fs.mkdirSync(distRoot, { recursive: true });

  acquireLock(lockDir);
  try {
    // Serialized via lock directory so only one writer mutates dist/protocol at a time.
    cleanupStaleTempDirs(distRoot, [staging, prev]);

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

    if (fs.existsSync(distProtocol)) {
      fs.renameSync(distProtocol, prev);
    }
    try {
      fs.renameSync(staging, distProtocol);
    } catch (err) {
      if (fs.existsSync(prev) && !fs.existsSync(distProtocol)) {
        fs.renameSync(prev, distProtocol);
      }
      throw err;
    }
    fs.rmSync(prev, { recursive: true, force: true });
    console.log(`[atlas-protocol] wrote standalone bundle to ${distProtocol}`);
  } finally {
    releaseLock(lockDir);
  }
}

main();
