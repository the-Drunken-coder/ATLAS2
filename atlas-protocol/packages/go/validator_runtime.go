package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates Atlas Protocol JSON payloads using embedded schemas
// (New) or schemas read from a protocol checkout root (NewWithProtocolRoot).
type Validator struct {
	protocolRoot string
	useEmbedded  bool

	mu       sync.Mutex
	compiler *jsonschema.Compiler
	initErr  error
}

func (v *Validator) ensureCompiler() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.compiler != nil {
		return v.initErr
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	var err error
	if v.useEmbedded {
		err = loadBundleIntoCompiler(c, schemaBundleJSON)
	} else {
		err = loadSchemaDirIntoCompiler(c, filepath.Join(v.protocolRoot, "source", "schemas"))
	}
	if err != nil {
		v.initErr = err
		return v.initErr
	}
	var entityDoc, objectDoc map[string]any
	if v.useEmbedded {
		entityDoc, err = schemaRootFromBundle(schemaBundleJSON, "entity")
		if err != nil {
			v.initErr = err
			return v.initErr
		}
		objectDoc, err = schemaRootFromBundle(schemaBundleJSON, "object")
		if err != nil {
			v.initErr = err
			return v.initErr
		}
	} else {
		schemaDir := filepath.Join(v.protocolRoot, "source", "schemas")
		entityDoc, err = readSchemaJSONFile(schemaDir, "entity.schema.json")
		if err != nil {
			v.initErr = err
			return v.initErr
		}
		objectDoc, err = readSchemaJSONFile(schemaDir, "object.schema.json")
		if err != nil {
			v.initErr = err
			return v.initErr
		}
	}
	if err := addVariantWrappers(c, entityDoc, "entity", []string{"asset", "track", "geofeature"}); err != nil {
		v.initErr = err
		return v.initErr
	}
	if err := addVariantWrappers(c, objectDoc, "object", []string{"log", "photo", "document", "observation_history", "track_provenance"}); err != nil {
		v.initErr = err
		return v.initErr
	}
	v.compiler = c
	return nil
}

func schemaRootFromBundle(raw []byte, key string) (map[string]any, error) {
	var bundle struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, err
	}
	doc, ok := bundle.Schemas[key]
	if !ok {
		return nil, fmt.Errorf("bundle missing schema key %q", key)
	}
	var root map[string]any
	if err := json.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func readSchemaJSONFile(dir, name string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func addVariantWrappers(c *jsonschema.Compiler, doc map[string]any, family string, variants []string) error {
	for _, variant := range variants {
		wrapper := map[string]any{
			"$schema": doc["$schema"],
			"title":   doc["title"],
			"$ref":    "#/$defs/" + variant,
			"$defs":   doc["$defs"],
		}
		url := "atlas://" + family + "/" + variant
		if err := c.AddResource(url, wrapper); err != nil {
			return fmt.Errorf("add wrapper %s: %w", url, err)
		}
		if _, err := c.Compile(url); err != nil {
			return fmt.Errorf("compile wrapper %s: %w", url, err)
		}
	}
	return nil
}

func loadBundleIntoCompiler(c *jsonschema.Compiler, raw []byte) error {
	var bundle struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("parse embedded schema bundle: %w", err)
	}
	keys := make([]string, 0, len(bundle.Schemas))
	for k := range bundle.Schemas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		doc := bundle.Schemas[key]
		var root any
		if err := json.Unmarshal(doc, &root); err != nil {
			return fmt.Errorf("parse schema %s: %w", key, err)
		}
		url := key + ".schema.json"
		if err := c.AddResource(url, root); err != nil {
			return fmt.Errorf("add schema %s: %w", url, err)
		}
		if _, err := c.Compile(url); err != nil {
			return fmt.Errorf("compile schema %s: %w", url, err)
		}
	}
	return nil
}

func loadSchemaDirIntoCompiler(c *jsonschema.Compiler, schemaDir string) error {
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return fmt.Errorf("read schema directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(schemaDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read schema %s: %w", entry.Name(), err)
		}
		var root any
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse schema %s: %w", entry.Name(), err)
		}
		if err := c.AddResource(entry.Name(), root); err != nil {
			return fmt.Errorf("add schema %s: %w", entry.Name(), err)
		}
		if _, err := c.Compile(entry.Name()); err != nil {
			return fmt.Errorf("compile schema %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (v *Validator) runSchemaURL(url string, value any) []ValidationIssue {
	if err := v.ensureCompiler(); err != nil {
		return []ValidationIssue{{Field: "json", Code: "invalid_value", Message: err.Error()}}
	}
	sch, err := v.compiler.Compile(url)
	if err != nil {
		return []ValidationIssue{{Field: "json", Code: "invalid_value", Message: err.Error()}}
	}
	if err := sch.Validate(value); err != nil {
		return mapSchemaValidateError(err)
	}
	return nil
}

func (v *Validator) ValidateBytes(kind ResourceKind, payload []byte, opts ...ValidateOption) []ValidationIssue {
	var o validateOptions
	for _, opt := range opts {
		opt(&o)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return []ValidationIssue{{Field: "json", Code: "invalid_json", Message: "json must be valid JSON"}}
	}
	root, ok := value.(map[string]any)
	if !ok {
		return []ValidationIssue{{Field: "json", Code: "invalid_type", Message: "json must be an object"}}
	}
	issues := collectLimitIssues("json", root, payload, rootMaxBytes, rootMaxDepth, rootMaxFields, rootMaxKeyLength)
	switch kind {
	case ResourceEntity:
		issues = append(issues, v.validateEntityWithSchema(root, o.variant)...)
	case ResourceTask:
		issues = append(issues, v.validateTaskWithSchema(root)...)
	case ResourceObservation:
		issues = append(issues, v.validateObservationWithSchema(root)...)
	case ResourceObject:
		issues = append(issues, v.validateObjectWithSchema(root, o.variant)...)
	case ResourceCommandCatalog:
		issues = append(issues, v.validateCommandCatalogWithSchema(root)...)
	case ResourceChangeEvent:
		issues = append(issues, v.validateChangeEventWithSchema(root)...)
	case ResourceCustomSection:
		issues = append(issues, validateCustomSection(root)...)
	default:
		issues = append(issues, ValidationIssue{Field: "resource", Code: "invalid_value", Message: fmt.Sprintf("unknown resource kind: %s", kind)})
	}
	return dedupe(issues)
}
