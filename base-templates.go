package main

import (
	"fmt"
	"strings"
)

// generateGoMod creates go.mod content using templates
func generateGoMod(moduleName string) string {
	variables := dialect.templateVariables(moduleName)
	variables["module_name"] = moduleName

	content, err := processTemplate("go-mod", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate go.mod: %v", err))
	}
	return content
}

// generateMainGo creates the main.go file using templates
func generateMainGo(moduleName string, tables []Table) string {
	if len(tables) == 0 {
		// When no tables, use the no-tables template
		panic(fmt.Sprintf("Failed to generate main.go (no tables)"))
	}

	// Build repository initialization code
	var repoInit strings.Builder
	var appServiceInit strings.Builder
	var adapterInit strings.Builder
	var serviceParams strings.Builder
	var endpoints strings.Builder
	var additionalImports strings.Builder

	additionalImports.WriteString(fmt.Sprintf("\n\t\"%s/internal/application\"", moduleName))
	additionalImports.WriteString(fmt.Sprintf("\n\t\"%s/internal/interactor\"", moduleName))
	additionalImports.WriteString(fmt.Sprintf("\n\t\"%s/internal/interactor/rest\"", moduleName))
	additionalImports.WriteString(fmt.Sprintf("\n\t\"%s/internal/repository/implementor/%s\"", moduleName, dialect.Package))

	repoInit.WriteString("\n\t// Initialize repositories\n")
	appServiceInit.WriteString("\n\t// Initialize application services\n")
	adapterInit.WriteString("\n\t// Initialize interactor adapters\n")

	for i, table := range tables {
		structName := toCamelCase(strings.TrimSuffix(table.Name, "s"))
		entityName := strings.ToLower(structName)
		entityPlural := strings.ToLower(table.Name)

		// Repository initialization
		repoInit.WriteString(fmt.Sprintf("\t%sRepo := %s.New%sRepo(db)\n", entityName, dialect.Package, structName))

		// Application service initialization
		appServiceInit.WriteString(fmt.Sprintf("\t%sAppService := application.New%sDomain(ctx, %sRepo)\n", entityName, structName, entityName))

		// Adapter initialization
		adapterInit.WriteString(fmt.Sprintf("\t%sAdapter := interactor.New%sAdapter(ctx, %sAppService)\n", entityName, structName, entityName))

		// Service parameters for REST API
		if i > 0 {
			serviceParams.WriteString(", ")
		}
		serviceParams.WriteString(fmt.Sprintf("%sAdapter", entityName))

		// Log endpoints
		endpoints.WriteString(fmt.Sprintf("\n\tlog.Info(\"  GET/POST /%s - %s management\")", entityPlural, structName))
		endpoints.WriteString(fmt.Sprintf("\n\tlog.Info(\"  GET/PUT/DELETE /%s/{id} - %s operations\")", entityPlural, structName))
	}

	variables := dialect.templateVariables(moduleName)
	for key, value := range map[string]string{
		"module_name":                        moduleName,
		"additional_imports":                 additionalImports.String(),
		"repository_initialization":          repoInit.String(),
		"application_service_initialization": appServiceInit.String(),
		"adapter_initialization":             adapterInit.String(),
		"service_parameters":                 ", " + serviceParams.String(),
		"endpoint_logging":                   endpoints.String(),
	} {
		variables[key] = value
	}

	content, err := processTemplate("main-go", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate main.go: %v", err))
	}
	return content
}

// generateReadme creates README.md content using templates
func generateReadme(moduleName string) string {
	variables := dialect.templateVariables(moduleName)
	variables["module_name"] = moduleName

	content, err := processTemplate("readme", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate README.md: %v", err))
	}
	return content
}

// generateConfig creates configuration structure using templates
func generateConfig(moduleName string) string {
	variables := dialect.templateVariables(moduleName)
	variables["module_name"] = moduleName

	content, err := processTemplate("config", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate config: %v", err))
	}
	return content
}

// generateDBConnection creates database connection code using templates
func generateDBConnection() string {
	variables := map[string]string{
		// No variables needed for this template
	}

	content, err := processTemplate(dialect.templateName("db-connection"), variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate DB connection: %v", err))
	}
	return content
}

// generateMetaField creates common fields for all models using templates
func generateMetaField() string {
	variables := map[string]string{
		// No variables needed for this template
	}

	content, err := processTemplate("meta-field", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate MetaField: %v", err))
	}
	return content
}
