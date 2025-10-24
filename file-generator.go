package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateAllFiles creates all template files for the hexagonal architecture
func generateAllFiles(moduleName string, tables []Table) error {
	// Generate base files
	if err := generateBaseFiles(moduleName, tables); err != nil {
		return err
	}

	// Generate domain models for each table
	if err := generateDomainModels(moduleName, tables); err != nil {
		return err
	}

	// Generate unified application layer interfaces (adapter.go)
	if err := generateUnifiedApplicationInterfacesFile(moduleName, tables); err != nil {
		return err
	}

	// Generate application service implementations
	if err := generateApplicationServices(moduleName, tables); err != nil {
		return err
	}

	// Generate DTOs
	if err := generateDTOs(moduleName, tables); err != nil {
		return err
	}

	// Generate unified interactor layer interfaces (adapter.go)
	if err := generateUnifiedInteractorInterfacesFile(moduleName, tables); err != nil {
		return err
	}

	// Generate interactor adapters
	if err := generateInteractorAdapters(moduleName, tables); err != nil {
		return err
	}

	// Generate repository implementations
	if err := generateRepositoryImplementations(moduleName, tables); err != nil {
		return err
	}

	// Generate REST API
	if err := generateRestAPI(moduleName, tables); err != nil {
		return err
	}

	// Generate Goose migration with schema support
	schema := ""
	if moduleName == "wearable-service" {
		schema = "wearable"
	}
	if err := generateGooseMigration(moduleName, tables, schema); err != nil {
		return err
	}

	fmt.Printf("Generated all files successfully!\n")
	return nil
}

// generateBaseFiles creates the basic configuration and setup files
func generateBaseFiles(moduleName string, tables []Table) error {
	files := map[string]string{
		// Go module file
		filepath.Join(moduleName, "go.mod"): generateGoMod(moduleName),

		// Main entry point
		filepath.Join(moduleName, "cmd", moduleName, "main.go"): generateMainGo(moduleName, tables),

		// README
		filepath.Join(moduleName, "README.md"): generateReadme(moduleName),

		// Environment configuration
		filepath.Join(moduleName, "internal", "config", "config.go"): generateConfig(moduleName),

		// Database connection
		filepath.Join(moduleName, "internal", "repository", "implementor", "postgres", "connection.go"): generateDBConnection(),

		// MetaField (common fields)
		filepath.Join(moduleName, "internal", "domain", "model", "meta.go"): generateMetaField(),

		// Docker files
		filepath.Join(moduleName, "Dockerfile"):         generateDockerfile(moduleName),
		filepath.Join(moduleName, "docker-compose.yml"): generateDockerCompose(moduleName),

		// Build scripts (Linux/macOS compatible)
		filepath.Join(moduleName, "script", "build.sh"):     generateBuildScript(moduleName),
		filepath.Join(moduleName, "script", "build-all.sh"): generateCrossPlatformBuildScript(moduleName),

		// Makefile
		filepath.Join(moduleName, "Makefile"): generateMakefile(moduleName),
	}

	for filePath, content := range files {
		if err := writeFile(filePath, content); err != nil {
			return err
		}
		fmt.Printf("Created: %s\n", filePath)
	}

	return nil
}

// generateDomainModels creates domain model structs for each table
func generateDomainModels(moduleName string, tables []Table) error {
	for _, table := range tables {
		modelContent := generateDomainModel(table)
		modelFile := filepath.Join(moduleName, "internal", "domain", "model", strings.ToLower(table.Name)+".go")

		if err := writeFile(modelFile, modelContent); err != nil {
			return err
		}
		fmt.Printf("Created domain model: %s\n", modelFile)
	}
	return nil
}

// generateUnifiedApplicationInterfacesFile creates a single adapter.go file with all interfaces
func generateUnifiedApplicationInterfacesFile(moduleName string, tables []Table) error {
	if len(tables) == 0 {
		return nil // Skip if no tables
	}

	interfaceContent := generateUnifiedApplicationInterfaces(moduleName, tables)
	interfaceFile := filepath.Join(moduleName, "internal", "application", "adapter.go")

	if err := writeFile(interfaceFile, interfaceContent); err != nil {
		return err
	}
	fmt.Printf("Created unified application interfaces: %s\n", interfaceFile)
	return nil
}

// generateApplicationInterfaces creates repository interfaces for application layer (LEGACY - kept for compatibility)
func generateApplicationInterfaces(moduleName string, tables []Table) error {
	for _, table := range tables {
		interfaceContent := generateApplicationInterface(moduleName, table)
		interfaceFile := filepath.Join(moduleName, "internal", "application", strings.ToLower(table.Name)+".go")

		if err := writeFile(interfaceFile, interfaceContent); err != nil {
			return err
		}
		fmt.Printf("Created application interface: %s\n", interfaceFile)
	}
	return nil
}

// generateApplicationServices creates concrete application service implementations
func generateApplicationServices(moduleName string, tables []Table) error {
	for _, table := range tables {
		structName := toCamelCase(table.Name)
		if strings.HasSuffix(structName, "s") {
			structName = structName[:len(structName)-1]
		}

		serviceContent := generateApplicationService(moduleName, table)
		serviceFile := filepath.Join(moduleName, "internal", "application", strings.ToLower(structName)+"_service.go")

		if err := writeFile(serviceFile, serviceContent); err != nil {
			return err
		}
		fmt.Printf("Created application service: %s\n", serviceFile)
	}
	return nil
}

// generateDTOs creates DTO structs for data transfer between layers
func generateDTOs(moduleName string, tables []Table) error {
	for _, table := range tables {
		structName := toCamelCase(table.Name)
		if strings.HasSuffix(structName, "s") {
			structName = structName[:len(structName)-1]
		}

		dtoContent := generateDTO(moduleName, table)
		dtoFile := filepath.Join(moduleName, "internal", "application", "dto", strings.ToLower(structName)+".go")

		if err := writeFile(dtoFile, dtoContent); err != nil {
			return err
		}
		fmt.Printf("Created DTO: %s\n", dtoFile)
	}
	return nil
}

// generateUnifiedInteractorInterfacesFile creates a single adapter.go file with all interactor interfaces
func generateUnifiedInteractorInterfacesFile(moduleName string, tables []Table) error {
	if len(tables) == 0 {
		return nil // Skip if no tables
	}

	interfaceContent := generateUnifiedInteractorInterfaces(moduleName, tables)
	interfaceFile := filepath.Join(moduleName, "internal", "interactor", "adapter.go")

	if err := writeFile(interfaceFile, interfaceContent); err != nil {
		return err
	}
	fmt.Printf("Created unified interactor interfaces: %s\n", interfaceFile)
	return nil
}

// generateInteractorServices creates service interfaces for interactor layer (LEGACY - kept for compatibility)
func generateInteractorServices(moduleName string, tables []Table) error {
	for _, table := range tables {
		serviceContent := generateInteractorService(moduleName, table)
		serviceFile := filepath.Join(moduleName, "internal", "interactor", strings.ToLower(table.Name)+"_service.go")

		if err := writeFile(serviceFile, serviceContent); err != nil {
			return err
		}
		fmt.Printf("Created interactor service: %s\n", serviceFile)
	}
	return nil
}

// generateInteractorAdapters creates adapter implementations for interactor layer
func generateInteractorAdapters(moduleName string, tables []Table) error {
	for _, table := range tables {
		structName := toCamelCase(table.Name)
		if strings.HasSuffix(structName, "s") {
			structName = structName[:len(structName)-1]
		}

		adapterContent := generateInteractorAdapter(moduleName, table)
		adapterFile := filepath.Join(moduleName, "internal", "interactor", strings.ToLower(structName)+"_adapter.go")

		if err := writeFile(adapterFile, adapterContent); err != nil {
			return err
		}
		fmt.Printf("Created interactor adapter: %s\n", adapterFile)
	}
	return nil
}

// generateRepositoryImplementations creates PostgreSQL repository implementations
func generateRepositoryImplementations(moduleName string, tables []Table) error {
	for _, table := range tables {
		repoContent := generatePostgresRepository(moduleName, table)
		repoFile := filepath.Join(moduleName, "internal", "repository", "implementor", "postgres", strings.ToLower(table.Name)+"_repo.go")

		if err := writeFile(repoFile, repoContent); err != nil {
			return err
		}
		fmt.Printf("Created repository implementation: %s\n", repoFile)
	}
	return nil
}

// generateRestAPI creates REST API handlers
func generateRestAPI(moduleName string, tables []Table) error {
	// Generate main REST API file
	restContent := generateRestAPIMain(moduleName, tables)
	restFile := filepath.Join(moduleName, "internal", "interactor", "rest", "rest.go")

	if err := writeFile(restFile, restContent); err != nil {
		return err
	}
	fmt.Printf("Created REST API: %s\n", restFile)

	// Generate REST parameter file
	parameterContent := generateRestParameter(moduleName, tables)
	parameterFile := filepath.Join(moduleName, "internal", "interactor", "rest", "rest_parameter.go")

	if err := writeFile(parameterFile, parameterContent); err != nil {
		return err
	}
	fmt.Printf("Created REST parameters: %s\n", parameterFile)

	// Generate individual handlers for each table
	for _, table := range tables {
		handlerContent := generateRestHandler(moduleName, table)
		handlerFile := filepath.Join(moduleName, "internal", "interactor", "rest", strings.ToLower(table.Name)+"_handler.go")

		if err := writeFile(handlerFile, handlerContent); err != nil {
			return err
		}
		fmt.Printf("Created REST handler: %s\n", handlerFile)
	}

	return nil
}

// writeFile writes content to a file, creating directories as needed
func writeFile(filePath, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	return nil
}

// writeExecutableFile writes content to an executable file, creating directories as needed
func writeExecutableFile(filePath, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Write file with executable permissions
	if err := os.WriteFile(filePath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write executable file %s: %v", filePath, err)
	}

	return nil
}
