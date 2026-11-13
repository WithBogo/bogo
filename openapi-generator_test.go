package main

import (
	"encoding/json"
	"testing"
)

func sampleOpenAPITables() []Table {
	return []Table{{
		Name: "users",
		Columns: []Column{
			{Name: "name", GoType: "string", Type: "VARCHAR(255)", IsNullable: false},
			{Name: "age", GoType: "int64", Type: "INT", IsNullable: true},
			{Name: "score", GoType: "float64", Type: "DECIMAL", IsNullable: true},
			{Name: "active", GoType: "bool", Type: "BOOLEAN", IsNullable: true},
			{Name: "birthday", GoType: "time.Time", Type: "DATE", IsNullable: true},
			{Name: "seen_at", GoType: "time.Time", Type: "TIMESTAMPTZ", IsNullable: true},
			{Name: "tags", GoType: "[]string", Type: "JSONB", IsNullable: true},
			{Name: "created_at", GoType: "time.Time", Type: "TIMESTAMPTZ"},
		},
	}}
}

func decodeSpec(t *testing.T, tables []Table) map[string]any {
	t.Helper()
	raw, err := generateOpenAPISpec("user-service", tables)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
	return spec
}

func lookup(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()
	current := root
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("missing %q in path %v", key, path)
		}
		current = next
	}
	return current
}

func TestGenerateOpenAPISpec_Paths(t *testing.T) {
	spec := decodeSpec(t, sampleOpenAPITables())

	if spec["openapi"] != "3.0.3" {
		t.Errorf("unexpected openapi version %v", spec["openapi"])
	}

	collection := lookup(t, spec, "paths", "/users")
	for _, method := range []string{"get", "post"} {
		if _, ok := collection[method]; !ok {
			t.Errorf("/users is missing %s", method)
		}
	}

	item := lookup(t, spec, "paths", "/users/{id}")
	for _, method := range []string{"get", "put", "delete"} {
		if _, ok := item[method]; !ok {
			t.Errorf("/users/{id} is missing %s", method)
		}
	}
}

func TestGenerateOpenAPISpec_SchemaTypes(t *testing.T) {
	spec := decodeSpec(t, sampleOpenAPITables())
	props := lookup(t, spec, "components", "schemas", "EntityUser", "properties")

	cases := map[string][2]string{
		"id":       {"integer", "int64"},
		"name":     {"string", ""},
		"age":      {"integer", "int64"},
		"score":    {"number", "double"},
		"active":   {"boolean", ""},
		"birthday": {"string", "date-time"},
		"seen_at":  {"string", "date-time"},
		"tags":     {"array", ""},
	}
	for name, want := range cases {
		prop := lookup(t, props, name)
		if prop["type"] != want[0] {
			t.Errorf("%s: type %v, want %s", name, prop["type"], want[0])
		}
		if want[1] != "" && prop["format"] != want[1] {
			t.Errorf("%s: format %v, want %s", name, prop["format"], want[1])
		}
	}

	if _, ok := props["created_at"]; ok {
		t.Error("meta fields are not part of the DTO and must not be in the schema")
	}
	if lookup(t, props, "name")["nullable"] == true {
		t.Error("NOT NULL columns must not be nullable")
	}
	if lookup(t, props, "age")["nullable"] != true {
		t.Error("nullable columns must be marked nullable")
	}
}

func TestGenerateOpenAPISpec_FilterParameters(t *testing.T) {
	spec := decodeSpec(t, sampleOpenAPITables())
	get := lookup(t, spec, "paths", "/users", "get")

	names := map[string]bool{}
	for _, p := range get["parameters"].([]any) {
		names[p.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"limit", "offset", "sort", "id", "name", "age", "tags"} {
		if !names[want] {
			t.Errorf("list endpoint is missing the %q parameter", want)
		}
	}
	if names["created_at"] {
		t.Error("meta fields are not filterable")
	}
}

func TestGenerateOpenAPISpec_Deterministic(t *testing.T) {
	first, _ := generateOpenAPISpec("svc", sampleOpenAPITables())
	second, _ := generateOpenAPISpec("svc", sampleOpenAPITables())
	if first != second {
		t.Error("the spec must be byte for byte identical across runs, or regeneration reports false changes")
	}
}

func TestGenerateOpenAPISpec_ErrorTableSchema(t *testing.T) {
	tables := append(sampleOpenAPITables(), Table{
		Name:    "errors",
		Columns: []Column{{Name: "code", GoType: "string", Type: "VARCHAR(255)"}},
	})
	spec := decodeSpec(t, tables)

	schemas := lookup(t, spec, "components", "schemas")
	if _, ok := schemas["EntityError"]; !ok {
		t.Fatal("the errors table schema is missing")
	}
	if _, ok := schemas[responseErrorSchemaName]; !ok {
		t.Fatal("the shared error response schema is missing")
	}
	if got := lookup(t, schemas, "EntityError", "properties", "code")["type"]; got != "string" {
		t.Errorf("errors table code property type = %v, want string", got)
	}

	for _, path := range []string{"/users", "/errors"} {
		errorSchema := lookup(t, spec, "paths", path, "get", "responses", "400", "content", "application/json", "schema")
		if got, want := errorSchema["$ref"], "#/components/schemas/"+responseErrorSchemaName; got != want {
			t.Errorf("%s 400 response schema ref = %v, want %s", path, got, want)
		}
	}
}

func TestGenerateOpenAPISpec_ListDataNullableAndVersionConflict(t *testing.T) {
	tables := append(sampleOpenAPITables(), Table{
		Name: "accounts",
		Columns: []Column{
			{Name: "name", GoType: "string", Type: "VARCHAR(255)"},
			{Name: "version", GoType: "int64", Type: "BIGINT"},
		},
	})
	spec := decodeSpec(t, tables)

	data := lookup(t, spec, "paths", "/users", "get", "responses", "200", "content", "application/json", "schema", "properties", "data")
	if data["nullable"] != true {
		t.Error("list data must be nullable: a nil slice is serialized as null")
	}
	if _, ok := lookup(t, spec, "paths", "/accounts/{id}", "put", "responses")["409"]; !ok {
		t.Error("PUT on a table with a version column must document 409")
	}
	if _, ok := lookup(t, spec, "paths", "/users/{id}", "put", "responses")["409"]; ok {
		t.Error("PUT on a table without a version column must not document 409")
	}
}
