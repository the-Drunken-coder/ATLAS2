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
  | "changeEvent"
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
  "target_entity_id",
  "created_at",
  "updated_at",
  "version",
]);

const OBJECT_RESERVED_FIELDS = new Set(["manifest", "manifest_version"]);

const STANDARD_GEOJSON_TYPES = new Set([
  "Point",
  "MultiPoint",
  "LineString",
  "MultiLineString",
  "Polygon",
  "MultiPolygon",
  "GeometryCollection",
]);

const SIGHTING_KINDS = new Set(["line_of_bearing", "point", "area"]);

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
  private readonly changeEventValidator: ValidateFunction;

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
    const changeEventSchema = this.readProtocolSchema(
      "source/schemas/change-event.schema.json",
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
      observation_history: this.ajv.compile(this.schemaDef(objectSchema, "observation_history")),
      track_provenance: this.ajv.compile(this.schemaDef(objectSchema, "track_provenance")),
      document: this.ajv.compile(this.schemaDef(objectSchema, "document")),
    };
    this.taskValidator = this.ajv.compile(taskSchema);
    this.observationValidator = this.ajv.compile(observationSchema);
    this.commandCatalogValidator = this.ajv.compile(commandCatalogSchema);
    this.changeEventValidator = this.ajv.compile(changeEventSchema);
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
      case "changeEvent":
        issues.push(...this.validateChangeEvent(root));
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
      if (variant === "geofeature" && isPlainObject(components.geometry)) {
        issues.push(...validateGeoJSONGeometry(components.geometry, "json.components.geometry"));
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
        ["identity", "latest_telemetry", "history_object_id", "extra"],
        "observation",
      ),
    );
    issues.push(...this.collectCustomIssues(root, "json"));

    for (const rejected of ["state", "latest_sighting", "sightings_object_id"] as const) {
      if (root[rejected] !== undefined) {
        issues.push({
          field: `json.${rejected}`,
          code: "unknown_field",
          message: `${rejected} is not allowed`,
        });
      }
    }
    if (isPlainObject(root.latest_telemetry)) {
      issues.push(...validateLatestSighting(root.latest_telemetry, "json.latest_telemetry"));
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
      command_catalog: ["type", "name", "description", "commands", "extra"],
      observation_history: ["format_version", "extra"],
      track_provenance: ["format_version", "extra"],
    };
    const topLevelLabel = variant === "command_catalog" ? "commandCatalog" : "object";
    issues.push(
      ...this.collectTopLevelIssues(root, allowedByVariant[variant ?? ""] ?? [], topLevelLabel),
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

    if (variant === "command_catalog") {
      issues.push(...this.validateCommandCatalogCommandRules(root));
      const parameterSchemaTypoPaths = this.commandCatalogParameterSchemaTypoPaths(root);
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

  private commandCatalogParameterSchemaTypoPaths(root: JsonObject): Set<string> {
    const paths = new Set<string>();
    const commands = root.commands;
    if (!Array.isArray(commands)) {
      return paths;
    }
    commands.forEach((command, index) => {
      if (!isPlainObject(command)) {
        return;
      }
      if (command.parameter_schema !== undefined) {
        paths.add(`json.commands[${index}].parameter_schema`);
      }
    });
    return paths;
  }

  private validateCommandCatalogCommandRules(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    const parameterSchemaTypoPaths = this.commandCatalogParameterSchemaTypoPaths(root);
    for (const field of parameterSchemaTypoPaths) {
      issues.push({
        field,
        code: "unknown_field",
        message: "\"parameter_schema\" is not allowed; use \"parameters_schema\"",
      });
    }
    const commands = root.commands;
    if (Array.isArray(commands)) {
      const seen = new Map<string, number>();
      commands.forEach((command, index) => {
        if (!isPlainObject(command)) {
          return;
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
    return issues;
  }

  private validateCommandCatalog(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(
      ...this.collectTopLevelIssues(
        root,
        ["type", "name", "description", "commands", "extra"],
        "commandCatalog",
      ),
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
    issues.push(...this.validateCommandCatalogCommandRules(root));
    const parameterSchemaTypoPaths = this.commandCatalogParameterSchemaTypoPaths(root);
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

  private validateChangeEvent(root: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    issues.push(...this.runSchema(this.changeEventValidator, root));

    const operation = typeof root.operation === "string" ? root.operation : "";
    const resource = typeof root.resource === "string" ? root.resource : "";
    const snapshot = root.snapshot;

    if (operation === "deleted") {
      if (snapshot !== null) {
        issues.push({
          field: "json.snapshot",
          code: "invalid_value",
          message: "snapshot must be null for deleted events",
        });
      }
      return dedupeIssues(issues);
    }

    if (operation === "created" || operation === "updated") {
      if (!isPlainObject(snapshot)) {
        issues.push({
          field: "json.snapshot",
          code: "invalid_type",
          message: "snapshot must be an object",
        });
        return dedupeIssues(issues);
      }
      const inferredResource = this.inferSnapshotResource(snapshot);
      if (inferredResource !== null && inferredResource !== resource) {
        issues.push({
          field: "json.snapshot",
          code: "invalid_value",
          message: `snapshot resource type ${inferredResource} does not match declared resource ${resource}`,
        });
        return dedupeIssues(issues);
      }
      const snapshotResource = inferredResource || resource;
      issues.push(...this.validateSnapshot(snapshotResource, snapshot));
      const snapshotIdentity = snapshotResourceIdentity(snapshotResource, snapshot);
      if (
        snapshotIdentity !== null &&
        typeof root.resource_id === "string" &&
        root.resource_id !== snapshotIdentity.value
      ) {
        issues.push({
          field: "json.resource_id",
          code: "invalid_value",
          message: `resource_id must match snapshot ${snapshotIdentity.field}`,
        });
      }
      if (
        Number.isInteger(root.resource_version) &&
        Number.isInteger(snapshot.version) &&
        root.resource_version !== snapshot.version
      ) {
        issues.push({
          field: "json.resource_version",
          code: "invalid_value",
          message: "resource_version must match snapshot version",
        });
      }
    }

    return dedupeIssues(issues);
  }

  private inferSnapshotResource(snapshot: JsonObject): string | null {
    if (snapshot.entity_id !== undefined || snapshot.entity_type !== undefined) {
      return "entity";
    }
    if (snapshot.object_id !== undefined || snapshot.object_type !== undefined) {
      return "object";
    }
    if (snapshot.task_id !== undefined) {
      return "task";
    }
    if (snapshot.observation_id !== undefined) {
      return "observation";
    }
    return null;
  }

  private validateSnapshot(resource: string, snapshot: JsonObject): ValidationIssue[] {
    const issues: ValidationIssue[] = [];
    const commonFields = ["version", "created_at", "updated_at", "json"];
    const commonRequired = ["version", "created_at", "updated_at", "json"];
    const validateCommon = (allowed: string[], required: string[]): void => {
      for (const key of Object.keys(snapshot)) {
        if (!allowed.includes(key)) {
          issues.push({
            field: `json.snapshot.${key}`,
            code: "unknown_field",
            message: `${key} is not allowed`,
          });
        }
      }
      for (const key of required) {
        if (snapshot[key] === undefined) {
          issues.push({
            field: `json.snapshot.${key}`,
            code: "required",
            message: `${key} is required`,
          });
        }
      }
      const version = snapshot.version;
      if (version !== undefined && (typeof version !== "number" || !Number.isInteger(version) || version < 1)) {
        issues.push({
          field: "json.snapshot.version",
          code: "invalid_value",
          message: "version is out of range",
        });
      }
      for (const key of ["created_at", "updated_at"]) {
        if (snapshot[key] !== undefined && typeof snapshot[key] !== "string") {
          issues.push({
            field: `json.snapshot.${key}`,
            code: "invalid_type",
            message: `${key} must be a string`,
          });
        }
      }
    };

    if (resource === "entity") {
      validateCommon(
        ["entity_id", "entity_type", "subtype", "alias", ...commonFields],
        ["entity_id", "entity_type", ...commonRequired],
      );
      if (typeof snapshot.entity_id !== "string" || snapshot.entity_id.length === 0) {
        issues.push({ field: "json.snapshot.entity_id", code: "invalid_value", message: "entity_id must not be empty" });
      }
      const variant = typeof snapshot.entity_type === "string" ? snapshot.entity_type : undefined;
      if (!["asset", "track", "geofeature"].includes(variant ?? "")) {
        issues.push({ field: "json.snapshot.entity_type", code: "invalid_value", message: "entity_type must be one of asset, track, geofeature" });
      }
      if (isPlainObject(snapshot.json)) {
        issues.push(...prefixIssues(this.validateEntity(snapshot.json, variant), "json.snapshot.json"));
      } else if (snapshot.json !== undefined) {
        issues.push({ field: "json.snapshot.json", code: "invalid_type", message: "json must be an object" });
      }
      return issues;
    }

    if (resource === "object") {
      validateCommon(
        ["object_id", "object_type", "owner_type", "owner_id", ...commonFields],
        ["object_id", "object_type", "owner_type", "owner_id", ...commonRequired],
      );
      checkNonEmptyString(issues, snapshot, "object_id", "json.snapshot.object_id");
      const variant = typeof snapshot.object_type === "string" ? snapshot.object_type : undefined;
      if (!["log", "photo", "document", "command_catalog", "observation_history", "track_provenance"].includes(variant ?? "")) {
        issues.push({ field: "json.snapshot.object_type", code: "invalid_value", message: "object_type must be one of log, photo, document, command_catalog, observation_history, track_provenance" });
      }
      if (!["entity", "observation", "task", "system"].includes(String(snapshot.owner_type ?? ""))) {
        issues.push({ field: "json.snapshot.owner_type", code: "invalid_value", message: "owner_type must be one of entity, observation, task, system" });
      }
      if (isPlainObject(snapshot.json)) {
        issues.push(...prefixIssues(this.validateObject(snapshot.json, variant), "json.snapshot.json"));
      } else if (snapshot.json !== undefined) {
        issues.push({ field: "json.snapshot.json", code: "invalid_type", message: "json must be an object" });
      }
      return issues;
    }

    if (resource === "task") {
      validateCommon(
        ["task_id", "status", "asset_id", "command_catalog_object_id", ...commonFields],
        ["task_id", "status", "asset_id", "command_catalog_object_id", ...commonRequired],
      );
      checkNonEmptyString(issues, snapshot, "task_id", "json.snapshot.task_id");
      if (!["pending", "acknowledged", "completed", "failed"].includes(String(snapshot.status ?? ""))) {
        issues.push({ field: "json.snapshot.status", code: "invalid_value", message: "status must be one of pending, acknowledged, completed, failed" });
      }
      if (isPlainObject(snapshot.json)) {
        issues.push(...prefixIssues(this.validateTask(snapshot.json), "json.snapshot.json"));
      } else if (snapshot.json !== undefined) {
        issues.push({ field: "json.snapshot.json", code: "invalid_type", message: "json must be an object" });
      }
      return issues;
    }

    if (resource === "observation") {
      validateCommon(
        ["observation_id", "source_asset_id", ...commonFields],
        ["observation_id", "source_asset_id", ...commonRequired],
      );
      checkNonEmptyString(issues, snapshot, "observation_id", "json.snapshot.observation_id");
      if (isPlainObject(snapshot.json)) {
        issues.push(...prefixIssues(this.validateObservation(snapshot.json), "json.snapshot.json"));
      } else if (snapshot.json !== undefined) {
        issues.push({ field: "json.snapshot.json", code: "invalid_type", message: "json must be an object" });
      }
      return issues;
    }

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

    const countFields = (node: unknown): number => {
      if (Array.isArray(node)) {
        return node.reduce((total, child) => total + countFields(child), 0);
      }
      if (!isPlainObject(node)) {
        return 0;
      }
      return Object.values(node).reduce<number>(
        (total, child) => total + 1 + countFields(child),
        0,
      );
    };
    if (countFields(value) > limits.maxFields) {
      issues.push({
        field: basePath,
        code: "limit_exceeded",
        message: `${basePath} exceeds the field-count limit`,
      });
    }
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

function prefixIssues(issues: ValidationIssue[], basePath: string): ValidationIssue[] {
  return issues.map((issue) => ({
    ...issue,
    field: issue.field === "json" ? basePath : issue.field.replace(/^json(?=\.|\[|$)/, basePath),
  }));
}

function snapshotResourceIdentity(
  resource: string,
  snapshot: JsonObject,
): { field: string; value: string } | null {
  const fieldByResource: Record<string, string> = {
    entity: "entity_id",
    object: "object_id",
    task: "task_id",
    observation: "observation_id",
  };
  const field = fieldByResource[resource];
  if (field === undefined) {
    return null;
  }
  const value = snapshot[field];
  if (typeof value !== "string" || value.length === 0) {
    return null;
  }
  return { field, value };
}

function checkNonEmptyString(
  issues: ValidationIssue[],
  root: JsonObject,
  key: string,
  field: string,
): void {
  const value = root[key];
  if (value === undefined) {
    return;
  }
  if (typeof value !== "string") {
    issues.push({ field, code: "invalid_type", message: `${key} must be a string` });
    return;
  }
  if (value.length === 0) {
    issues.push({ field, code: "invalid_value", message: `${key} must not be empty` });
  }
}

function validateLatestSighting(sighting: JsonObject, basePath: string): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const kind = typeof sighting.kind === "string" ? sighting.kind : "";
  const data = sighting.data;
  if (!kind || !SIGHTING_KINDS.has(kind)) {
    issues.push({
      field: `${basePath}.kind`,
      code: "invalid_value",
      message: "kind must be one of line_of_bearing, point, area",
    });
    return issues;
  }
  if (!isPlainObject(data)) {
    return issues;
  }
  if (kind === "line_of_bearing") {
    issues.push(...validateAllowedFields(data, `${basePath}.data`, [
      "observer_latitude",
      "observer_longitude",
      "observer_altitude_m",
      "azimuth_deg",
      "elevation_deg",
      "range_m",
      "uncertainty_deg",
    ]));
    for (const required of ["observer_latitude", "observer_longitude", "azimuth_deg"]) {
      if (data[required] === undefined) {
        issues.push({ field: `${basePath}.data.${required}`, code: "required", message: `${required} is required` });
      }
    }
    checkNumberRange(issues, data, "observer_latitude", `${basePath}.data.observer_latitude`, -90, 90);
    checkNumberRange(issues, data, "observer_longitude", `${basePath}.data.observer_longitude`, -180, 180);
    checkNumberRange(issues, data, "azimuth_deg", `${basePath}.data.azimuth_deg`, 0, 360, true);
    checkNumberRange(issues, data, "elevation_deg", `${basePath}.data.elevation_deg`, -90, 90);
    checkNumberRange(issues, data, "range_m", `${basePath}.data.range_m`, 0);
    checkNumberRange(issues, data, "uncertainty_deg", `${basePath}.data.uncertainty_deg`, 0);
    return issues;
  }
  if (kind === "point") {
    issues.push(...validateAllowedFields(data, `${basePath}.data`, [
      "latitude",
      "longitude",
      "altitude_m",
      "uncertainty_radius_m",
    ]));
    for (const required of ["latitude", "longitude"]) {
      if (data[required] === undefined) {
        issues.push({ field: `${basePath}.data.${required}`, code: "required", message: `${required} is required` });
      }
    }
    checkNumberRange(issues, data, "latitude", `${basePath}.data.latitude`, -90, 90);
    checkNumberRange(issues, data, "longitude", `${basePath}.data.longitude`, -180, 180);
    checkNumberRange(issues, data, "uncertainty_radius_m", `${basePath}.data.uncertainty_radius_m`, 0);
    return issues;
  }
  if (kind === "area") {
    issues.push(...validateAllowedFields(data, `${basePath}.data`, ["geometry", "confidence"]));
    if (data.geometry === undefined) {
      issues.push({ field: `${basePath}.data.geometry`, code: "required", message: "geometry is required" });
    } else if (isPlainObject(data.geometry)) {
      issues.push(...validateGeoJSONGeometry(data.geometry, `${basePath}.data.geometry`, new Set(["Polygon", "MultiPolygon"])));
    } else {
      issues.push({ field: `${basePath}.data.geometry`, code: "invalid_type", message: "geometry must be an object" });
    }
    checkNumberRange(issues, data, "confidence", `${basePath}.data.confidence`, 0, 1);
  }
  return issues;
}

function validateAllowedFields(root: JsonObject, basePath: string, allowedFields: string[]): ValidationIssue[] {
  const allowed = new Set(allowedFields);
  const issues: ValidationIssue[] = [];
  for (const key of Object.keys(root)) {
    if (!allowed.has(key)) {
      issues.push({ field: `${basePath}.${key}`, code: "unknown_field", message: `${key} is not allowed` });
    }
  }
  return issues;
}

function checkNumberRange(
  issues: ValidationIssue[],
  root: JsonObject,
  key: string,
  field: string,
  min?: number,
  max?: number,
  exclusiveMax = false,
): void {
  if (root[key] === undefined) {
    return;
  }
  if (typeof root[key] !== "number") {
    issues.push({ field, code: "invalid_type", message: `${lastFieldSegment(field)} must be a number` });
    return;
  }
  const value = root[key] as number;
  if ((min !== undefined && value < min) || (max !== undefined && (exclusiveMax ? value >= max : value > max))) {
    issues.push({ field, code: "invalid_value", message: `${lastFieldSegment(field)} is out of range` });
  }
}

function validateGeoJSONGeometry(
  geometry: JsonObject,
  basePath: string,
  allowedTypes = STANDARD_GEOJSON_TYPES,
): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const type = typeof geometry.type === "string" ? geometry.type : "";
  if (!type || !allowedTypes.has(type)) {
    issues.push({
      field: `${basePath}.type`,
      code: "invalid_value",
      message: `type must be one of ${[...allowedTypes].join(", ")}`,
    });
    return issues;
  }
  if (type === "GeometryCollection") {
    if (!Array.isArray(geometry.geometries)) {
      issues.push({ field: `${basePath}.geometries`, code: "required", message: "geometries is required" });
      return issues;
    }
    geometry.geometries.forEach((child, index) => {
      if (isPlainObject(child)) {
        issues.push(...validateGeoJSONGeometry(child, `${basePath}.geometries[${index}]`));
      } else {
        issues.push({ field: `${basePath}.geometries[${index}]`, code: "invalid_type", message: `${lastFieldSegment(`${basePath}.geometries[${index}]`)} must be an object` });
      }
    });
    return issues;
  }
  if (!Array.isArray(geometry.coordinates)) {
    issues.push({ field: `${basePath}.coordinates`, code: "required", message: "coordinates is required" });
    return issues;
  }
  switch (type) {
    case "Point":
      validatePosition(issues, geometry.coordinates, `${basePath}.coordinates`);
      break;
    case "MultiPoint":
      validatePositions(issues, geometry.coordinates, `${basePath}.coordinates`);
      break;
    case "LineString":
      validateLineString(issues, geometry.coordinates, `${basePath}.coordinates`);
      break;
    case "MultiLineString":
      validateNested(issues, geometry.coordinates, `${basePath}.coordinates`, validateLineString);
      break;
    case "Polygon":
      validatePolygon(issues, geometry.coordinates, `${basePath}.coordinates`);
      break;
    case "MultiPolygon":
      validateNested(issues, geometry.coordinates, `${basePath}.coordinates`, validatePolygon);
      break;
  }
  return issues;
}

type Coordinate = [number, number];

function validatePosition(issues: ValidationIssue[], value: unknown, field: string): Coordinate | undefined {
  if (!Array.isArray(value) || value.length < 2 || value.length > 3) {
    issues.push({ field, code: "invalid_value", message: "position must be [longitude, latitude] or [longitude, latitude, altitude_m]" });
    return undefined;
  }
  const [longitude, latitude] = value;
  if (typeof longitude !== "number" || longitude < -180 || longitude > 180) {
    issues.push({ field: `${field}[0]`, code: "invalid_value", message: "longitude is out of range" });
    return undefined;
  }
  if (typeof latitude !== "number" || latitude < -90 || latitude > 90) {
    issues.push({ field: `${field}[1]`, code: "invalid_value", message: "latitude is out of range" });
    return undefined;
  }
  if (value.length === 3 && typeof value[2] !== "number") {
    issues.push({ field: `${field}[2]`, code: "invalid_type", message: "altitude_m must be a number" });
  }
  return [longitude, latitude];
}

function validatePositions(issues: ValidationIssue[], value: unknown, field: string): Coordinate[] {
  if (!Array.isArray(value)) {
    issues.push({ field, code: "invalid_type", message: `${lastFieldSegment(field)} must be an array` });
    return [];
  }
  return value
    .map((item, index) => validatePosition(issues, item, `${field}[${index}]`))
    .filter((item): item is Coordinate => item !== undefined);
}

function validateLineString(issues: ValidationIssue[], value: unknown, field: string): Coordinate[] {
  const positions = validatePositions(issues, value, field);
  if (positions.length > 0 && positions.length < 2) {
    issues.push({ field, code: "invalid_value", message: "LineString must contain at least 2 positions" });
  }
  for (let i = 1; i < positions.length; i += 1) {
    if (positions[i][0] === positions[i - 1][0] && positions[i][1] === positions[i - 1][1]) {
      issues.push({ field: `${field}[${i}]`, code: "invalid_value", message: "LineString must not contain zero-length segments" });
      break;
    }
  }
  return positions;
}

function validatePolygon(issues: ValidationIssue[], value: unknown, field: string): void {
  if (!Array.isArray(value)) {
    issues.push({ field, code: "invalid_type", message: `${lastFieldSegment(field)} must be an array` });
    return;
  }
  value.forEach((ring, index) => {
    const ringPath = `${field}[${index}]`;
    const positions = validatePositions(issues, ring, ringPath);
    if (positions.length > 0 && positions.length < 4) {
      issues.push({ field: ringPath, code: "invalid_value", message: "Polygon ring must contain at least 4 positions" });
    }
    if (positions.length >= 2) {
      const first = positions[0];
      const last = positions[positions.length - 1];
      if (first[0] !== last[0] || first[1] !== last[1]) {
        issues.push({ field: ringPath, code: "invalid_value", message: "Polygon ring must be closed" });
      }
    }
    if (ringSelfIntersects(positions)) {
      issues.push({ field: ringPath, code: "invalid_value", message: "Polygon ring must not self-intersect" });
    }
  });
}

function validateNested(
  issues: ValidationIssue[],
  value: unknown,
  field: string,
  validator: (issues: ValidationIssue[], value: unknown, field: string) => unknown,
): void {
  if (!Array.isArray(value)) {
    issues.push({ field, code: "invalid_type", message: `${lastFieldSegment(field)} must be an array` });
    return;
  }
  value.forEach((child, index) => validator(issues, child, `${field}[${index}]`));
}

function ringSelfIntersects(ring: Coordinate[]): boolean {
  if (ring.length < 4) {
    return false;
  }
  for (let i = 0; i < ring.length - 1; i += 1) {
    for (let j = i + 1; j < ring.length - 1; j += 1) {
      if (Math.abs(i - j) <= 1) {
        continue;
      }
      if (i === 0 && j === ring.length - 2) {
        continue;
      }
      if (segmentsIntersect(ring[i], ring[i + 1], ring[j], ring[j + 1])) {
        return true;
      }
    }
  }
  return false;
}

function segmentsIntersect(a: Coordinate, b: Coordinate, c: Coordinate, d: Coordinate): boolean {
  const orient = (p: Coordinate, q: Coordinate, r: Coordinate): number =>
    Math.sign((q[0] - p[0]) * (r[1] - p[1]) - (q[1] - p[1]) * (r[0] - p[0]));
  const onSegment = (p: Coordinate, q: Coordinate, r: Coordinate): boolean =>
    q[0] <= Math.max(p[0], r[0]) && q[0] >= Math.min(p[0], r[0]) &&
    q[1] <= Math.max(p[1], r[1]) && q[1] >= Math.min(p[1], r[1]);
  const o1 = orient(a, b, c);
  const o2 = orient(a, b, d);
  const o3 = orient(c, d, a);
  const o4 = orient(c, d, b);
  if (o1 !== o2 && o3 !== o4) {
    return true;
  }
  if (o1 === 0 && onSegment(a, c, b)) {
    return true;
  }
  if (o2 === 0 && onSegment(a, d, b)) {
    return true;
  }
  if (o3 === 0 && onSegment(c, a, d)) {
    return true;
  }
  if (o4 === 0 && onSegment(c, b, d)) {
    return true;
  }
  return false;
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
