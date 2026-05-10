import entitySchema from '../schemas/entity.schema.json' with { type: 'json' };
import objectSchema from '../schemas/object.schema.json' with { type: 'json' };
import taskSchema from '../schemas/task.schema.json' with { type: 'json' };
import observationSchema from '../schemas/observation.schema.json' with { type: 'json' };
import commandCatalogSchema from '../schemas/command-catalog.schema.json' with { type: 'json' };
import changeEventSchema from '../schemas/change-event.schema.json' with { type: 'json' };
import validationErrorSchema from '../schemas/validation-error.schema.json' with { type: 'json' };

import type { SchemaName } from './types.js';

export const schemas = {
  'entity': entitySchema,
  'object': objectSchema,
  'task': taskSchema,
  'observation': observationSchema,
  'command-catalog': commandCatalogSchema,
  'change-event': changeEventSchema,
  'validation-error': validationErrorSchema,
} as const satisfies Record<SchemaName, object>;

export type SchemaMap = typeof schemas;
