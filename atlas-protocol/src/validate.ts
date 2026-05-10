import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

import { normalizeAjvErrors, normalizeIssues } from './errors.js';
import { applyAtlasRules } from './atlas-rules.js';
import { schemas } from './schemas.js';
import type { SchemaName, ValidationContext, ValidationIssue, ValidationResult } from './types.js';

const ajv = new Ajv2020({
  allErrors: true,
  strict: false,
  validateSchema: true,
});
addFormats(ajv);
for (const schema of Object.values(schemas)) {
  ajv.addSchema(schema);
}

const validators = {
  entity: ajv.compile(schemas.entity),
  object: ajv.compile(schemas.object),
  task: ajv.compile(schemas.task),
  observation: ajv.compile(schemas.observation),
  'command-catalog': ajv.compile(schemas['command-catalog']),
  'change-event': ajv.compile(schemas['change-event']),
  'validation-error': ajv.compile(schemas['validation-error']),
} as const;

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(stableValue);
  }
  if (!value || typeof value !== 'object') {
    return value;
  }
  const entries = Object.entries(value as Record<string, unknown>)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, child]) => [key, stableValue(child)]);
  return Object.fromEntries(entries);
}

function parseRoot(raw: string): ValidationResult<Record<string, unknown>> {
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { ok: false, errors: [{ field: 'json', code: 'INVALID_TYPE', message: 'must be a JSON object' }] };
    }
    return { ok: true, value: parsed as Record<string, unknown>, normalized: '', errors: [] };
  } catch {
    return { ok: false, errors: [{ field: 'json', code: 'INVALID_JSON', message: 'must be valid JSON' }] };
  }
}

export function validateJson(
  schemaName: SchemaName,
  rawJson: string,
  context: ValidationContext = {},
): ValidationResult<Record<string, unknown>> {
  const parsed = parseRoot(rawJson);
  if (!parsed.ok) {
    return parsed;
  }
  const root = structuredClone(parsed.value);
  if (!(schemaName in validators)) {
    return {
      ok: false,
      errors: [{ field: 'schema', code: 'INVALID_SCHEMA', message: `unsupported schema: ${schemaName}` }],
    };
  }
  const validate = validators[schemaName as keyof typeof validators];
  const ok = validate(root);
  const errors: ValidationIssue[] = [];
  if (!ok) {
    errors.push(...normalizeAjvErrors(validate.errors));
  }
  errors.push(...applyAtlasRules(schemaName, root, rawJson, context));
  const normalizedErrors = normalizeIssues(errors);
  if (normalizedErrors.length > 0) {
    return { ok: false, errors: normalizedErrors };
  }
  const stable = stableValue(root) as Record<string, unknown>;
  return {
    ok: true,
    value: stable,
    normalized: `${JSON.stringify(stable)}\n`,
    errors: [],
  };
}
