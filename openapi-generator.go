package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// obj is a JSON object; encoding/json sorts map keys, so the generated spec is deterministic
type obj = map[string]any

const filterOperatorsHelp = "Supports operators: like(x), in(a,b), not_in(a,b), not(x), range(a,b), gt(x), gte(x), lt(x), lte(x)."

const responseErrorSchemaName = "ResponseError"

// openAPISchema maps a column to an OpenAPI schema
func openAPISchema(col Column) obj {
	switch col.GoType {
	case "int64":
		return obj{"type": "integer", "format": "int64"}
	case "float64":
		return obj{"type": "number", "format": "double"}
	case "bool":
		return obj{"type": "boolean"}
	case "time.Time":
		return obj{"type": "string", "format": "date-time"}
	case "[]string":
		return obj{"type": "array", "items": obj{"type": "string"}}
	}
	return obj{"type": "string"}
}

// openAPIEntityColumns returns the columns exposed through the DTO, excluding the
// meta fields that MetaField provides
func openAPIEntityColumns(table Table) []Column {
	var columns []Column
	for _, col := range table.Columns {
		switch strings.ToLower(col.Name) {
		case "id", "created_at", "updated_at", "deleted_at", "is_deleted":
			continue
		}
		columns = append(columns, col)
	}
	return columns
}

func openAPIEntitySchema(table Table) obj {
	properties := obj{"id": obj{"type": "integer", "format": "int64", "readOnly": true}}
	for _, col := range openAPIEntityColumns(table) {
		schema := openAPISchema(col)
		if col.IsNullable {
			schema["nullable"] = true
		}
		properties[strings.ToLower(col.Name)] = schema
	}
	return obj{"type": "object", "properties": properties}
}

func jsonResponse(description string, schema obj) obj {
	return obj{
		"description": description,
		"content":     obj{"application/json": obj{"schema": schema}},
	}
}

func errorResponse(description string) obj {
	return jsonResponse(description, obj{"$ref": "#/components/schemas/" + responseErrorSchemaName})
}

// envelope describes the response wrapper used by every handler: { data, message }
func envelope(data obj) obj {
	return obj{
		"type": "object",
		"properties": obj{
			"data":    data,
			"message": obj{"type": "string"},
		},
	}
}

// openAPIFilterParameters lists the query parameters accepted by the list endpoint,
// mirroring the generated filter configuration (id plus every non meta column)
func openAPIFilterParameters(table Table) []any {
	params := []any{
		obj{"name": "limit", "in": "query", "description": "Page size, defaults to 10", "schema": obj{"type": "integer", "minimum": 1, "default": 10}},
		obj{"name": "offset", "in": "query", "description": "Number of items to skip", "schema": obj{"type": "integer", "minimum": 0, "default": 0}},
		obj{"name": "sort", "in": "query", "description": "Sort order, for example asc(name),desc(id)", "schema": obj{"type": "string"}},
		obj{"name": "id", "in": "query", "description": "Filter by id. " + filterOperatorsHelp, "schema": obj{"type": "string"}},
	}
	for _, col := range openAPIEntityColumns(table) {
		switch col.Name {
		case "limit", "offset", "sort":
			continue
		}
		params = append(params, obj{
			"name":        col.Name,
			"in":          "query",
			"description": fmt.Sprintf("Filter by %s. %s", col.Name, filterOperatorsHelp),
			"schema":      obj{"type": "string"},
		})
	}
	return params
}

var idPathParameter = obj{
	"name": "id", "in": "path", "required": true,
	"schema": obj{"type": "integer", "format": "int64"},
}

// openAPIPaths builds the paths of one table
func openAPIPaths(table Table) (collectionPath, itemPath string, collection, item obj) {
	name := openAPIStructName(table)
	entity := strings.ToLower(name)
	plural := entity + "s"
	ref := obj{"$ref": "#/components/schemas/Entity" + name}
	tag := []string{name}

	collectionPath = "/" + plural
	itemPath = "/" + plural + "/{id}"

	collection = obj{
		"get": obj{
			"tags":        tag,
			"operationId": "getAll" + name + "s",
			"summary":     "List " + plural,
			"parameters":  openAPIFilterParameters(table),
			"responses": obj{
				"200": jsonResponse("Paginated list of "+plural, obj{
					"type": "object",
					"properties": obj{
						"data":    obj{"type": "array", "items": ref},
						"meta":    obj{"type": "object", "description": "Pagination info and links"},
						"message": obj{"type": "string"},
					},
				}),
				"400": errorResponse("Invalid filter or sort parameter"),
				"500": errorResponse("Unexpected error"),
			},
		},
		"post": obj{
			"tags":        tag,
			"operationId": "create" + name,
			"summary":     "Create a " + entity,
			"requestBody": obj{
				"required": true,
				"content":  obj{"application/json": obj{"schema": ref}},
			},
			"responses": obj{
				"201": jsonResponse("Created, returns the new id", envelope(obj{"type": "integer", "format": "int64"})),
				"400": errorResponse("Invalid request body"),
				"500": errorResponse("Unexpected error"),
			},
		},
	}

	item = obj{
		"get": obj{
			"tags":        tag,
			"operationId": "get" + name + "ByID",
			"summary":     "Get a " + entity + " by id",
			"parameters":  []any{idPathParameter},
			"responses": obj{
				"200": jsonResponse("The "+entity, envelope(ref)),
				"400": errorResponse("Invalid id"),
				"404": errorResponse(name + " not found"),
				"500": errorResponse("Unexpected error"),
			},
		},
		"put": obj{
			"tags":        tag,
			"operationId": "update" + name,
			"summary":     "Update a " + entity,
			"parameters":  []any{idPathParameter},
			"requestBody": obj{
				"required": true,
				"content":  obj{"application/json": obj{"schema": ref}},
			},
			"responses": obj{
				"200": jsonResponse("The updated "+entity, envelope(ref)),
				"400": errorResponse("Invalid id or request body"),
				"404": errorResponse(name + " not found"),
				"500": errorResponse("Unexpected error"),
			},
		},
		"delete": obj{
			"tags":        tag,
			"operationId": "delete" + name,
			"summary":     "Delete a " + entity,
			"parameters":  []any{idPathParameter},
			"responses": obj{
				"200": jsonResponse("Deleted, returns the id", envelope(obj{
					"type":       "object",
					"properties": obj{"id": obj{"type": "integer", "format": "int64"}},
				})),
				"400": errorResponse("Invalid id"),
				"404": errorResponse(name + " not found"),
				"500": errorResponse("Unexpected error"),
			},
		},
	}
	return
}

// generateOpenAPISpec builds an OpenAPI 3.0 document for the generated REST API
func generateOpenAPISpec(moduleName string, tables []Table) (string, error) {
	paths := obj{}
	schemas := obj{
		responseErrorSchemaName: obj{
			"type": "object",
			"properties": obj{
				"error":   obj{"description": "Technical error detail"},
				"message": obj{"type": "string"},
			},
		},
	}
	var tags []any

	for _, table := range tables {
		name := openAPIStructName(table)
		collectionPath, itemPath, collection, item := openAPIPaths(table)
		paths[collectionPath] = collection
		paths[itemPath] = item
		schemas["Entity"+name] = openAPIEntitySchema(table)
		tags = append(tags, obj{"name": name})
	}

	spec := obj{
		"openapi": "3.0.3",
		"info": obj{
			"title":       moduleName,
			"version":     "1.0.0",
			"description": "REST API generated by boGO",
		},
		"tags":  tags,
		"paths": paths,
		"components": obj{
			"schemas": schemas,
			"securitySchemes": obj{
				"bearerAuth": obj{"type": "http", "scheme": "bearer"},
			},
		},
		"security": []any{obj{"bearerAuth": []any{}}},
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAPI spec: %v", err)
	}
	return string(data) + "\n", nil
}

// generateOpenAPIFiles writes the spec and the Go package that serves it
func generateOpenAPIFiles(moduleName string, tables []Table) error {
	if len(tables) == 0 {
		return nil
	}

	spec, err := generateOpenAPISpec(moduleName, tables)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(moduleName, "docs", "openapi.json"), spec); err != nil {
		return err
	}

	docsGo, err := processTemplate("docs-go", map[string]string{"module_name": moduleName})
	if err != nil {
		return fmt.Errorf("failed to process docs-go template: %v", err)
	}
	return writeFile(filepath.Join(moduleName, "docs", "docs.go"), docsGo)
}

// openAPIStructName returns the Go struct name used for a table (singular, CamelCase)
func openAPIStructName(table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}
	return structName
}
