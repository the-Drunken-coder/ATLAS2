import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { validateJson } from './validate.js';

const root = path.resolve(import.meta.dirname, '..');
const examplesDir = path.join(root, 'examples');

async function loadExample(name: string): Promise<Record<string, unknown>> {
  const raw = await readFile(path.join(examplesDir, name), 'utf8');
  return JSON.parse(raw) as Record<string, unknown>;
}

test('valid protocol examples pass', async () => {
  const assets = await loadExample('assets.json');
  const tracks = await loadExample('tracks.json');
  const geofeatures = await loadExample('geofeatures.json');
  const tasks = await loadExample('tasks.json');
  const observations = await loadExample('observations.json');
  const documents = await loadExample('objects-document.json');
  const logs = await loadExample('objects-log.json');
  const photos = await loadExample('objects-photo.json');
  const commandCatalog = await loadExample('command-catalog.json');
  const changeEvent = await loadExample('change-event.json');
  const validationError = await loadExample('validation-error.json');

  assert.equal(validateJson('entity', JSON.stringify(assets.minimum), { entityType: 'asset' }).ok, true);
  assert.equal(validateJson('entity', JSON.stringify(assets.full), { entityType: 'asset' }).ok, true);
  assert.equal(validateJson('entity', JSON.stringify(tracks.minimum), { entityType: 'track' }).ok, true);
  assert.equal(validateJson('entity', JSON.stringify(tracks.full), { entityType: 'track' }).ok, true);
  assert.equal(validateJson('entity', JSON.stringify(geofeatures.minimum), { entityType: 'geofeature' }).ok, true);
  assert.equal(validateJson('entity', JSON.stringify(geofeatures.full), { entityType: 'geofeature' }).ok, true);
  assert.equal(validateJson('task', JSON.stringify(tasks.minimum)).ok, true);
  assert.equal(validateJson('task', JSON.stringify(tasks.full_success)).ok, true);
  assert.equal(validateJson('task', JSON.stringify(tasks.full_error)).ok, true);
  assert.equal(validateJson('observation', JSON.stringify(observations.minimum)).ok, true);
  assert.equal(validateJson('observation', JSON.stringify(observations.full)).ok, true);
  assert.equal(validateJson('object', JSON.stringify(documents.minimum), { objectType: 'document', objectId: 'briefing-001' }).ok, true);
  assert.equal(validateJson('object', JSON.stringify(documents.full), { objectType: 'document', objectId: 'briefing-001' }).ok, true);
  assert.equal(validateJson('object', JSON.stringify(logs.minimum), { objectType: 'log', objectId: 'log-001' }).ok, true);
  assert.equal(validateJson('object', JSON.stringify(logs.full), { objectType: 'log', objectId: 'log-001' }).ok, true);
  assert.equal(validateJson('object', JSON.stringify(photos.minimum), { objectType: 'photo', objectId: 'photo-001' }).ok, true);
  assert.equal(validateJson('object', JSON.stringify(photos.full), { objectType: 'photo', objectId: 'photo-001' }).ok, true);
  assert.equal(validateJson('command-catalog', JSON.stringify(commandCatalog.minimum)).ok, true);
  assert.equal(validateJson('command-catalog', JSON.stringify(commandCatalog.full)).ok, true);
  assert.equal(validateJson('object', JSON.stringify(commandCatalog.minimum), { objectType: 'document', objectId: 'command_catalog' }).ok, true);
  assert.equal(validateJson('change-event', JSON.stringify(changeEvent.create)).ok, true);
  assert.equal(validateJson('change-event', JSON.stringify(changeEvent.update)).ok, true);
  assert.equal(validateJson('change-event', JSON.stringify(changeEvent.delete)).ok, true);
  assert.equal(validateJson('change-event', JSON.stringify(changeEvent.manifest_sync)).ok, true);
  assert.equal(validateJson('validation-error', JSON.stringify(validationError.single)).ok, true);
});

test('invalid entity payloads fail with field-targeted errors', () => {
  const result = validateJson('entity', JSON.stringify({ components: { telemetry: { latitude: 10 } } }), { entityType: 'track' });
  assert.equal(result.ok, false);
  assert.deepEqual(result.errors.map((issue) => issue.field), ['json.components.telemetry', 'json.components.telemetry']);
});

test('reserved object fields fail', () => {
  const result = validateJson('object', JSON.stringify({ manifest: {} }), { objectType: 'document', objectId: 'doc-001' });
  assert.equal(result.ok, false);
  assert.equal(result.errors[0]?.field, 'json.manifest');
});

test('task custom sections cannot shadow core keys', () => {
  const result = validateJson('task', JSON.stringify({
    components: {
      command: { type: 'move_to_location' },
      parameters: {},
    },
    custom_vendor: {
      command: true,
    },
  }));
  assert.equal(result.ok, false);
  assert.equal(result.errors[0]?.field, 'json.custom_vendor.command');
});

test('command catalog requires parameters_schema', () => {
  const result = validateJson('command-catalog', JSON.stringify({ commands: { move_to_location: {} } }));
  assert.equal(result.ok, false);
  assert.equal(result.errors[0]?.field, 'json.commands.move_to_location.parameters_schema');
});

test('command catalog document object requires commands but normal documents do not', () => {
  const missingCommands = validateJson('object', '{}', { objectType: 'document', objectId: 'command_catalog' });
  assert.equal(missingCommands.ok, false);
  assert.equal(missingCommands.errors[0]?.field, 'json.commands');

  const catalogDocument = validateJson('object', '{"commands":{}}', { objectType: 'document', objectId: 'command_catalog' });
  assert.equal(catalogDocument.ok, true);

  const normalDocument = validateJson('object', '{}', { objectType: 'document', objectId: 'doc-123' });
  assert.equal(normalDocument.ok, true);
});

test('normalization adds extra and sorts keys', () => {
  const result = validateJson('task', '{"components":{"parameters":{},"command":{"type":"move_to_location"}}}');
  assert.equal(result.ok, true);
  assert.equal(result.normalized, '{"components":{"command":{"type":"move_to_location"},"parameters":{}},"extra":{}}\n');
});
