package main

import "fmt"

// generateDockerfile creates Dockerfile content
func generateDockerfile(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate("dockerfile", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process dockerfile template: %v", err))
	}
	return result
}

// generateDockerCompose creates docker-compose.yml content
func generateDockerCompose(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate(dialect.templateName("docker-compose"), variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process docker-compose template: %v", err))
	}
	return result
}

// generateBuildScript creates build.sh script for Linux/macOS
func generateBuildScript(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate("build-script", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process build-script template: %v", err))
	}
	return result
}

// generateBuildScriptWindows creates build.bat script for Windows
func generateBuildScriptWindows(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate("build-script-windows", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process build-script-windows template: %v", err))
	}
	return result
}

// generateCrossPlatformBuildScript creates build script for all platforms
func generateCrossPlatformBuildScript(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate("build-script-cross-platform", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process build-script-cross-platform template: %v", err))
	}
	return result
}

// generateMakefile creates Makefile content
func generateMakefile(moduleName string) string {
	variables := map[string]string{
		"module_name": moduleName,
	}

	result, err := processTemplate("makefile", variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to process makefile template: %v", err))
	}
	return result
}
