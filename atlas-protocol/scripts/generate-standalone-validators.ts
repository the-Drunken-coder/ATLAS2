import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import standaloneCode from 'ajv/dist/standalone/index.js';

import { schemas } from '../src/schemas.js';

const ajv = new Ajv2020({ allErrors: true, code: { esm: true, source: true }, strict: false });
addFormats(ajv);
for (const [name, schema] of Object.entries(schemas)) {
  ajv.addSchema(schema, name);
}
const chunks: string[] = [];
const exportsMap: string[] = [];
for (const name of Object.keys(schemas)) {
  const validate = ajv.getSchema(name);
  if (!validate) {
    throw new Error(`missing validator for ${name}`);
  }
  const exportName = name.replace(/-/g, '_');
  const code = standaloneCode(ajv, validate)
    .replace(/export const validate/g, `const ${exportName}`)
    .replace(/export default validate;?/g, '');
  chunks.push(code);
  exportsMap.push(`  '${name}': ${exportName}`);
}
const moduleCode = `${chunks.join('\n\n')}\n\nexport const validators = {\n${exportsMap.join(',\n')}\n};\n`;
const root = path.resolve(import.meta.dirname, '..');
const outputPath = path.join(root, 'generated', 'validators', 'index.mjs');
await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(outputPath, moduleCode);
