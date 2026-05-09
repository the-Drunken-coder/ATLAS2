package blob

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func TestParseAndNormalize_InvalidJSONBytes(t *testing.T) {
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: []byte(`{"broken`)}
	err := NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Violations) != 1 || ve.Violations[0].Code != "INVALID_JSON" {
		t.Fatalf("want INVALID_JSON violation, got %#v", err)
	}
}

func TestParseAndNormalize_NonObjectRoot(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"array", `[]`},
		{"string", `"hi"`},
		{"number", `42`},
		{"bool", `true`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: []byte(tc.raw)}
			err := NormalizeEntity(entity, OperationCreate)
			var ve *ValidationError
			if !errors.As(err, &ve) || len(ve.Violations) != 1 {
				t.Fatalf("want single ValidationError, got %#v", err)
			}
			if ve.Violations[0].Code != "INVALID_TYPE" || ve.Violations[0].Field != "json" {
				t.Fatalf("want INVALID_TYPE on json, got %+v", ve.Violations[0])
			}
		})
	}
}

func TestNormalizeEntity_AssetNilJSONFailsSupportedCommands(t *testing.T) {
	entity := &model.Entity{EntityID: "asset-1", Type: model.EntityTypeAsset, JSON: nil}
	err := NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Field == "json.components.supported_commands" && v.Code == "REQUIRED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing supported_commands violation, got %#v", ve.Violations)
	}
}

func TestNormalize_RootBlobTooLarge(t *testing.T) {
	// Single large string value so raw UTF-8 length exceeds maxJSONBlobSize before decode.
	payload := []byte(`{"p":"` + strings.Repeat("y", maxJSONBlobSize) + `"}`)
	if len(payload) <= maxJSONBlobSize {
		t.Fatalf("test payload len=%d must exceed maxJSONBlobSize=%d", len(payload), maxJSONBlobSize)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: payload}
	err := NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Violations) != 1 {
		t.Fatalf("want single violation, got %#v", err)
	}
	if ve.Violations[0].Code != "TOO_LARGE" || ve.Violations[0].Field != "json" {
		t.Fatalf("want TOO_LARGE on json, got %+v", ve.Violations[0])
	}
}

func TestNormalize_CommonLimits_TooDeep(t *testing.T) {
	deepExtra := nestedObjectWraps(map[string]any{}, maxJSONDepth-1)
	raw, err := json.Marshal(map[string]any{"extra": deepExtra})
	if err != nil {
		t.Fatal(err)
	}
	obj := &model.Object{ObjectID: "o1", Type: model.ObjectTypeLog, JSON: raw}
	err = NormalizeObject(obj, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "TOO_DEEP" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want TOO_DEEP in %#v", ve.Violations)
	}
}

func TestNormalize_CommonLimits_KeyTooLong(t *testing.T) {
	longKey := strings.Repeat("k", maxJSONKeyLength+1)
	payload := map[string]any{
		"components": map[string]any{
			"supported_commands": map[string]any{"commands": []any{}},
		},
		"extra": map[string]any{longKey: true},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: raw}
	err = NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "KEY_TOO_LONG" && strings.Contains(v.Field, longKey[:8]) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want KEY_TOO_LONG under json.extra, got %#v", ve.Violations)
	}
}

func TestNormalize_CommonLimits_TooManyFields(t *testing.T) {
	extras := make(map[string]any, maxJSONFields+1)
	for i := 0; i <= maxJSONFields; i++ {
		extras["f"+strconv.Itoa(i)] = true
	}
	raw, err := json.Marshal(map[string]any{"extra": extras})
	if err != nil {
		t.Fatal(err)
	}
	obj := &model.Object{ObjectID: "o1", Type: model.ObjectTypeLog, JSON: raw}
	err = NormalizeObject(obj, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "TOO_MANY_FIELDS" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want TOO_MANY_FIELDS in %#v", ve.Violations)
	}
}

func TestNormalize_CustomSection_TooDeep(t *testing.T) {
	customVal := nestedObjectWraps(map[string]any{}, 8)
	payload := map[string]any{
		"components": map[string]any{
			"supported_commands": map[string]any{"commands": []any{}},
		},
		"extra":        map[string]any{},
		"custom_depth": customVal,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: raw}
	err = NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "TOO_DEEP" && strings.HasPrefix(v.Field, "json.custom_depth") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want TOO_DEEP under json.custom_depth, got %#v", ve.Violations)
	}
}

func TestNormalize_CustomSection_KeyTooLong(t *testing.T) {
	longKey := strings.Repeat("c", maxCustomKeyLen+1)
	payload := map[string]any{
		"components": map[string]any{
			"supported_commands": map[string]any{"commands": []any{}},
		},
		"extra":           map[string]any{},
		"custom_longkeys": map[string]any{longKey: 1},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: raw}
	err = NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "KEY_TOO_LONG" && strings.HasPrefix(v.Field, "json.custom_longkeys") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want KEY_TOO_LONG under custom section, got %#v", ve.Violations)
	}
}

func TestNormalize_CustomSection_TooManyFields(t *testing.T) {
	inner := make(map[string]any, maxCustomFields+1)
	for i := 0; i <= maxCustomFields; i++ {
		inner["c"+strconv.Itoa(i)] = i
	}
	payload := map[string]any{
		"components": map[string]any{
			"supported_commands": map[string]any{"commands": []any{}},
		},
		"extra":          map[string]any{},
		"custom_manyfld": inner,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: raw}
	err = NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Code == "TOO_MANY_FIELDS" && strings.HasPrefix(v.Field, "json.custom_manyfld") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want TOO_MANY_FIELDS under custom section, got %#v", ve.Violations)
	}
}

func TestNormalizeTask_ParametersMustBeObject(t *testing.T) {
	raw := []byte(`{"components":{"command":{"type":"move_to_location"},"parameters":[]},"extra":{}}`)
	task := &model.Task{TaskID: "t1", JSON: raw}
	err := NormalizeTask(task, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Field == "json.components.parameters" && v.Code == "INVALID_TYPE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want INVALID_TYPE on json.components.parameters, got %#v", ve.Violations)
	}
}

func TestNormalizeEntity_UnknownTopLevelKey(t *testing.T) {
	payload := map[string]any{
		"junk": true,
		"components": map[string]any{
			"supported_commands": map[string]any{"commands": []any{}},
		},
		"extra": map[string]any{},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	entity := &model.Entity{EntityID: "e1", Type: model.EntityTypeAsset, JSON: raw}
	err = NormalizeEntity(entity, OperationCreate)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	found := false
	for _, v := range ve.Violations {
		if v.Field == "json.junk" && v.Code == "UNKNOWN_FIELD" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want UNKNOWN_FIELD on json.junk, got %#v", ve.Violations)
	}
}

// nestedObjectWraps returns a map with a single-key chain of depth `layers`
// wrapping `leaf`. Walking from the outer map into `leaf` crosses depth layers+1.
func nestedObjectWraps(leaf map[string]any, layers int) map[string]any {
	cur := leaf
	for i := 0; i < layers; i++ {
		cur = map[string]any{"w": cur}
	}
	return cur
}
