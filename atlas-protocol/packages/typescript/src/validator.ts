import fs from "fs";
import path from "path";
import Ajv2020, {
  type AnySchema,
  type ErrorObject,
  type ValidateFunction,
} from "ajv/dist/2020";
import addFormats from "ajv-formats";

export type ValidationIssue = {
  field: string;
  code: string;
  message: string;
};

export type ResourceKind =
  | "entity"
  | "task"
  | "observation"
  | "object"
  | "commandCatalog"
  | "customSection";

export type ValidExampleCase = {
  id: string;
  resource: ResourceKind;
  source: string;
  example: string;
  variant?: string;
};

export type ValidExampleManifest = {
  cases: ValidExampleCase[];
};

export type InvalidCase = {
  id: string;
  resource: Exclude<ResourceKind, "customSection">;
  source: string;
  variant?: string;
  expected: ValidationIssue[];
};

export type InvalidCaseManifest = {
  cases: InvalidCase[];
};

type JsonObject = Record<string, unknown>;

type VariantValidators = Record<string, ValidateFunction>;

const PROMOTED_FIELDS = new Set([
  "entity_id",
  "object_id",
  "task_id",
  "observation_id",
  "type",
  "status",
  "owner_type",
  "owner_id",
  "asset_id",
  "source_asset_id",
  "command_catalog_object_id",
  "created_at",
  "updated_at",
  "version",
]);

const OBJECT_RESERVED_FIELDS = new Set(["manifest", "manifest_version"]);

const ROOT_LIMITS = {
  maxBytes: 64 * 1024,
  maxDepth: 16,
  maxFields: 500,
  maxKeyLength: 100,
};

const CUSTOM_LIMITS = {
  maxBytes: 16 * 1024,
  maxDepth: 8,
  maxFields: 100,
  maxKeyLength: 100,
};

export class AtlasProtocolValidator {
  private readonly ajv: Ajv2020;
  private readonly repoRoot: string;
  private readonly atlasProtocolRoot: string;
  private readonly validationErrorValidator: ValidateFunction;
  private readonly entityValidators: VariantValidators;
  private readonly objectValidators: VariantValidators;
  private readonly taskValidator: ValidateFunction;
  private readonly observationValidator: ValidateFunction;
  private readonly commandCatalogValidator: ValidateFunction;

  constructor(repoRoot: string, atlasProtocolRoot: string) {
    this.repoRoot = repoRoot;
    this.atlasProtocolRoot = atlasProtocolRoot;
    this.ajv = new Ajv2020({ allErrors: true, strict: false });
    addFormats(this.ajv);

    const entitySchema = this.readProtocolSchema("source/schemas/entity.schema.json");
    const taskSchema = this.readProtocolSchema("source/schemas/task.schema.json");
    const observationSchema = this.readProtocolSchema("source/schemas/observation.schema.json");
    const objectSchema = this.readProtocolSchema("source/schemas/object.schema.json");
    const commandCatalogSchema = this.readProtocolSchema(
      "source/schemas/command-catalog.schema.json",
    );
    const validationErrorSchema = this.readProtocolSchema(
      "source/schemas/validation-error.schema.json",
    );

    this.entityValidators = {
      asset: this.ajv.compile(this.schemaDef(entitySchema, "asset")),
      track: this.ajv.compile(this.schemaDef(entitySchema, "track")),
      geofeature: this.ajv.compile(this.schemaDef(entitySchema, "geofeature")),
    };
    this.objectValidators = {
      log: this.ajv.compile(this.schemaDef(objectSchema, "log")),
      photo: this.ajv.compile(this.schemaDef(objectSchema, "photo")),
      document: this.ajv.compile(this.schemaDef(objectSchema, "document")),
    };
    this.taskValidator = this.ajv.compile(taskSchema);
    this.observationValidator = this.ajv.compile(observationSchema);
    this.commandCatalogValidator = this.ajv.compile(commandCatalogSchema);
    this.validationErrorValidator = this.ajv.compile(validationErrorSchema);
  }

  readManifest<T>(relativePath: string): T {
    const absolutePath = path.join(this.atlasProtocolRoot, relativePath);
    return JSON.parse(fs.readFileSync(absolutePath, "utf8")) as T;
  }

  validateText(
    resource: Exclude<ResourceKind, "customSection">,
    text: string,
    variant?: string,
  ): ValidationIssue[] {
    let value: unknown;
    try {
      value = JSON.parse(text);
    } catch {
      return [
        {
          field: "json",
          code: "invalid_json",
          message: "json must be valid JSON",
        },
      ];
    }
    return this.validateValue(resource, value, {
      rawText: text,
      variant,
    });
  }

  validateValue(
    resource: ResourceKind,
    value: unknown,
    options: { rawText?: string; variant?: string } = {},
  ): ValidationIssue[] {
    const issues: ValidationIssue[] = [];

    if (!isPlainObject(value)) {
      return [
        {
          field: "json",
          code: "invalid_type",
          message: "json must be an object",
        },
      ];
    }

    const root = value as JsonObject;
    issues.push(
      ...this.collectLimitIssues("json", root, options.rawText, ROOT_LIMITS),
    );

    switch (resource) {
      case "entity":
        issues.push(...this.validateEntity(root, options.variant));
        break;
      case "task":
        issues.push(...this.validateTask(root));
        break;
      case "observation":
        issues.push(...this.validateObservation(root));
        break;
      case "object":
        issues.push(...this.validateObject(root, options.variant));
        break;
      case "commandCatalog":
        issues.push(...this.validateCommandCatalog(root));
        break;
      case "customSection":
        issues.push(...this.validateCustomSectionExample(root));
        break;
      default: {
        const unknown = resource as string;
        issues.push({
          field: "resource",
          code: "invalid_value",
          message: `unknown resource kind: ${unknown}`,
        });
        break;
      }
    }

    return dedupeIssues(issues);
  }

  assertValidationIssue(issue: ValidationIssue): void {
    if (!this.validationErrorValidator(issue)) {
      throw new Error(
        `validation issue failed schema validation: ${JSON.stringify(issue)}`,
      );
    }
  }

  resolveRepoPath(relativePath: string): string {
    return path.resolve(this.repoRoot, relativePath);
  }

  resolveProtocolPath(relativePath: string): string {
    return path.resolve(this.atlasProtocolRoot, relativePath);
  }

  /** Path under the Atlas Protocol root (schemas, manifests, goldens, examples). */
  resolveAtlasProtocolPath(relativePath: string): string {
    return path.join(this.atlasProtocolRoot, relativePath);
  }

  listExampleFiles(): string[] {
    const examplesDir = path.join(this.atlasProtocolRoot, "examples");
    if (!fs.existsSync(examplesDir)) {
      return [];
    }
    return fs
      .readdirSync(examplesDir)
      .filter((entry) => entry.endsWith(".json"))
      .sort()
      .map((entry) => path.join(examplesDir, entry));
  }

  private readProtocolSchema(relativePath: string): AnySchema {
    const absolutePath = path.join(this.atlasProtocolRoot, relativePath);
    return JSON.parse(fs.readFileSync(absolutePath, "utf8")) as AnySchema;
  }

  private schemaDef(schema: AnySchema, defName: string): AnySchema {
    const rootSchema = schema as {
      $defs: Record<string, unknown>;
      $schema?: string;
      title?: string;
    };
    return {
      $schema: rootSchema.$schema,
      title: rootSchema.title,
      $ref: `#/$defs/${defName}`,
      $defs: rootSchema.$defs,
    };
  }

  private validateEntity(root: JsonObject, variant?: string): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(
      ...this.collectTopLevelIssues(root, ["components", "extra"], "entity"),
    );
    issues.push(...this.collectCustomIssues(root, "json"));

    const components = root.components;
    if (components !== undefined && !isPlainObject(components)) {
      issues.push({
        field: "json.components",
        code: "invalid_type",
        message: "components must be an object",
      });
      return issues;
    }

    if (variant === "asset") {
      if (!isPlainObject(components) || components.supported_commands === undefined) {
        issues.push({
          field: "json.components.supported_commands",
          code: "required",
          message: "supported_commands is required for asset entity JSON",
        });
      }
    }

    if (isPlainObject(components)) {
      const telemetry = components.telemetry;
      if (isPlainObject(telemetry)) {
        const hasLatitude = telemetry.latitude !== undefined;
        const hasLongitude = telemetry.longitude !== undefined;
        if (hasLatitude !== hasLongitude) {
          issues.push({
            field: `json.components.telemetry.${hasLatitude ? "longitude" : "latitude"}`,
            code: "required_pair",
            message: "latitude and longitude must be provided together",
          });
        }
      }
    }

    const validator = variant ? this.entityValidators[variant] : undefined;
    if (!validator) {
      issues.push({
        field: "json",
        code: "invalid_value",
        message: "entity variant is required",
      });
      return issues;
    }
    const promotedPaths = new Set(
      issues.filter((i) => i.code === "promoted_field").map((i) => i.field),
    );
    const customRequiredFields = new Set(
      issues.filter((i) => i.code === "required").map((i) => i.field),
    );
    const requiredPairFields = new Set(
      issues.filter((i) => i.code === "required_pair").map((i) => i.field),
    );
    issues.push(
      ...this.runSchema(validator, root).filter((s) => {
        if (promotedPaths.has(s.field) && s.code === "unknown_field") {
          return false;
        }
        if (s.code === "required" && customRequiredFields.has(s.field)) {
          return false;
        }
        if (s.code === "required" && requiredPairFields.has(s.field)) {
          return false;
        }
        return true;
      }),
    );
    return issues;
  }

  private validateTask(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(
      ...this.collectTopLevelIssues(root, ["description", "created_by", "components", "extra"], "task"),
    );
    issues.push(...this.collectCustomIssues(root, "json"));

    const components = root.components;
    if (components !== undefined && !isPlainObject(components)) {
      issues.push({
        field: "json.components",
        code: "invalid_type",
        message: "components must be an object",
      });
      return issues;
    }

    if (!isPlainObject(components) || !isPlainObject(components.command)) {
      issues.push({
        field: "json.components.command.type",
        code: "required",
        message: "command.type is required",
      });
    } else if (
      typeof components.command.type !== "string" ||
      components.command.type.length === 0
    ) {
      issues.push({
        field: "json.components.command.type",
        code: "required",
        message: "command.type is required",
      });
    }

    const promotedPathsTask = new Set(
      issues.filter((i) => i.code === "promoted_field").map((i) => i.field),
    );
    const customRequiredTask = new Set(
      issues.filter((i) => i.code === "required").map((i) => i.field),
    );
    issues.push(
      ...this.runSchema(this.taskValidator, root).filter((s) => {
        if (promotedPathsTask.has(s.field) && s.code === "unknown_field") {
          return false;
        }
        if (s.code === "required" && customRequiredTask.has(s.field)) {
          return false;
        }
        return true;
      }),
    );
    return issues;
  }

  private validateObservation(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(
      ...this.collectTopLevelIssues(
        root,
        ["state", "latest_sighting", "sightings_object_id", "extra"],
        "observation",
      ),
    );
    issues.push(...this.collectCustomIssues(root, "json"));

    if (root.state === undefined) {
      issues.push({
        field: "json.state",
        code: "required",
        message: "state is required",
      });
    } else if (
      root.state !== "active" &&
      root.state !== "inactive" &&
      root.state !== "ended"
    ) {
      issues.push({
        field: "json.state",
        code: "invalid_value",
        message: "state must be one of active, inactive, ended",
      });
    }

    const promotedPathsObs = new Set(
      issues.filter((i) => i.code === "promoted_field").map((i) => i.field),
    );
    const customRequiredObs = new Set(
      issues.filter((i) => i.code === "required").map((i) => i.field),
    );
    issues.push(
      ...this.runSchema(this.observationValidator, root).filter((s) => {
        if (promotedPathsObs.has(s.field) && s.code === "unknown_field") {
          return false;
        }
        if (s.code === "required" && customRequiredObs.has(s.field)) {
          return false;
        }
        return true;
      }),
    );
    return issues;
  }

  private validateObject(root: JsonObject, variant?: string): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    const allowedByVariant: Record<string, string[]> = {
      log: ["log_type", "started_at", "ended_at", "extra"],
      photo: ["content_type", "captured_at", "width_px", "height_px", "extra"],
      document: ["content_type", "extra"],
    };
    issues.push(
      ...this.collectTopLevelIssues(root, allowedByVariant[variant ?? ""] ?? [], "object"),
    );
    issues.push(...this.collectCustomIssues(root, "json"));

    for (const key of Object.keys(root)) {
      if (OBJECT_RESERVED_FIELDS.has(key)) {
        issues.push({
          field: `json.${key}`,
          code: "reserved_field",
          message: `${key} is reserved for internal manifest cache writes`,
        });
      }
    }

    const validator = variant ? this.objectValidators[variant] : undefined;
    if (!validator) {
      issues.push({
        field: "json",
        code: "invalid_value",
        message: "object variant is required",
      });
      return issues;
    }
    const promotedPathsObj = new Set(
      issues.filter((i) => i.code === "promoted_field").map((i) => i.field),
    );
    const reservedPathsObj = new Set(
      issues.filter((i) => i.code === "reserved_field").map((i) => i.field),
    );
    const customRequiredObj = new Set(
      issues.filter((i) => i.code === "required").map((i) => i.field),
    );
    issues.push(
      ...this.runSchema(validator, root).filter((s) => {
        if (promotedPathsObj.has(s.field) && s.code === "unknown_field") {
          return false;
        }
        if (reservedPathsObj.has(s.field) && s.code === "unknown_field") {
          return false;
        }
        if (s.code === "required" && customRequiredObj.has(s.field)) {
          return false;
        }
        return true;
      }),
    );
    return issues;
  }

  private validateCommandCatalog(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(
        ...this.collectTopLevelIssues(
          root,
          ["type", "name", "description", "commands"],
          "commandCatalog",
        ),
    );
    const commands = root.commands;
    const parameterSchemaTypoPaths = new Set<string>();
    if (Array.isArray(commands)) {
      const seen = new Map<string, number>();
      commands.forEach((command, index) => {
        if (!isPlainObject(command)) {
          return;
        }
        if (command.parameter_schema !== undefined) {
          const field = `json.commands[${index}].parameter_schema`;
          parameterSchemaTypoPaths.add(field);
          issues.push({
            field,
            code: "unknown_field",
            message: "\"parameter_schema\" is not allowed; use \"parameters_schema\"",
          });
        }
        if (typeof command.id === "string") {
          const prior = seen.get(command.id);
          if (prior !== undefined) {
            issues.push({
              field: `json.commands[${index}].id`,
              code: "duplicate_command_id",
              message: `command id "${command.id}" must be unique`,
            });
          } else {
            seen.set(command.id, index);
          }
        }
      });
    }

    issues.push(
      ...this.runSchema(this.commandCatalogValidator, root).filter((schemaIssue) => {
        if (
          schemaIssue.code === "unknown_field" &&
          parameterSchemaTypoPaths.has(schemaIssue.field)
        ) {
          return false;
        }
        return true;
      }),
    );
    return issues;
  }

  private validateCustomSectionExample(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    for (const [key, value] of Object.entries(root)) {
      if (!key.startsWith("custom_")) {
        issues.push({
          field: `json.${key}`,
          code: "unknown_field",
          message: `${key} is not allowed at the top level`,
        });
        continue;
      }
      if (!isPlainObject(value)) {
        issues.push({
          field: `json.${key}`,
          code: "invalid_type",
          message: `${key} must be an object`,
        });
        continue;
      }
      issues.push(...this.collectLimitIssues(`json.${key}`, value, undefined, CUSTOM_LIMITS));
    }
    return issues;
  }

  private collectTopLevelIssues(
    root: JsonObject,
    allowedFields: string[],
    resourceLabel: string,
  ): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    const allowed = new Set(allowedFields);
    for (const key of Object.keys(root)) {
      if (resourceLabel !== "commandCatalog" && resourceLabel !== "customSection" && PROMOTED_FIELDS.has(key)) {
        issues.push({
          field: `json.${key}`,
          code: "promoted_field",
          message: `${key} is a promoted top-level field`,
        });
        continue;
      }
      if (!allowed.has(key) && !key.startsWith("custom_")) {
        if (resourceLabel === "object" && OBJECT_RESERVED_FIELDS.has(key)) {
          continue;
        }
        issues.push({
          field: `json.${key}`,
          code: "unknown_field",
          message: `${key} is not allowed`,
        });
      }
    }
    return issues;
  }

  private collectCustomIssues(root: JsonObject, basePath: string): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    for (const [key, value] of Object.entries(root)) {
      if (!key.startsWith("custom_")) {
        continue;
      }
      if (!isPlainObject(value)) {
        issues.push({
          field: `${basePath}.${key}`,
          code: "invalid_type",
          message: `${key} must be an object`,
        });
        continue;
      }
      issues.push(...this.collectLimitIssues(`${basePath}.${key}`, value, undefined, CUSTOM_LIMITS));
    }
    const components = root.components;
    if (isPlainObject(components)) {
      for (const [key, value] of Object.entries(components)) {
        if (!key.startsWith("custom_")) {
          continue;
        }
        if (!isPlainObject(value)) {
          issues.push({
            field: `${basePath}.components.${key}`,
            code: "invalid_type",
            message: `${key} must be an object`,
          });
          continue;
        }
        issues.push(
          ...this.collectLimitIssues(
            `${basePath}.components.${key}`,
            value,
            undefined,
            CUSTOM_LIMITS,
          ),
        );
      }
    }
    return issues;
  }

  private collectLimitIssues(
    basePath: string,
    value: JsonObject,
    rawText: string | undefined,
    limits: {
      maxBytes: number;
      maxDepth: number;
      maxFields: number;
      maxKeyLength: number;
    },
  ): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    const bytes = Buffer.byteLength(rawText ?? JSON.stringify(value), "utf8");
    if (bytes > limits.maxBytes) {
      issues.push({
        field: basePath,
        code: "limit_exceeded",
        message: `${basePath} exceeds the size limit`,
      });
    }

    let fieldCount = 0;
    const walk = (node: unknown, currentPath: string, depth: number): void => {
      if (depth > limits.maxDepth) {
        issues.push({
          field: currentPath,
          code: "limit_exceeded",
          message: `${currentPath} exceeds the nesting limit`,
        });
        return;
      }
      if (Array.isArray(node)) {
        node.forEach((item, index) => walk(item, `${currentPath}[${index}]`, depth + 1));
        return;
      }
      if (!isPlainObject(node)) {
        return;
      }
      for (const [key, child] of Object.entries(node)) {
        fieldCount += 1;
        if (fieldCount > limits.maxFields) {
          issues.push({
            field: currentPath,
            code: "limit_exceeded",
            message: `${currentPath} exceeds the field-count limit`,
          });
          return;
        }
        if (key.length > limits.maxKeyLength) {
          issues.push({
            field: `${currentPath}.${key}`,
            code: "limit_exceeded",
            message: `${currentPath}.${key} exceeds the key-length limit`,
          });
        }
        walk(child, `${currentPath}.${key}`, depth + 1);
      }
    };
    walk(value, basePath, 1);
    return issues;
  }

  private runSchema(validator: ValidateFunction, value: JsonObject): ValidationIssue[] {
    if (validator(value)) {
      return [];
    }
    return (validator.errors ?? [])
      .map((error) => this.mapAjvError(error))
      .filter((issue): issue is ValidationIssue => issue !== null);
  }

  private mapAjvError(error: ErrorObject): ValidationIssue | null {
    if (error.keyword === "required") {
      const missingProperty = String((error.params as { missingProperty: string }).missingProperty);
      return {
        field: instancePathToField(error.instancePath, missingProperty),
        code: "required",
        message: `${missingProperty} is required`,
      };
    }
    if (error.keyword === "additionalProperties") {
      const additionalProperty = String(
        (error.params as { additionalProperty: string }).additionalProperty,
      );
      return {
        field: instancePathToField(error.instancePath, additionalProperty),
        code: "unknown_field",
        message: `${additionalProperty} is not allowed`,
      };
    }
    if (error.keyword === "enum") {
      const allowedValues = (error.params as { allowedValues: unknown[] }).allowedValues;
      return {
        field: instancePathToField(error.instancePath),
        code: "invalid_value",
        message: `${lastFieldSegment(instancePathToField(error.instancePath))} must be one of ${allowedValues.join(", ")}`,
      };
    }
    if (error.keyword === "type") {
      const field = instancePathToField(error.instancePath);
      const typeName = String((error.params as { type: string }).type);
      return {
        field,
        code: "invalid_type",
        message: `${lastFieldSegment(field)} must be ${typeName === "object" ? "an object" : `a ${typeName}`}`,
      };
    }
    if (error.keyword === "minLength") {
      const field = instancePathToField(error.instancePath);
      return {
        field,
        code: "invalid_value",
        message: `${lastFieldSegment(field)} must not be empty`,
      };
    }
    if (error.keyword === "format") {
      const field = instancePathToField(error.instancePath);
      const fmt = String((error.params as { format?: string }).format ?? "format");
      return {
        field,
        code: "invalid_value",
        message: `${lastFieldSegment(field)} must match ${fmt} format`,
      };
    }
    if (
      error.keyword === "minimum" ||
      error.keyword === "maximum" ||
      error.keyword === "exclusiveMaximum"
    ) {
      const field = instancePathToField(error.instancePath);
      return {
        field,
        code: "invalid_value",
        message: `${lastFieldSegment(field)} is out of range`,
      };
    }
    return {
      field: instancePathToField(error.instancePath),
      code: "invalid_value",
      message: error.message ?? "invalid value",
    };
  }
}

function isPlainObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function instancePathToField(instancePath: string, child?: string): string {
  const segments = instancePath
    .split("/")
    .filter(Boolean)
    .map((segment) => segment.replace(/~1/g, "/").replace(/~0/g, "~"));

  const parts = ["json"];
  for (const segment of segments) {
    if (/^\d+$/.test(segment)) {
      const last = parts.pop() ?? "json";
      parts.push(`${last}[${segment}]`);
    } else {
      parts.push(segment);
    }
  }
  if (child) {
    parts.push(child);
  }
  return parts.join(".");
}

function lastFieldSegment(field: string): string {
  const parts = field.split(".");
  return parts[parts.length - 1] ?? field;
}

function dedupeIssues(issues: ValidationIssue[]): ValidationIssue[] {
  const seen = new Set<string>();
  const result: ValidationIssue[] = [];
  for (const issue of issues) {
    const key = `${issue.field}|${issue.code}|${issue.message}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push(issue);
  }
  return result;
}

/** Deterministic sort for conformance comparisons (field, code, message). */
export function normalizeValidationIssues(issues: ValidationIssue[]): ValidationIssue[] {
  return [...issues].sort((a, b) => {
    const c0 = a.field.localeCompare(b.field);
    if (c0 !== 0) {
      return c0;
    }
    const c1 = a.code.localeCompare(b.code);
    if (c1 !== 0) {
      return c1;
    }
    return a.message.localeCompare(b.message);
  });
}
