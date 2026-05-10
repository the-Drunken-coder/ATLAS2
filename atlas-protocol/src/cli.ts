import { readFileSync } from 'node:fs';
import process from 'node:process';

import { validateJson } from './validate.js';
import type { EntityType, ObjectType, Operation, SchemaName } from './types.js';

interface CliRequest {
  schema: SchemaName;
  context?: {
    operation?: Operation;
    entityType?: EntityType;
    objectType?: ObjectType;
    objectId?: string;
  };
  json: string;
}

function fail(message: string): never {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}

function parseRequest(argv: string[]): CliRequest {
  if (argv[0] === '--request') {
    const raw = argv[1];
    if (!raw) {
      fail('--request requires a JSON argument');
    }
    return JSON.parse(raw) as CliRequest;
  }
  const args = new Map<string, string>();
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    const value = argv[index + 1];
    if (!key?.startsWith('--') || value === undefined) {
      fail('expected --flag value pairs');
    }
    args.set(key.slice(2), value);
  }
  const schema = args.get('schema') as SchemaName | undefined;
  if (!schema) {
    fail('--schema is required');
  }
  const file = args.get('file');
  const json = file ? readFileSync(file, 'utf8') : readFileSync(0, 'utf8');
  return {
    schema,
    context: {
      operation: args.get('operation') as Operation | undefined,
      entityType: args.get('entity-type') as EntityType | undefined,
      objectType: args.get('object-type') as ObjectType | undefined,
      objectId: args.get('object-id') ?? undefined,
    },
    json,
  };
}

const request = parseRequest(process.argv.slice(2));
const result = validateJson(request.schema, request.json, request.context);
process.stdout.write(`${JSON.stringify(result)}\n`);
process.exit(result.ok ? 0 : 2);
