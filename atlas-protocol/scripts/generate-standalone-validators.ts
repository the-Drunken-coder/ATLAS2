import { mkdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import standaloneCode from 'ajv/dist/standalone/index.js';
import { build } from 'esbuild';

import { schemas } from '../src/schemas.js';

const root = path.resolve(import.meta.dirname, '..');
const outputDir = path.join(root, 'generated', 'validators');
const tempDir = path.join(root, '.atlas-protocol-validators-tmp');
await rm(tempDir, { recursive: true, force: true });
await mkdir(tempDir, { recursive: true });
await rm(outputDir, { recursive: true, force: true });
await mkdir(outputDir, { recursive: true });

const imports: string[] = [];
const mappings: string[] = [];

try {
  for (const [name, schema] of Object.entries(schemas)) {
    const ajv = new Ajv2020({ allErrors: true, code: { esm: true, source: true }, strict: false });
    addFormats(ajv);
    const validate = ajv.compile(schema);
    const fileBase = name.replace(/[^a-z0-9-]/gi, '-');
    const exportName = name.replace(/-([a-z])/g, (_, letter: string) => letter.toUpperCase());
    await writeFile(path.join(tempDir, `${fileBase}.mjs`), standaloneCode(ajv, validate));
    imports.push(`import ${exportName} from './${fileBase}.mjs';`);
    mappings.push(`  '${name}': ${exportName},`);
  }

  const indexModule = `${imports.join('\n')}\n\nconst atlasProtocolValidators = {\n${mappings.join('\n')}\n};\n\nexport const validators = atlasProtocolValidators;\nexport default atlasProtocolValidators;\n`;
  const tempEntry = path.join(tempDir, 'index.mjs');
  await writeFile(tempEntry, indexModule);

  await build({
    absWorkingDir: tempDir,
    entryPoints: ['index.mjs'],
    outfile: path.join(outputDir, 'index.mjs'),
    bundle: true,
    format: 'esm',
    platform: 'node',
    target: 'node20',
    nodePaths: [path.join(root, 'node_modules')],
  });
} finally {
  await rm(tempDir, { recursive: true, force: true });
}
