package main

import (
	"fmt"
	"strings"
)

// generateDomainModel creates domain model struct based on table schema
func generateDomainModel(table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	var fields strings.Builder

	// MetaField always provides ID and the audit columns; the DTO layer relies on it
	fields.WriteString("\tMetaField\n")

	// Add table-specific fields
	for _, col := range table.Columns {
		fieldName := toCamelCase(col.Name)

		// Skip meta fields if they're explicitly defined
		if strings.ToLower(col.Name) == "id" ||
			strings.ToLower(col.Name) == "created_at" ||
			strings.ToLower(col.Name) == "updated_at" ||
			strings.ToLower(col.Name) == "deleted_at" ||
			strings.ToLower(col.Name) == "is_deleted" {
			continue
		}

		fields.WriteString(fmt.Sprintf("\t%s %s `%s %s`\n",
			fieldName, col.GoType, col.GormTag, col.JSONTag))
	}

	variables := map[string]string{
		"import_statement": getImportStatement(table),
		"struct_name":      structName,
		"entity_name":      strings.ToLower(structName),
		"fields":           fields.String(),
		"table_name":       table.Name,
	}

	result, err := processTemplate("domain-model", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process domain-model template: %v", err))
	}
	return result
}

// getImportStatement returns import statements based on table column types
func getImportStatement(table Table) string {
	hasTimeFields := false
	for _, col := range table.Columns {
		// Skip meta fields as they're handled by MetaField
		if strings.ToLower(col.Name) == "created_at" ||
			strings.ToLower(col.Name) == "updated_at" ||
			strings.ToLower(col.Name) == "deleted_at" {
			continue
		}
		if col.GoType == "time.Time" {
			hasTimeFields = true
			break
		}
	}
	if hasTimeFields {
		return `import "time"`
	}
	return ""
}

// getDTOImportStatement returns import statements for DTO files
func getDTOImportStatement(table Table) string {
	hasTimeFields := false
	for _, col := range table.Columns {
		// Skip meta fields and id as they're handled differently in DTOs
		if strings.ToLower(col.Name) == "id" ||
			strings.ToLower(col.Name) == "created_at" ||
			strings.ToLower(col.Name) == "updated_at" ||
			strings.ToLower(col.Name) == "deleted_at" ||
			strings.ToLower(col.Name) == "is_deleted" {
			continue
		}
		if col.GoType == "time.Time" {
			hasTimeFields = true
			break
		}
	}
	if hasTimeFields {
		return `	"time"`
	}
	return ""
}

// generateApplicationInterface creates repository interface for application layer
func generateApplicationInterface(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	variables := map[string]string{
		"module_name":  moduleName,
		"entity_name":  strings.ToLower(structName),
		"struct_name":  structName,
		"entity_param": strings.ToLower(structName),
	}

	result, err := processTemplate("application-interface", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process application-interface template: %v", err))
	}
	return result
}

// generateDTO creates DTO structs and methods based on table schema - FIXED VERSION
func generateDTO(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	// Generate DTO fields based on domain model fields
	var fields strings.Builder
	fields.WriteString("\tID int64 `json:\"id,omitempty\"`\n")

	// Generate field mappings for Marshal/Unmarshal methods
	var marshalFields strings.Builder
	var marshalPointerFields strings.Builder
	var unmarshalFields strings.Builder

	// Add table-specific fields
	for _, col := range table.Columns {
		// Skip meta fields as they're handled differently in DTOs
		if strings.ToLower(col.Name) == "id" ||
			strings.ToLower(col.Name) == "created_at" ||
			strings.ToLower(col.Name) == "updated_at" ||
			strings.ToLower(col.Name) == "deleted_at" ||
			strings.ToLower(col.Name) == "is_deleted" {
			continue
		}

		fieldName := toCamelCase(col.Name)
		jsonTag := fmt.Sprintf("`json:\"%s,omitempty\"`", strings.ToLower(col.Name))
		if col.Validate != "" {
			jsonTag = fmt.Sprintf("`json:\"%s,omitempty\" validate:\"%s\"`", strings.ToLower(col.Name), col.Validate)
		}

		if col.usesPointer() {
			// Optional number whose rules reject 0: a pointer tells "not sent" from "sent as 0"
			fields.WriteString(fmt.Sprintf("\t%s *%s %s\n", fieldName, col.GoType, jsonTag))
			marshalPointerFields.WriteString(fmt.Sprintf("\n\tif d.%[1]s != nil {\n\t\tdomainModel.%[1]s = *d.%[1]s\n\t}", fieldName))
			unmarshalFields.WriteString(fmt.Sprintf("\n\t{\n\t\tv := domainModel.%[1]s\n\t\td.%[1]s = &v\n\t}", fieldName))
			continue
		}

		fields.WriteString(fmt.Sprintf("\t%s %s %s\n", fieldName, col.GoType, jsonTag))

		// Add field mappings
		marshalFields.WriteString(fmt.Sprintf("\n\t\t%s: d.%s,", fieldName, fieldName))
		unmarshalFields.WriteString(fmt.Sprintf("\n\td.%s = domainModel.%s", fieldName, fieldName))
	}

	// Use singular table name for DTO struct and plural for collections (like Users []User)
	dtoStructName := structName
	pluralName := structName + "s"

	// Check if DTO needs time import
	dtoImportStatement := getDTOImportStatement(table)

	variables := map[string]string{
		"module_name":      moduleName,
		"dto_struct_name":  dtoStructName,
		"entity_name":      strings.ToLower(dtoStructName),
		"fields":           fields.String(),
		"plural_name":      pluralName,
		"struct_name":      structName,
		"import_statement": dtoImportStatement,
		"marshal_fields":   marshalFields.String(),
		"unmarshal_fields": unmarshalFields.String(),

		"marshal_pointer_fields": marshalPointerFields.String(),
	}

	result, err := processTemplate("dto", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process dto template: %v", err))
	}
	return result
}

// generateApplicationService creates concrete application service implementation
func generateApplicationService(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	serviceName := fmt.Sprintf("%sDomain", structName)
	repoFieldName := fmt.Sprintf("%sRepo", strings.ToLower(structName))

	variables := map[string]string{
		"module_name":     moduleName,
		"service_name":    serviceName,
		"entity_name":     strings.ToLower(structName),
		"struct_name":     structName,
		"repo_field_name": repoFieldName,
	}

	result, err := processTemplate("application-service", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process application-service template: %v", err))
	}
	return result
}

// generateInteractorService creates service interface for interactor layer
func generateInteractorService(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	serviceName := fmt.Sprintf("I%sService", structName)
	dtoName := structName
	dtoPlural := structName + "s"

	variables := map[string]string{
		"module_name":      moduleName,
		"service_name":     serviceName,
		"entity_name":      strings.ToLower(structName),
		"dto_plural_param": strings.ToLower(dtoPlural),
		"dto_plural":       dtoPlural,
		"dto_param":        strings.ToLower(dtoName),
		"dto_name":         dtoName,
	}

	result, err := processTemplate("interactor-service", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process interactor-service template: %v", err))
	}
	return result
}

// generateInteractorAdapter creates adapter implementation that connects interactor to application layer
func generateInteractorAdapter(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	serviceName := fmt.Sprintf("I%sService", structName)
	adapterName := fmt.Sprintf("%sAdapter", structName)
	appServiceName := fmt.Sprintf("%sDomain", structName)
	dtoName := structName
	dtoPlural := structName + "s"

	variables := map[string]string{
		"module_name":      moduleName,
		"adapter_name":     adapterName,
		"service_name":     serviceName,
		"app_service_name": strings.ToLower(appServiceName),
		"app_service_type": appServiceName,
		"entity_name":      strings.ToLower(structName),
		"dto_plural_param": strings.ToLower(dtoPlural),
		"dto_plural":       dtoPlural,
		"dto_param":        strings.ToLower(dtoName),
		"dto_name":         dtoName,
	}

	result, err := processTemplate("interactor-adapter", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process interactor-adapter template: %v", err))
	}
	return result
}

// generatePostgresRepository creates PostgreSQL repository implementation
func generatePostgresRepository(moduleName string, table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}

	repoName := fmt.Sprintf("%sRepo", structName)

	variables := map[string]string{
		"module_name":        moduleName,
		"repo_name":          repoName,
		"entity_name":        strings.ToLower(structName),
		"entity_name_plural": strings.ToLower(structName) + "s",
		"struct_name":        structName,
		"entity_param":       strings.ToLower(structName),
	}

	result, err := processTemplate("postgres-repository", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process postgres-repository template: %v", err))
	}
	return result
}
