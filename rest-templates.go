package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateRestAPIMain creates the main REST API file
func generateRestAPIMain(moduleName string, tables []Table) string {
	var serviceFields strings.Builder
	var serviceParams strings.Builder
	var serviceInit strings.Builder
	var routeRegistrations strings.Builder
	var interactorImport string

	// Only import interactor package if there are tables
	if len(tables) > 0 {
		interactorImport = fmt.Sprintf("\n\n\t\"%s/internal/interactor\"", moduleName)
	}

	// Generate service interfaces and initialization for each table
	for i, table := range tables {
		structName := toCamelCase(table.Name)
		if strings.HasSuffix(structName, "s") {
			structName = structName[:len(structName)-1]
		}

		interfaceName := fmt.Sprintf("interactor.I%sService", structName)
		fieldName := fmt.Sprintf("%sService", strings.ToLower(structName))
		entityName := strings.ToLower(structName)
		entityPlural := entityName + "s"

		serviceFields.WriteString(fmt.Sprintf("\t%s %s\n", fieldName, interfaceName))

		if i > 0 {
			serviceParams.WriteString(", ")
		}
		serviceParams.WriteString(fmt.Sprintf("%s %s", fieldName, interfaceName))

		serviceInit.WriteString(fmt.Sprintf("\t\t%s: %s,\n", fieldName, fieldName))

		// Generate route registrations using template
		routeVars := map[string]string{
			"struct_name":   structName,
			"entity_name":   entityName,
			"entity_plural": entityPlural,
			"plural_name":   structName + "s",
			"field_name":    fieldName,
		}

		routeResult, err := processTemplate("rest-routes", routeVars)
		if err != nil {
			panic(fmt.Sprintf("Error processing rest-routes template: %v", err))
		}
		routeRegistrations.WriteString("\n")
		routeRegistrations.WriteString(routeResult)
	}

	serviceParamsStr := ""
	if serviceParams.Len() > 0 {
		serviceParamsStr = ", " + serviceParams.String()
	}

	vars := map[string]string{
		"module_name":         moduleName,
		"service_fields":      serviceFields.String(),
		"service_params":      serviceParamsStr,
		"service_init":        serviceInit.String(),
		"route_registrations": routeRegistrations.String(),
		"interactor_import":   interactorImport,
	}

	result, err := processTemplate("rest-api-main", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-api-main template: %v", err))
	}
	return result
}

// generateRestHandler creates REST handler for individual table
func generateRestHandler(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	entityName := strings.ToLower(structName)
	entityPlural := entityName + "s"
	singularName := structName
	pluralName := structName + "s"
	dtoName := structName
	entityVar := entityName

	vars := map[string]string{
		"module_name":     moduleName,
		"struct_name":     structName,
		"entity_singular": entityName,
		"entity_plural":   entityPlural,
		"entity_snake":    entityName,
		"singular_name":   singularName,
		"plural_name":     pluralName,
		"dto_name":        dtoName,
		"entity_var":      entityVar,
	}

	var handler strings.Builder

	// Package and imports
	headerResult, err := processTemplate("rest-handler-header", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-handler-header template: %v", err))
	}
	handler.WriteString(headerResult)

	// Individual handler methods
	getAllResult, err := processTemplate("rest-func-get-all", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-func-get-all template: %v", err))
	}
	handler.WriteString("\n")
	handler.WriteString(getAllResult)

	createResult, err := processTemplate("rest-func-create", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-func-create template: %v", err))
	}
	handler.WriteString("\n")
	handler.WriteString(createResult)

	getByIDResult, err := processTemplate("rest-func-get-by-id", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-func-get-by-id template: %v", err))
	}
	handler.WriteString("\n")
	handler.WriteString(getByIDResult)

	updateResult, err := processTemplate("rest-func-update", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-func-update template: %v", err))
	}
	handler.WriteString("\n")
	handler.WriteString(updateResult)

	deleteResult, err := processTemplate("rest-func-delete", vars)
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-func-delete template: %v", err))
	}
	handler.WriteString("\n")
	handler.WriteString(deleteResult)

	return handler.String()
}

// generateRestParameter creates the REST parameter file and JSON config files
func generateRestParameter(moduleName string, tables []Table) string {
	// Add package header and imports using template
	headerResult, err := processTemplate("rest-parameter-header", map[string]string{})
	if err != nil {
		panic(fmt.Sprintf("Error processing rest-parameter-header template: %v", err))
	}

	// Generate JSON config files for each table
	for _, table := range tables {
		structName := toCamelCase(table.Name)
		if strings.HasSuffix(structName, "s") {
			structName = structName[:len(structName)-1]
		}

		entitySnake := strings.ToLower(structName)
		generateRestConfigFile(table, entitySnake, moduleName)
	}

	return headerResult
}

// generateRestConfigFile creates a JSON configuration file for a specific entity
func generateRestConfigFile(table Table, entitySnake string, moduleName string) {
	type QueryField struct {
		DBKey     string `json:"db_key"`
		QueryKey  string `json:"query_key"`
		Kind      string `json:"kind"`
		Omitempty bool   `json:"omitempty,omitempty"`
	}

	type QueryConfig struct {
		Filter  []QueryField `json:"filter"`
		Sorting []QueryField `json:"sorting"`
	}

	var config QueryConfig

	// Add ID field first
	idField := QueryField{
		DBKey:     "id",
		QueryKey:  "id",
		Kind:      "int64",
		Omitempty: true,
	}
	config.Filter = append(config.Filter, idField)

	idSortField := QueryField{
		DBKey:    "id",
		QueryKey: "id",
		Kind:     "int64",
	}
	config.Sorting = append(config.Sorting, idSortField)

	// Add other fields
	for _, col := range table.Columns {
		if strings.ToLower(col.Name) == "id" ||
			strings.ToLower(col.Name) == "created_at" ||
			strings.ToLower(col.Name) == "updated_at" ||
			strings.ToLower(col.Name) == "deleted_at" ||
			strings.ToLower(col.Name) == "is_deleted" {
			continue
		}

		kindType := "string"
		if col.GoType == "int64" {
			kindType = "int64"
		} else if col.GoType == "float64" {
			kindType = "float64"
		} else if col.GoType == "bool" {
			kindType = "bool"
		}

		filterField := QueryField{
			DBKey:     col.Name,
			QueryKey:  col.Name,
			Kind:      kindType,
			Omitempty: true,
		}
		config.Filter = append(config.Filter, filterField)

		sortField := QueryField{
			DBKey:    col.Name,
			QueryKey: col.Name,
			Kind:     kindType,
		}
		config.Sorting = append(config.Sorting, sortField)
	}

	// Convert to pretty JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Error marshaling config for %s: %v", entitySnake, err))
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Join(moduleName, "config", "rest")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating config directory: %v", err))
	}

	// Write config file
	configFile := filepath.Join(configDir, entitySnake+".json")
	if err := os.WriteFile(configFile, jsonData, 0644); err != nil {
		panic(fmt.Sprintf("Error writing config file %s: %v", configFile, err))
	}

	fmt.Printf("Created REST config: %s\n", configFile)
}
