import fs from "fs";
import path from "path";

function findAncestorWithPaths(start: string, requiredPaths: string[]): string | undefined {
  let current = path.resolve(start);
  while (true) {
    if (requiredPaths.every((requiredPath) => fs.existsSync(path.join(current, requiredPath)))) {
      return current;
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return undefined;
    }
    current = parent;
  }
}

function assertExistingDirectory(dirPath: string, label: string): string {
  const resolvedPath = path.resolve(dirPath);
  let stat: fs.Stats;
  try {
    stat = fs.statSync(resolvedPath);
  } catch {
    throw new Error(`${label} does not exist: ${resolvedPath}`);
  }
  if (!stat.isDirectory()) {
    throw new Error(`${label} is not a directory: ${resolvedPath}`);
  }
  return resolvedPath;
}

export function resolveAtlasProtocolPackageRoot(): string {
  for (const start of [__dirname, process.cwd()]) {
    const found = findAncestorWithPaths(start, ["package.json", path.join("source", "schemas")]);
    if (found !== undefined) {
      return found;
    }
  }
  throw new Error("unable to locate atlas-protocol package root");
}

export function resolveAtlasProtocolRuntimeRoot(): string {
  if (process.env.ATLAS_PROTOCOL_ROOT) {
    return assertExistingDirectory(process.env.ATLAS_PROTOCOL_ROOT, "ATLAS_PROTOCOL_ROOT");
  }
  return resolveAtlasProtocolPackageRoot();
}

export function resolveRepoRoot(atlasProtocolRoot: string): string {
  for (const start of [atlasProtocolRoot, path.dirname(atlasProtocolRoot), process.cwd()]) {
    const found = findAncestorWithPaths(start, ["atlas.py", "atlas-core"]);
    if (found !== undefined) {
      return found;
    }
  }
  return path.resolve(atlasProtocolRoot, "..");
}
