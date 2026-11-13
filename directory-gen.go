package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// createHexagonalArchitecture creates the complete hexagonal architecture
func createHexagonalArchitecture(moduleName, sqlSchemaFile string) error {
	// Parse SQL schema to extract table information
	tables, err := parseSQLSchema(sqlSchemaFile)
	if err != nil {
		return fmt.Errorf("failed to parse SQL schema: %v", err)
	}

	fmt.Printf("Found %d tables in SQL schema\n", len(tables))
	for _, table := range tables {
		fmt.Printf("  - Table: %s (%d columns)\n", table.Name, len(table.Columns))
	}

	// Validate that we have tables to generate code for
	if len(tables) == 0 {
		return fmt.Errorf("❌ No tables found in SQL schema '%s'.\n"+
			"💡 Please ensure your SQL file contains valid CREATE TABLE statements.\n"+
			"📋 Example:\n"+
			"   CREATE TABLE users (\n"+
			"       id BIGSERIAL PRIMARY KEY,\n"+
			"       name VARCHAR(255) NOT NULL,\n"+
			"       email VARCHAR(255) UNIQUE NOT NULL,\n"+
			"       created_at TIMESTAMPTZ DEFAULT NOW()\n"+
			"   );", sqlSchemaFile)
	}

	// Create directory structure
	err = createDirectoryStructure(moduleName)
	if err != nil {
		return fmt.Errorf("failed to create directory structure: %v", err)
	}

	// Generate all template files
	err = generateAllFiles(moduleName, tables)
	if err != nil {
		return fmt.Errorf("failed to generate files: %v", err)
	}

	// Format generated Go files
	err = formatGeneratedFiles(moduleName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not format files: %v\n", err)
		// Don't fail the entire process for formatting issues
	}

	// Initialize Go module and install dependencies
	err = initializeGoModule(moduleName)
	if err != nil {
		return fmt.Errorf("failed to initialize Go module: %v", err)
	}

	return nil
}

// createDirectoryStructure creates the hexagonal architecture folder structure
func createDirectoryStructure(moduleName string) error {
	folders := []string{
		// Root module folder
		moduleName,

		// Service entrypoint
		filepath.Join(moduleName, "cmd", moduleName),

		// Build directory
		filepath.Join(moduleName, "build"),

		// Internal application logic
		filepath.Join(moduleName, "internal", "application"),
		filepath.Join(moduleName, "internal", "application", "dto"),

		// Domain models
		filepath.Join(moduleName, "internal", "domain", "model"),

		// Interactor layer
		filepath.Join(moduleName, "internal", "interactor"),
		filepath.Join(moduleName, "internal", "interactor", "rest"),
		filepath.Join(moduleName, "internal", "interactor", "grpc"),

		// Repository layer
		filepath.Join(moduleName, "internal", "repository"),
		filepath.Join(moduleName, "internal", "repository", "implementor", dialect.Package),
		filepath.Join(moduleName, "internal", "repository", "implementor", "cache"),

		// Package dependencies
		filepath.Join(moduleName, "pkg"),

		// Scripts directory
		filepath.Join(moduleName, "script"),
	}

	// Create all folders
	for _, folder := range folders {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			return fmt.Errorf("failed to create folder %s: %v", folder, err)
		}
		fmt.Printf("Created: %s\n", folder)
	}

	return nil
}

// initializeGoModule initializes Go module and installs dependencies
func initializeGoModule(moduleName string) error {
	fmt.Printf("\n🔧 Initializing Go module for %s...\n", moduleName)

	// Change to the module directory
	moduleDir, err := filepath.Abs(moduleName)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Run go mod tidy to download dependencies
	fmt.Printf("📦 Running 'go mod tidy' to install dependencies...\n")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = moduleDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️  Warning: go mod tidy failed: %v\n", err)
		fmt.Printf("Output: %s\n", string(output))
		fmt.Printf("💡 You can manually run 'go mod tidy' in the %s directory\n", moduleName)
		return nil // Don't fail the entire process, just warn
	}

	fmt.Printf("✅ Dependencies installed successfully!\n")

	// Try to run go build to verify everything compiles
	fmt.Printf("🏗️  Testing build...\n")
	cmd = exec.Command("go", "build", "./cmd/"+moduleName)
	cmd.Dir = moduleDir

	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️  Warning: build test failed: %v\n", err)
		fmt.Printf("Output: %s\n", string(output))
		fmt.Printf("💡 You may need to implement the service interfaces before building\n")
		return nil // Don't fail, just inform user
	}

	fmt.Printf("✅ Build test successful!\n")

	// Run linting
	fmt.Printf("🔍 Running lint checks...\n")
	err = runLintChecks(moduleDir)
	if err != nil {
		fmt.Printf("⚠️  Lint warnings found (non-blocking)\n")
	} else {
		fmt.Printf("✅ Lint checks passed!\n")
	}

	fmt.Printf("\n🎉 Generated project is ready to use!\n")
	fmt.Printf("📁 Navigate to: cd %s\n", moduleName)
	fmt.Printf("🚀 Run service: go run ./cmd/%s\n", moduleName)

	return nil
}

// formatGeneratedFiles formats all generated Go files using gofmt
func formatGeneratedFiles(moduleName string) error {
	fmt.Printf("🎨 Formatting generated files...\n")

	moduleDir := moduleName

	// Try goimports first (includes gofmt + import management)
	cmd := exec.Command("goimports", "-w", ".")
	cmd.Dir = moduleDir
	err := cmd.Run()

	if err != nil {
		// Fallback to gofmt if goimports is not available
		fmt.Printf("  - goimports not available, using gofmt\n")
		cmd = exec.Command("gofmt", "-w", ".")
		cmd.Dir = moduleDir
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to format files: %v", err)
		}
	}

	fmt.Printf("✅ Files formatted successfully!\n")
	return nil
}

// runLintChecks runs various lint checks on the generated code
func runLintChecks(moduleDir string) error {
	var hasErrors bool

	// Run go vet
	fmt.Printf("  - Running go vet...")
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = moduleDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf(" ⚠️\n")
		fmt.Printf("    go vet warnings:\n%s", string(output))
		hasErrors = true
	} else {
		fmt.Printf(" ✅\n")
	}

	// Run goimports check (includes gofmt functionality plus import management)
	fmt.Printf("  - Checking formatting and imports...")
	cmd = exec.Command("goimports", "-l", ".")
	cmd.Dir = moduleDir
	output, err = cmd.CombinedOutput()
	if err == nil && len(output) > 0 {
		fmt.Printf(" ⚠️\n")
		fmt.Printf("    Files needing formatting/import fixes:\n%s", string(output))
		hasErrors = true
	} else if err != nil {
		// goimports not available, fallback to gofmt
		fmt.Printf(" 📦 (goimports unavailable, using gofmt)\n")
		fmt.Printf("  - Checking formatting...")
		cmd = exec.Command("gofmt", "-l", ".")
		cmd.Dir = moduleDir
		output, err = cmd.CombinedOutput()
		if err == nil && len(output) > 0 {
			fmt.Printf(" ⚠️\n")
			fmt.Printf("    Files needing formatting:\n%s", string(output))
			hasErrors = true
		} else {
			fmt.Printf(" ✅\n")
		}
	} else {
		fmt.Printf(" ✅\n")
	}

	// Run golint check (if available)
	fmt.Printf("  - Running golint...")
	cmd = exec.Command("golint", "./...")
	cmd.Dir = moduleDir
	output, err = cmd.CombinedOutput()

	if err != nil {
		// golint not installed, try to install it
		fmt.Printf(" 📦\n")
		fmt.Printf("    Installing golint (golang.org/x/lint/golint@latest)...\n")
		installCmd := exec.Command("go", "install", "golang.org/x/lint/golint@latest")
		if installErr := installCmd.Run(); installErr != nil {
			fmt.Printf("    ⚠️  Could not install golint: %v\n", installErr)
			fmt.Printf("    💡 You can manually install with: go install golang.org/x/lint/golint@latest\n")
			return nil // Don't fail the entire process if golint can't be installed
		}

		fmt.Printf("    ✅ Golint installed successfully!\n")
		fmt.Printf("  - Running golint...")
		// Run golint after successful installation
		cmd = exec.Command("golint", "./...")
		cmd.Dir = moduleDir
		output, err = cmd.CombinedOutput()
	}

	// Process golint results (whether from first attempt or after installation)
	if err == nil {
		if len(output) > 0 {
			fmt.Printf(" ⚠️\n")
			fmt.Printf("    Golint suggestions:\n%s", string(output))
			// Note: golint suggestions are warnings, not blocking errors
			// hasErrors = true  // Uncomment if you want golint to be blocking
		} else {
			fmt.Printf(" ✅\n")
		}
	} else {
		fmt.Printf(" ⚠️\n")
		fmt.Printf("    Golint unavailable: %v\n", err)
		fmt.Printf("    💡 Install manually: go install golang.org/x/lint/golint@latest\n")
	}

	if hasErrors {
		return fmt.Errorf("lint issues found")
	}
	return nil
}
