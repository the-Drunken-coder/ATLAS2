import { Buffer } from 'node:buffer';

import type { EntityType, ObjectType, ValidationContext, ValidationIssue } from './types.js';

const MAX_JSON_BLOB_SIZE = 64 * 1024;
const MAX_JSON_DEPTH = 16;
const MAX_JSON_FIELDS = 500;
const MAX_JSON_KEY_LENGTH = 100;
const MAX_CUSTOM_BLOB_SIZE = 16 * 1024;
const MAX_CUSTOM_DEPTH = 8;
const MAX_CUSTOM_FIELDS = 100;
const MAX_CUSTOM_KEY_LENGTH = 100;
const COMMAND_CATALOG_OBJECT_ID = 'command_catalog';

const promotedFields = new Set([
  'entity_id',
  'object_id',
  'task_id',
  'observation_id',
  'type',
  'status',
  'owner_type',
  'owner_id',
  'asset_id',
  'source_asset_id',
  'command_catalog_object_id',
  'created_at',
  'updated_at',
  'version',
]);

const topLevelReservedForCustom = new Set([
  'components',
  'extra',
  'description',
  'created_by',
  'state',
  'latest_sighting',
  'sightings_object_id',
  'log_type',
  'started_at',
  'ended_at',
  'content_type',
  'captured_at',
  'width_px',
  'height_px',
  'manifest',
  'manifest_version',
  'commands',
  'supported_commands',
  'telemetry',
  'geometry',
  'heartbeat',
  'health',
  'communications',
  'sensor_refs',
  'fusion_summary',
  'command',
  'parameters',
  'progress',
  'result',
  'error',
]);

const entityAllowedComponents: Record<EntityType, { required: string[]; optional: string[] }> = {
  asset: {
    required: ['supported_commands'],
    optional: ['telemetry', 'status', 'heartbeat', 'health', 'communications', 'sensor_refs'],
  },
  track: {
    required: ['telemetry'],
    optional: ['status', 'fusion_summary'],
  },
  geofeature: {
    required: ['geometry'],
    optional: ['status'],
  },
};

const topLevelAllowedBySchema = {
  entity: new Set(['components', 'extra']),
  task: new Set(['description', 'created_by', 'components', 'extra']),
  observation: new Set(['state', 'latest_sighting', 'sightings_object_id', 'extra']),
};

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function join(base: string, key: string): string {
  return base ? `${base}.${key}` : key;
}

function add(out: ValidationIssue[], field: string, code: string, message: string): void {
  out.push({ field, code, message });
}

function validateLimits(value: unknown, field: string, out: ValidationIssue[], limits: {
  maxSize: number;
  maxDepth: number;
  maxFields: number;
  maxKeyLength: number;
}): void {
  const size = Buffer.byteLength(JSON.stringify(value));
  if (size > limits.maxSize) {
    add(out, field, 'TOO_LARGE', `must be ${limits.maxSize} bytes or fewer`);
  }
  let fields = 0;
  const walk = (current: unknown, currentField: string, depth: number): void => {
    if (depth > limits.maxDepth) {
      add(out, currentField, 'TOO_DEEP', `must not exceed ${limits.maxDepth} levels`);
      return;
    }
    if (Array.isArray(current)) {
      current.forEach((item, index) => walk(item, `${currentField}[${index}]`, depth + 1));
      return;
    }
    if (!isPlainObject(current)) {
      return;
    }
    const keys = Object.keys(current).sort();
    fields += keys.length;
    if (fields > limits.maxFields) {
      add(out, currentField, 'TOO_MANY_FIELDS', `must not contain more than ${limits.maxFields} object fields`);
      return;
    }
    for (const key of keys) {
      if (key.length > limits.maxKeyLength) {
        add(out, join(currentField, key), 'KEY_TOO_LONG', `key length must be ${limits.maxKeyLength} characters or fewer`);
      }
      walk(current[key], join(currentField, key), depth + 1);
    }
  };
  walk(value, field, 1);
}

function validateCustomSection(path: string, value: unknown, out: ValidationIssue[]): void {
  if (!isPlainObject(value)) {
    add(out, path, 'INVALID_TYPE', 'must be an object');
    return;
  }
  validateLimits(value, path, out, {
    maxSize: MAX_CUSTOM_BLOB_SIZE,
    maxDepth: MAX_CUSTOM_DEPTH,
    maxFields: MAX_CUSTOM_FIELDS,
    maxKeyLength: MAX_CUSTOM_KEY_LENGTH,
  });
  for (const key of Object.keys(value).sort()) {
    if (promotedFields.has(key) || topLevelReservedForCustom.has(key)) {
      add(out, join(path, key), 'CORE_FIELD', 'must not duplicate a promoted field or core section');
    }
  }
}

function validateTopLevelKeys(root: Record<string, unknown>, allowed: Set<string>, out: ValidationIssue[]): void {
  for (const key of Object.keys(root).sort()) {
    if (promotedFields.has(key)) {
      add(out, join('json', key), 'PROMOTED_FIELD', 'must not duplicate a promoted field');
      continue;
    }
    if (key.startsWith('custom_')) {
      validateCustomSection(join('json', key), root[key], out);
      continue;
    }
    if (!allowed.has(key)) {
      add(out, join('json', key), 'UNKNOWN_FIELD', 'is not allowed');
    }
  }
}

function ensureExtra(root: Record<string, unknown>, out: ValidationIssue[]): void {
  if (!('extra' in root)) {
    root.extra = {};
    return;
  }
  if (!isPlainObject(root.extra)) {
    add(out, 'json.extra', 'INVALID_TYPE', 'must be an object');
  }
}

function validateEntityRules(root: Record<string, unknown>, context: ValidationContext, out: ValidationIssue[]): void {
  validateTopLevelKeys(root, topLevelAllowedBySchema.entity, out);
  ensureExtra(root, out);
  const entityType = context.entityType;
  if (!entityType) {
    add(out, 'entity_type', 'INVALID_INPUT', 'entity_type is required for entity validation');
    return;
  }
  const matrix = entityAllowedComponents[entityType];
  const componentsValue = root.components;
  if (!isPlainObject(componentsValue)) {
    if (componentsValue === undefined) {
      add(out, 'json.components', 'REQUIRED', 'is required');
    }
    return;
  }
  const known = new Set([...matrix.required, ...matrix.optional]);
  for (const required of matrix.required) {
    if (!(required in componentsValue)) {
      add(out, join('json.components', required), 'REQUIRED', 'is required');
    }
  }
  for (const key of Object.keys(componentsValue).sort()) {
    if (key.startsWith('custom_')) {
      validateCustomSection(join('json.components', key), componentsValue[key], out);
      continue;
    }
    if (!known.has(key)) {
      add(out, join('json.components', key), 'UNKNOWN_FIELD', 'is not allowed for this entity type');
    }
  }
  const telemetry = componentsValue.telemetry;
  if (isPlainObject(telemetry)) {
    const latitude = telemetry.latitude;
    const longitude = telemetry.longitude;
    const hasLat = typeof latitude === 'number';
    const hasLon = typeof longitude === 'number';
    if (hasLat && (latitude < -90 || latitude > 90)) {
      add(out, 'json.components.telemetry.latitude', 'OUT_OF_RANGE', 'must be between -90 and 90');
    }
    if (hasLon && (longitude < -180 || longitude > 180)) {
      add(out, 'json.components.telemetry.longitude', 'OUT_OF_RANGE', 'must be between -180 and 180');
    }
    if (hasLat !== hasLon) {
      add(out, 'json.components.telemetry', 'INVALID_VALUE', 'latitude and longitude must be provided together');
    }
    if (entityType === 'track' && (!hasLat || !hasLon)) {
      add(out, 'json.components.telemetry', 'REQUIRED', 'latitude and longitude are required');
    }
    if (typeof telemetry.speed_m_s === 'number' && telemetry.speed_m_s < 0) {
      add(out, 'json.components.telemetry.speed_m_s', 'OUT_OF_RANGE', 'must be greater than or equal to 0');
    }
    if (typeof telemetry.heading_deg === 'number' && (telemetry.heading_deg < 0 || telemetry.heading_deg >= 360)) {
      add(out, 'json.components.telemetry.heading_deg', 'OUT_OF_RANGE', 'must be greater than or equal to 0 and less than 360');
    }
    if (typeof telemetry.uncertainty_radius_m === 'number') {
      if (telemetry.uncertainty_radius_m < 0) {
        add(out, 'json.components.telemetry.uncertainty_radius_m', 'OUT_OF_RANGE', 'must be greater than or equal to 0');
      }
      if (!hasLat || !hasLon) {
        add(out, 'json.components.telemetry.uncertainty_radius_m', 'INVALID_VALUE', 'requires latitude and longitude');
      }
    }
  }
  const supported = componentsValue.supported_commands;
  if (isPlainObject(supported) && Array.isArray(supported.commands)) {
    const seen = new Set<string>();
    for (const command of supported.commands) {
      if (typeof command !== 'string') {
        add(out, 'json.components.supported_commands.commands', 'INVALID_TYPE', 'must contain only strings');
        continue;
      }
      if (!command.trim()) {
        add(out, 'json.components.supported_commands.commands', 'INVALID_VALUE', 'must not contain empty strings');
      }
      if (seen.has(command)) {
        add(out, 'json.components.supported_commands.commands', 'INVALID_VALUE', 'must not contain duplicate commands');
      }
      seen.add(command);
    }
  }
  const status = componentsValue.status;
  if (isPlainObject(status) && typeof status.priority === 'number' && status.priority < 0) {
    add(out, 'json.components.status.priority', 'OUT_OF_RANGE', 'must be greater than or equal to 0');
  }
  const heartbeat = componentsValue.heartbeat;
  if (isPlainObject(heartbeat) && typeof heartbeat.sequence === 'number' && heartbeat.sequence < 0) {
    add(out, 'json.components.heartbeat.sequence', 'OUT_OF_RANGE', 'must be greater than or equal to 0');
  }
  const health = componentsValue.health;
  if (isPlainObject(health) && typeof health.battery_percent === 'number' && (health.battery_percent < 0 || health.battery_percent > 100)) {
    add(out, 'json.components.health.battery_percent', 'OUT_OF_RANGE', 'must be between 0 and 100');
  }
  const sensors = isPlainObject(componentsValue.sensor_refs) ? componentsValue.sensor_refs.sensors : undefined;
  if (Array.isArray(sensors)) {
    sensors.forEach((sensor, index) => {
      if (!isPlainObject(sensor) || !isPlainObject(sensor.mount)) {
        return;
      }
      const mount = sensor.mount;
      const base = `json.components.sensor_refs.sensors[${index}].mount`;
      if (typeof mount.bearing_deg === 'number' && (mount.bearing_deg < 0 || mount.bearing_deg >= 360)) {
        add(out, `${base}.bearing_deg`, 'OUT_OF_RANGE', 'must be greater than or equal to 0 and less than 360');
      }
      if (typeof mount.elevation_deg === 'number' && (mount.elevation_deg < -90 || mount.elevation_deg > 90)) {
        add(out, `${base}.elevation_deg`, 'OUT_OF_RANGE', 'must be between -90 and 90');
      }
      if (typeof mount.roll_deg === 'number' && (mount.roll_deg < 0 || mount.roll_deg >= 360)) {
        add(out, `${base}.roll_deg`, 'OUT_OF_RANGE', 'must be greater than or equal to 0 and less than 360');
      }
    });
  }
  const fusionSummary = componentsValue.fusion_summary;
  if (isPlainObject(fusionSummary)) {
    if (typeof fusionSummary.source_count === 'number' && fusionSummary.source_count < 0) {
      add(out, 'json.components.fusion_summary.source_count', 'OUT_OF_RANGE', 'must be greater than or equal to 0');
    }
    if (typeof fusionSummary.confidence === 'number' && (fusionSummary.confidence < 0 || fusionSummary.confidence > 1)) {
      add(out, 'json.components.fusion_summary.confidence', 'OUT_OF_RANGE', 'must be between 0 and 1');
    }
  }
}

function validateTaskRules(root: Record<string, unknown>, out: ValidationIssue[]): void {
  validateTopLevelKeys(root, topLevelAllowedBySchema.task, out);
  ensureExtra(root, out);
}

function validateObservationRules(root: Record<string, unknown>, out: ValidationIssue[]): void {
  validateTopLevelKeys(root, topLevelAllowedBySchema.observation, out);
  ensureExtra(root, out);
}

function validateObjectRules(root: Record<string, unknown>, context: ValidationContext, out: ValidationIssue[]): void {
  ensureExtra(root, out);
  const objectType = context.objectType;
  if (!objectType) {
    add(out, 'object_type', 'INVALID_INPUT', 'object_type is required for object validation');
    return;
  }
  const allowed = new Set(['extra']);
  if (objectType === 'log') {
    allowed.add('log_type');
    allowed.add('started_at');
    allowed.add('ended_at');
  }
  if (objectType === 'photo') {
    allowed.add('content_type');
    allowed.add('captured_at');
    allowed.add('width_px');
    allowed.add('height_px');
  }
  if (objectType === 'document') {
    allowed.add('content_type');
    if (context.objectId === COMMAND_CATALOG_OBJECT_ID) {
      allowed.add('commands');
    }
  }
  for (const key of Object.keys(root).sort()) {
    if (promotedFields.has(key)) {
      add(out, join('json', key), 'PROMOTED_FIELD', 'must not duplicate a promoted field');
      continue;
    }
    if (key === 'manifest' || key === 'manifest_version') {
      add(out, join('json', key), 'RESERVED_FIELD', 'is reserved');
      continue;
    }
    if (key.startsWith('custom_')) {
      validateCustomSection(join('json', key), root[key], out);
      continue;
    }
    if (!allowed.has(key)) {
      add(out, join('json', key), 'UNKNOWN_FIELD', 'is not allowed');
    }
  }
  if (typeof root.width_px === 'number' && root.width_px <= 0) {
    add(out, 'json.width_px', 'OUT_OF_RANGE', 'must be a positive integer');
  }
  if (typeof root.height_px === 'number' && root.height_px <= 0) {
    add(out, 'json.height_px', 'OUT_OF_RANGE', 'must be a positive integer');
  }
  if (objectType === 'document' && context.objectId === COMMAND_CATALOG_OBJECT_ID) {
    if (!('commands' in root)) {
      add(out, 'json.commands', 'REQUIRED', 'is required');
    } else if (!isPlainObject(root.commands)) {
      add(out, 'json.commands', 'INVALID_TYPE', 'must be an object');
    }
  }
}

export function applyAtlasRules(
  schemaName: string,
  root: Record<string, unknown>,
  rawJson: string,
  context: ValidationContext,
): ValidationIssue[] {
  const out: ValidationIssue[] = [];
  validateLimits(root, 'json', out, {
    maxSize: MAX_JSON_BLOB_SIZE,
    maxDepth: MAX_JSON_DEPTH,
    maxFields: MAX_JSON_FIELDS,
    maxKeyLength: MAX_JSON_KEY_LENGTH,
  });
  if (Buffer.byteLength(rawJson) > MAX_JSON_BLOB_SIZE) {
    add(out, 'json', 'TOO_LARGE', `must be ${MAX_JSON_BLOB_SIZE} bytes or fewer`);
  }
  switch (schemaName) {
    case 'entity':
      validateEntityRules(root, context, out);
      break;
    case 'task':
      validateTaskRules(root, out);
      break;
    case 'observation':
      validateObservationRules(root, out);
      break;
    case 'object':
      validateObjectRules(root, context, out);
      break;
    default:
      break;
  }
  return out;
}
