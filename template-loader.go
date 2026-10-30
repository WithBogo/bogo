package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getTemplateDirectory returns the directory for a given template
func getTemplateDirectory(templateName string) string {
	// Map template names to their respective directories
	templateDirs := map[string]string{
		// Application layer
		"application-interface-content": "application",
		"application-interface":         "application",
		"application-interfaces":        "application",
		"application-service":           "application",
		"dto":                           "application",

		// Interactor layer
		"interactor-adapter":           "interactor",
		"interactor-interface-content": "interactor",
		"interactor-interfaces":        "interactor",
		"interactor-service":           "interactor",

		// Domain layer
		"domain-model": "domain",
		"meta-field":   "domain",

		// Repository layer
		"postgres-repository": "repository",

		// REST layer
		"rest-api-main":         "rest",
		"rest-func-create":      "rest",
		"rest-func-delete":      "rest",
		"rest-func-get-all":     "rest",
		"rest-func-get-by-id":   "rest",
		"rest-func-update":      "rest",
		"rest-handler-header":   "rest",
		"rest-parameter-header": "rest",
		"rest-parameter":        "rest",
		"rest-routes":           "rest",

		// Base templates
		"go-mod":            "base",
		"main-go":           "base",
		"main-go-no-tables": "base",
		"readme":            "base",
		"config":            "base",
		"docs-go":           "base",
		"db-connection":     "base",

		// Migration templates
		"goose-migration": "migration",

		// Docker templates
		"dockerfile":                  "docker",
		"docker-compose":              "docker",
		"build-script":                "docker",
		"build-script-windows":        "docker",
		"build-script-cross-platform": "docker",
		"makefile":                    "docker",
	}

	if dir, exists := templateDirs[templateName]; exists {
		return dir
	}
	return "" // fallback to root templates directory
}

// loadTemplate reads a template file and returns its content
func loadTemplate(templateName string) (string, error) {
	dir := getTemplateDirectory(templateName)
	var templatePath string

	if dir != "" {
		templatePath = filepath.Join("templates", dir, templateName+".template")
	} else {
		templatePath = filepath.Join("templates", templateName+".template")
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templateName, err)
	}
	return string(content), nil
}

// replaceTemplateVariables replaces template variables with actual values
func replaceTemplateVariables(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("<%s>", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// processTemplate loads a template and replaces variables
func processTemplate(templateName string, variables map[string]string) (string, error) {
	template, err := loadTemplate(templateName)
	if err != nil {
		return "", err
	}

	return replaceTemplateVariables(template, variables), nil
}
