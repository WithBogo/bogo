package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

// isMetaColumn reports whether a column is handled by MetaField and absent from the DTO
func isMetaColumn(name string) bool {
	switch strings.ToLower(name) {
	case "id", "created_at", "updated_at", "deleted_at", "is_deleted":
		return true
	}
	return false
}

// dtoColumns returns the columns that appear as DTO fields
func dtoColumns(table Table) []Column {
	var columns []Column
	for _, col := range table.Columns {
		if !isMetaColumn(col.Name) {
			columns = append(columns, col)
		}
	}
	return columns
}

// sampleJSONValue returns a JSON literal that satisfies the validation rules of the column
func sampleJSONValue(col Column) string {
	switch col.GoType {
	case "string":
		switch {
		case len(col.OneOf) > 0:
			return fmt.Sprintf("%q", col.OneOf[0])
		case strings.HasPrefix(strings.ToUpper(col.Type), "UUID"):
			return `"123e4567-e89b-12d3-a456-426614174000"`
		}
		return `"x"`
	case "int64", "float64":
		return formatBound(sampleNumber(col))
	case "bool":
		return "true"
	case "time.Time":
		return `"2024-01-01T00:00:00Z"`
	case "[]string":
		return `["a"]`
	}
	return `"x"`
}

// sampleNumber picks a number inside the CHECK bounds of the column, preferring 1
func sampleNumber(col Column) float64 {
	if len(col.OneOf) > 0 && col.GoType == "int64" && allIntegers(col.OneOf) {
		var first float64
		fmt.Sscan(col.OneOf[0], &first)
		return first
	}

	v := 1.0
	if col.Gte != nil && v < *col.Gte {
		v = *col.Gte
	}
	if col.Gt != nil && v <= *col.Gt {
		v = math.Floor(*col.Gt) + 1
	}
	if col.Lte != nil && v > *col.Lte {
		v = *col.Lte
	}
	if col.Lt != nil && v >= *col.Lt {
		v = math.Ceil(*col.Lt) - 1
	}
	return v
}

// validRequestBody builds a JSON body that passes every validation rule
func validRequestBody(table Table) string {
	var fields []string
	for _, col := range dtoColumns(table) {
		fields = append(fields, fmt.Sprintf("%q:%s", strings.ToLower(col.Name), sampleJSONValue(col)))
	}
	return "{" + strings.Join(fields, ",") + "}"
}

// validationFailureTest returns a handler test that sends an invalid body, or "" when the
// table has no rule that a request could violate
func validationFailureTest(table Table, structName, singularName string) string {
	var bodyExpr string
	for _, col := range dtoColumns(table) {
		if strings.HasPrefix(col.Validate, "required") {
			bodyExpr = "`{}`" // a missing required field
			break
		}
	}
	if bodyExpr == "" {
		for _, col := range dtoColumns(table) {
			if col.GoType == "string" && col.MaxLength > 0 {
				// one character longer than the column allows
				bodyExpr = fmt.Sprintf("`{\"%s\":\"` + strings.Repeat(\"x\", %d) + `\"}`",
					strings.ToLower(col.Name), col.MaxLength+1)
				break
			}
		}
	}
	if bodyExpr == "" {
		return ""
	}

	return fmt.Sprintf(`
	t.Run("rejects a body that violates validation rules", func(t *testing.T) {
		svc := &mock%[1]sService{}
		h := New%[1]sHandler(context.Background(), svc)

		rec := do%[1]sRequest(h.Create%[2]s, http.MethodPost, %[3]s, nil)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %%d", rec.Code)
		}
		if svc.createCalls != 0 {
			t.Error("service must not be called when validation fails")
		}
	})
`, structName, singularName, bodyExpr)
}

// generateServiceTest creates unit tests for the application service of a table
func generateServiceTest(moduleName string, table Table) string {
	structName := singularStructName(table)

	variables := map[string]string{
		"module_name":  moduleName,
		"struct_name":  structName,
		"entity_name":  strings.ToLower(structName),
		"service_name": fmt.Sprintf("%sDomain", structName),
	}

	result, err := processTemplate("service-test", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process service-test template: %v", err))
	}
	return result
}

// generateRestHandlerTest creates unit tests for the REST handler of a table
func generateRestHandlerTest(moduleName string, table Table) string {
	structName := singularStructName(table)
	entityName := strings.ToLower(structName)

	variables := map[string]string{
		"module_name":     moduleName,
		"struct_name":     structName,
		"entity_singular": entityName,
		"entity_plural":   entityName + "s",
		"singular_name":   structName,
		"plural_name":     structName + "s",
		"valid_body":      validRequestBody(table),

		"validation_failure_test": validationFailureTest(table, structName, structName),
	}

	result, err := processTemplate("rest-handler-test", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process rest-handler-test template: %v", err))
	}
	return result
}

// singularStructName returns the Go struct name used for a table
func singularStructName(table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}
	return structName
}

// generateTests writes unit tests for every service and REST handler
func generateTests(moduleName string, tables []Table) error {
	for _, table := range tables {
		structName := singularStructName(table)

		serviceTestFile := filepath.Join(moduleName, "internal", "application", strings.ToLower(structName)+"_service_test.go")
		if err := writeFile(serviceTestFile, generateServiceTest(moduleName, table)); err != nil {
			return err
		}

		handlerTestFile := filepath.Join(moduleName, "internal", "interactor", "rest", strings.ToLower(table.Name)+"_handler_test.go")
		if err := writeFile(handlerTestFile, generateRestHandlerTest(moduleName, table)); err != nil {
			return err
		}
	}
	return nil
}
