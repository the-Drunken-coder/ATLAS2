import type { ErrorObject } from 'ajv';
import type { ValidationIssue } from './types.js';

function instancePathToField(instancePath: string): string {
  if (!instancePath) {
    return 'json';
  }
  const decoded = instancePath
    .split('/')
    .slice(1)
    .map((segment) => segment.replace(/~1/g, '/').replace(/~0/g, '~'));
  return `json.${decoded.join('.')}`;
}

function keywordCode(error: ErrorObject): string {
  switch (error.keyword) {
    case 'required':
      return 'REQUIRED';
    case 'additionalProperties':
      return 'UNKNOWN_FIELD';
    case 'type':
      return 'INVALID_TYPE';
    case 'enum':
    case 'const':
      return 'INVALID_VALUE';
    case 'minimum':
    case 'maximum':
    case 'exclusiveMinimum':
    case 'exclusiveMaximum':
      return 'OUT_OF_RANGE';
    case 'minLength':
    case 'pattern':
    case 'format':
      return 'INVALID_VALUE';
    default:
      return error.keyword.toUpperCase();
  }
}

function messageFor(error: ErrorObject): string {
  if (error.keyword === 'required') {
    return 'is required';
  }
  if (error.keyword === 'additionalProperties') {
    return 'is not allowed';
  }
  return error.message ?? 'is invalid';
}

export function normalizeAjvErrors(errors: ErrorObject[] | null | undefined): ValidationIssue[] {
  if (!errors?.length) {
    return [];
  }
  return errors
    .map((error) => {
      if (error.keyword === 'required') {
        const missing = String(error.params.missingProperty ?? '');
        return {
          field: `${instancePathToField(error.instancePath)}.${missing}`,
          code: keywordCode(error),
          message: messageFor(error),
        } satisfies ValidationIssue;
      }
      if (error.keyword === 'additionalProperties') {
        const additional = String(error.params.additionalProperty ?? '');
        return {
          field: `${instancePathToField(error.instancePath)}.${additional}`,
          code: keywordCode(error),
          message: messageFor(error),
        } satisfies ValidationIssue;
      }
      return {
        field: instancePathToField(error.instancePath),
        code: keywordCode(error),
        message: messageFor(error),
      } satisfies ValidationIssue;
    })
    .sort((left, right) => {
      if (left.field !== right.field) {
        return left.field.localeCompare(right.field);
      }
      if (left.code !== right.code) {
        return left.code.localeCompare(right.code);
      }
      return left.message.localeCompare(right.message);
    });
}

export function normalizeIssues(errors: ValidationIssue[]): ValidationIssue[] {
  const sorted = [...errors].sort((left, right) => {
    if (left.field !== right.field) {
      return left.field.localeCompare(right.field);
    }
    if (left.code !== right.code) {
      return left.code.localeCompare(right.code);
    }
    return left.message.localeCompare(right.message);
  });
  return sorted.filter((issue, index) => (
    index === 0
    || issue.field !== sorted[index - 1]?.field
    || issue.code !== sorted[index - 1]?.code
    || issue.message !== sorted[index - 1]?.message
  ));
}
