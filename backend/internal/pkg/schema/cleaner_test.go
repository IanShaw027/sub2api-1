package schema

import "testing"

func TestCleanJSONSchema_FilterUnsupportedFields(t *testing.T) {
	input := map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"pattern":              "^root$",
		"format":               "date-time",
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"pattern":   "^[a-z]+$",
				"format":    "email",
				"minLength": 1,
			},
		},
	}

	cleaned := CleanJSONSchema(input)

	for _, key := range []string{"additionalProperties", "pattern", "format"} {
		if _, ok := cleaned[key]; ok {
			t.Fatalf("expected %q to be removed from root", key)
		}
	}

	props, ok := cleaned["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties to be a map, got %T", cleaned["properties"])
	}
	nameSchema, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties.name to be a map, got %T", props["name"])
	}
	for _, key := range []string{"pattern", "format", "minLength"} {
		if _, ok := nameSchema[key]; ok {
			t.Fatalf("expected %q to be removed from properties.name", key)
		}
	}
}

func TestCleanJSONSchema_TypeNormalization(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{
			name: "string type",
			in:   map[string]any{"type": "string"},
			want: "STRING",
		},
		{
			name: "array type",
			in:   map[string]any{"type": []any{"null", "integer"}},
			want: "INTEGER",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned := CleanJSONSchema(tc.in)
			got, ok := cleaned["type"].(string)
			if !ok {
				t.Fatalf("expected type to be string, got %T", cleaned["type"])
			}
			if got != tc.want {
				t.Fatalf("expected type %q, got %q", tc.want, got)
			}
		})
	}
}

func TestCleanJSONSchema_RequiredFiltering(t *testing.T) {
	cases := []struct {
		name     string
		input    map[string]any
		want     []any
		wantGone bool
	}{
		{
			name: "keep only existing properties",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []any{"name", "missing"},
			},
			want: []any{"name"},
		},
		{
			name: "drop required when none exist",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"missing"},
			},
			wantGone: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned := CleanJSONSchema(tc.input)
			required, ok := cleaned["required"]
			if tc.wantGone {
				if ok {
					t.Fatalf("expected required to be removed, got %v", required)
				}
				return
			}
			if !ok {
				t.Fatalf("expected required to exist")
			}
			got, ok := required.([]any)
			if !ok {
				t.Fatalf("expected required to be []any, got %T", required)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("expected required length %d, got %d", len(tc.want), len(got))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("expected required[%d] %v, got %v", i, tc.want[i], got[i])
				}
			}
		})
	}
}
