package main

import (
	"fmt"
	"strconv"
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
		"valid_body":      strconv.Quote(validRequestBody(table)),

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
