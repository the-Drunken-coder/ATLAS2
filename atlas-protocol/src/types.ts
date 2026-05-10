export type SchemaName =
  | 'entity'
  | 'object'
  | 'task'
  | 'observation'
  | 'command-catalog'
  | 'change-event'
  | 'validation-error';

export type Operation = 'create' | 'update' | 'upsert';

export type EntityType = 'asset' | 'track' | 'geofeature';
export type ObjectType = 'document' | 'log' | 'photo';

export interface ValidationIssue {
  field: string;
  code: string;
  message: string;
}

export interface ValidationContext {
  operation?: Operation;
  entityType?: EntityType;
  objectType?: ObjectType;
  objectId?: string;
}

export interface ValidationSuccess<T = unknown> {
  ok: true;
  value: T;
  normalized: string;
  errors: [];
}

export interface ValidationFailure {
  ok: false;
  errors: ValidationIssue[];
}

export type ValidationResult<T = unknown> = ValidationSuccess<T> | ValidationFailure;
