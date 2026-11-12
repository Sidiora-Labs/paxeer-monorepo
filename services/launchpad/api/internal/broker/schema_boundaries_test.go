package broker

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestStreamSchemaValidatorHonorsFrozenValueConstraints(t *testing.T) {
	for _, check := range []struct {
		name   string
		schema map[string]any
		value  any
		valid  bool
	}{
		{"reference absent", map[string]any{"$ref": "#/components/schemas/Absent"}, nil, false},
		{"one valid variant", map[string]any{"anyOf": []any{map[string]any{"type": "null"}, map[string]any{"type": "string"}}}, "ready", true},
		{"no valid variant", map[string]any{"anyOf": []any{map[string]any{"type": "null"}, map[string]any{"type": "boolean"}}}, "ready", false},
		{"constant", map[string]any{"type": "string", "const": "ready"}, "other", false},
		{"enumeration", map[string]any{"type": "string", "enum": []any{"ready"}}, "other", false},
		{"missing type", map[string]any{}, "ready", false},
		{"null type", map[string]any{"type": "null"}, "ready", false},
		{"boolean type", map[string]any{"type": "boolean"}, "ready", false},
		{"string type", map[string]any{"type": "string"}, true, false},
		{"minimum length", map[string]any{"type": "string", "minLength": float64(2)}, "a", false},
		{"maximum length", map[string]any{"type": "string", "maxLength": float64(2)}, "abc", false},
		{"string pattern", map[string]any{"type": "string", "pattern": "^[a-z]+$"}, "ABC", false},
		{"date time", map[string]any{"type": "string", "format": "date-time"}, "2026-10-12", false},
		{"uuid", map[string]any{"type": "string", "format": "uuid"}, "not-a-uuid", false},
		{"uri", map[string]any{"type": "string", "format": "uri"}, "http://example.test", false},
		{"unsupported format", map[string]any{"type": "string", "format": "hostname"}, "example.test", false},
		{"integer type", map[string]any{"type": "integer"}, 1, false},
		{"integer syntax", map[string]any{"type": "integer"}, json.Number("1.5"), false},
		{"integer minimum", map[string]any{"type": "integer", "minimum": float64(1)}, json.Number("0"), false},
		{"integer maximum", map[string]any{"type": "integer", "maximum": float64(1)}, json.Number("2"), false},
		{"integer within bounds", map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(2)}, json.Number("2"), true},
		{"object type", map[string]any{"type": "object"}, "ready", false},
		{"required property", map[string]any{"type": "object", "required": []any{"id"}}, map[string]any{}, false},
		{"additional property", map[string]any{"type": "object", "additionalProperties": false}, map[string]any{"id": "ready"}, false},
		{"nested property", map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}, map[string]any{"id": 1}, false},
		{"array type", map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "ready", false},
		{"array minimum", map[string]any{"type": "array", "minItems": float64(1), "items": map[string]any{"type": "string"}}, []any{}, false},
		{"array maximum", map[string]any{"type": "array", "maxItems": float64(1), "items": map[string]any{"type": "string"}}, []any{"a", "b"}, false},
		{"array items absent", map[string]any{"type": "array"}, []any{"a"}, false},
		{"array item type", map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, []any{1}, false},
		{"array accepted", map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, []any{"a"}, true},
		{"unsupported type", map[string]any{"type": "number"}, 1, false},
	} {
		t.Run(check.name, func(t *testing.T) {
			err := validateSchema(check.schema, check.value, 0)
			if check.valid && err != nil || !check.valid && !errors.Is(err, ErrFrame) {
				t.Fatalf("schema result: %v, valid=%t", err, check.valid)
			}
		})
	}
	if err := validateSchema(map[string]any{"type": "null"}, nil, 65); !errors.Is(err, ErrFrame) {
		t.Fatalf("nested schema depth: %v", err)
	}
}
