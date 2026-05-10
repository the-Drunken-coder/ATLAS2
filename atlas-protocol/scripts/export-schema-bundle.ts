import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..');
const schemaDir = path.join(root, 'schemas');
const outputPath = path.join(root, 'generated', 'schema-bundle.json');

const files = [
  'change-event.schema.json',
  'command-catalog.schema.json',
  'entity.schema.json',
  'object.schema.json',
  'observation.schema.json',
  'task.schema.json',
  'validation-error.schema.json',
].sort();

const schemas = [];
for (const file of files) {
  const raw = await readFile(path.join(schemaDir, file), 'utf8');
  schemas.push({
    name: file.replace('.schema.json', ''),
    path: `schemas/${file}`,
    sha256: createHash('sha256').update(raw).digest('hex'),
    schema: JSON.parse(raw),
  });
}

await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(
  outputPath,
  `${JSON.stringify({
    name: 'atlas-protocol',
    schema_dialect: 'https://json-schema.org/draft/2020-12/schema',
    schemas,
  }, null, 2)}\n`,
);
