package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// generateGooseMigration creates the Goose migration for the SQL schema.
//
// Goose never runs an applied migration again, so a migration that already exists is never
// rewritten. When the schema gained tables since the last run, a new migration that creates
// only those tables is added instead. Changed columns of existing tables cannot be derived
// from the schema alone: they are reported, and need a hand written ALTER migration.
func generateGooseMigration(moduleName string, tables []Table, schema string) error {
	if len(tables) == 0 {
		return nil // Skip if no tables
	}

	existingPath := existingMigrationPath(moduleName)
	if existingPath == "" {
		content, err := buildMigration(tables, schema)
		if err != nil {
			return err
		}
		return writeMigration(moduleName, "create", content)
	}

	applied := tablesInMigrations(moduleName)
	var added []Table
	for _, table := range tables {
		if !applied[strings.ToLower(table.Name)] {
			added = append(added, table)
		}
	}

	warnAboutChangedTables(moduleName, tables, applied, schema)

	if len(added) == 0 {
		return nil
	}

	content, err := buildMigration(added, schema)
	if err != nil {
		return err
	}
	fmt.Printf("New tables since the last run, adding a migration for: %s\n", tableNames(added))
	return writeMigration(moduleName, "add", content)
}

// writeMigration writes a migration with a fresh timestamp
func writeMigration(moduleName, action, content string) error {
	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s_%s_tables.sql", timestamp, action, moduleName)
	migrationPath := filepath.Join(moduleName, "migrations", filename)

	// Write migration file (writeFile creates directories as needed)
	if err := writeFile(migrationPath, content); err != nil {
		return fmt.Errorf("failed to write migration file: %v", err)
	}
	return nil
}

// buildMigration renders the Up and Down sections that create and drop the given tables
func buildMigration(tables []Table, schema string) (string, error) {
	// Generate schema creation
	var schemaCreation strings.Builder
	if schema != "" {
		schemaCreation.WriteString(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS \"%s\";\n\n", schema))
	}

	// Generate table creations
	var tableCreations strings.Builder
	var tableDrops strings.Builder
	var indexCreations strings.Builder
	var indexDrops strings.Builder

	for _, table := range tables {
		// Generate CREATE TABLE statement
		tableCreations.WriteString(generateCreateTable(table, schema))
		tableCreations.WriteString("\n")

		// Generate DROP TABLE statement (reverse order for dependencies)
		tableName := table.Name
		if schema != "" {
			tableName = fmt.Sprintf("%s.%s", schema, table.Name)
		}
		tableDrops.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", tableName))

		// Generate indexes
		indexCreations.WriteString(generateIndexes(table, schema))
		indexDrops.WriteString(generateDropIndexes(table, schema))
	}

	// Reverse the order of table drops for proper dependency handling
	tableDropLines := strings.Split(strings.TrimSpace(tableDrops.String()), "\n")
	var reversedDrops strings.Builder
	for i := len(tableDropLines) - 1; i >= 0; i-- {
		if tableDropLines[i] != "" {
			reversedDrops.WriteString(tableDropLines[i])
			reversedDrops.WriteString("\n")
		}
	}

	// Generate schema drops
	var schemaDrops strings.Builder
	if schema != "" {
		schemaDrops.WriteString(fmt.Sprintf("DROP SCHEMA IF EXISTS \"%s\" CASCADE;\n", schema))
	}

	// Generate migration content
	vars := map[string]string{
		"schema_creation": schemaCreation.String(),
		"table_creations": tableCreations.String(),
		"index_creations": indexCreations.String(),
		"index_drops":     indexDrops.String(),
		"table_drops":     reversedDrops.String(),
		"schema_drops":    schemaDrops.String(),
	}

	result, err := processTemplate("goose-migration", vars)
	if err != nil {
		return "", fmt.Errorf("failed to process goose-migration template: %v", err)
	}
	return result, nil
}

var createTableStatementRegex = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:\w+\.)?(\w+)\s*\(`)

// migrationTexts returns the content of every migration of the service
func migrationTexts(moduleName string) []string {
	files, _ := filepath.Glob(filepath.Join(moduleName, "migrations", "*.sql"))
	var texts []string
	for _, file := range files {
		if data, err := os.ReadFile(file); err == nil {
			texts = append(texts, string(data))
		}
	}
	return texts
}

// tablesInMigrations returns the lower case names of the tables created by existing migrations
func tablesInMigrations(moduleName string) map[string]bool {
	tables := map[string]bool{}
	for _, text := range migrationTexts(moduleName) {
		for _, match := range createTableStatementRegex.FindAllStringSubmatch(text, -1) {
			tables[strings.ToLower(match[1])] = true
		}
	}
	return tables
}

// warnAboutChangedTables reports tables that already have a migration but whose definition in
// the schema no longer matches it. The migration was probably applied, so it is left alone.
func warnAboutChangedTables(moduleName string, tables []Table, applied map[string]bool, schema string) {
	all := strings.Join(migrationTexts(moduleName), "\n")

	var changed []Table
	for _, table := range tables {
		if applied[strings.ToLower(table.Name)] && !strings.Contains(all, generateCreateTable(table, schema)) {
			changed = append(changed, table)
		}
	}
	if len(changed) > 0 {
		fmt.Printf("⚠️  The schema of %s differs from the existing migration. Applied migrations are not rewritten:\n", tableNames(changed))
		fmt.Printf("    write an ALTER TABLE migration by hand for the changed columns.\n")
	}
}

func tableNames(tables []Table) string {
	names := make([]string, len(tables))
	for i, table := range tables {
		names[i] = table.Name
	}
	return strings.Join(names, ", ")
}

// existingMigrationPath returns the migration created by a previous run, or "" if there is none
func existingMigrationPath(moduleName string) string {
	pattern := filepath.Join(moduleName, "migrations", fmt.Sprintf("*_create_%s_tables.sql", moduleName))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// generateCreateTable generates CREATE TABLE SQL for a single table
func generateCreateTable(table Table, schema string) string {
	var sql strings.Builder

	tableName := table.Name
	if schema != "" {
		tableName = fmt.Sprintf("%s.%s", schema, table.Name)
	}

	sql.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", tableName))

	// Always add MetaField columns first if not explicitly present
	hasID := false
	hasCreatedAt := false
	hasUpdatedAt := false
	hasDeletedAt := false
	hasIsDeleted := false

	for _, col := range table.Columns {
		lowerName := strings.ToLower(col.Name)
		switch lowerName {
		case "id":
			hasID = true
		case "created_at":
			hasCreatedAt = true
		case "updated_at":
			hasUpdatedAt = true
		case "deleted_at":
			hasDeletedAt = true
		case "is_deleted":
			hasIsDeleted = true
		}
	}

	// Add missing MetaField columns
	if !hasID {
		sql.WriteString("    id BIGSERIAL PRIMARY KEY,\n")
	}

	// Add table-specific columns
	for _, col := range table.Columns {
		sql.WriteString("    ")
		sql.WriteString(generateColumnDefinition(col, table.Name))
		sql.WriteString(",\n")
	}

	// Add missing MetaField columns at the end
	if !hasCreatedAt {
		sql.WriteString("    created_at TIMESTAMPTZ DEFAULT NOW(),\n")
	}
	if !hasUpdatedAt {
		sql.WriteString("    updated_at TIMESTAMPTZ DEFAULT NOW(),\n")
	}
	if !hasDeletedAt {
		sql.WriteString("    deleted_at TIMESTAMPTZ,\n")
	}
	if !hasIsDeleted {
		sql.WriteString("    is_deleted BOOLEAN DEFAULT FALSE\n")
	} else {
		// Remove the trailing comma from the last column
		sqlStr := sql.String()
		sql.Reset()
		sql.WriteString(strings.TrimSuffix(sqlStr, ",\n") + "\n")
	}

	sql.WriteString(");")
	return sql.String()
} // generateColumnDefinition generates SQL column definition
func generateColumnDefinition(col Column, tableName string) string {
	var def strings.Builder

	def.WriteString(col.Name)
	def.WriteString(" ")

	// Use original SQL type if available, otherwise map from Go type
	var pgType string
	if col.Type != "" {
		// Use the original SQL type from the schema
		pgType = col.Type
	} else {
		// Fallback to mapping from Go type
		pgType = mapGoTypeToPGType(col.GoType, col.Name)
	}
	def.WriteString(pgType)

	// Add constraints
	if col.IsPrimaryKey {
		def.WriteString(" PRIMARY KEY")
	}

	if !col.IsNullable && !col.IsPrimaryKey {
		def.WriteString(" NOT NULL")
	}

	if col.DefaultValue != "" && !col.IsPrimaryKey {
		def.WriteString(" DEFAULT ")
		def.WriteString(col.DefaultValue)
	}

	return def.String()
}

// mapGoTypeToPGType maps Go types back to PostgreSQL types
func mapGoTypeToPGType(goType string, columnName string) string {
	// Handle common column names with specific types
	lowerName := strings.ToLower(columnName)

	// Primary key ID columns
	if lowerName == "id" {
		return "BIGSERIAL"
	}

	// Foreign key ID columns
	if strings.HasSuffix(lowerName, "_id") && (goType == "int64" || goType == "uint" || goType == "int") {
		return "BIGINT"
	}

	// Timestamp columns
	if lowerName == "created_at" || lowerName == "updated_at" || lowerName == "deleted_at" {
		return "TIMESTAMPTZ"
	}

	// Timestamp fields (Unix timestamps)
	if strings.Contains(lowerName, "timestamp") || strings.Contains(lowerName, "_ts") || lowerName == "key_time" {
		return "BIGINT"
	}

	// JSON columns
	if lowerName == "state" && (goType == "string" || goType == "[]byte") {
		return "JSONB"
	}

	// Handle JSONB fields (mapped to []string in Go)
	if goType == "[]int" {
		return "JSONB"
	}

	// Handle basic type mappings
	switch goType {
	case "string":
		// Specific string column mappings
		if strings.Contains(lowerName, "type") || lowerName == "activity_flag" || lowerName == "awake_state" || lowerName == "light_state" || lowerName == "deep_state" || lowerName == "rem_state" {
			return "INTEGER"
		}
		if strings.Contains(lowerName, "name") || strings.Contains(lowerName, "id") && !strings.HasSuffix(lowerName, "_id") {
			return "TEXT"
		}
		return "TEXT"
	case "int", "int32":
		return "INTEGER"
	case "int64":
		return "BIGINT"
	case "float32":
		return "REAL"
	case "float64":
		return "DOUBLE PRECISION"
	case "bool":
		return "BOOLEAN"
	case "time.Time":
		return "TIMESTAMPTZ"
	case "[]byte":
		return "BYTEA"
	default:
		// Try to infer from column name for unknown types
		if strings.Contains(lowerName, "count") || strings.Contains(lowerName, "battery") || strings.Contains(lowerName, "mins") || strings.Contains(lowerName, "hr") || strings.Contains(lowerName, "pp") || strings.Contains(lowerName, "idx") {
			return "INTEGER"
		}
		if strings.Contains(lowerName, "distance") || strings.Contains(lowerName, "calories") || strings.Contains(lowerName, "threshold") || strings.Contains(lowerName, "psv") || strings.Contains(lowerName, "conv_") || strings.Contains(lowerName, "pwm") || strings.Contains(lowerName, "rr") || strings.Contains(lowerName, "psf") || strings.Contains(lowerName, "lf") || strings.Contains(lowerName, "hf") || strings.Contains(lowerName, "rmssd") || strings.Contains(lowerName, "tatpim") || strings.Contains(lowerName, "bmr") || strings.Contains(lowerName, "sdhr") {
			return "DOUBLE PRECISION"
		}
		return "TEXT" // Default fallback
	}
} // generateIndexes generates CREATE INDEX statements for a table
func generateIndexes(table Table, schema string) string {
	var indexes strings.Builder

	tableName := table.Name
	schemaPrefix := ""
	if schema != "" {
		tableName = fmt.Sprintf("%s.%s", schema, table.Name)
		schemaPrefix = schema + "."
	}

	// Generate indexes for common columns
	for _, col := range table.Columns {
		lowerName := strings.ToLower(col.Name)

		// Index foreign keys and common lookup columns
		if strings.Contains(lowerName, "user_id") ||
			strings.Contains(lowerName, "_id") && !col.IsPrimaryKey ||
			lowerName == "timestamp" ||
			lowerName == "key_time" ||
			lowerName == "start_ts" ||
			lowerName == "end_ts" {

			indexName := fmt.Sprintf("idx_%s_%s", table.Name, col.Name)
			indexes.WriteString(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s%s ON %s(%s);\n",
				schemaPrefix, indexName, tableName, col.Name))
		}
	}

	return indexes.String()
}

// generateDropIndexes generates DROP INDEX statements for a table
func generateDropIndexes(table Table, schema string) string {
	var drops strings.Builder

	schemaPrefix := ""
	if schema != "" {
		schemaPrefix = schema + "."
	}

	// Generate drops for the same indexes we created
	for _, col := range table.Columns {
		lowerName := strings.ToLower(col.Name)

		if strings.Contains(lowerName, "user_id") ||
			strings.Contains(lowerName, "_id") && !col.IsPrimaryKey ||
			lowerName == "timestamp" ||
			lowerName == "key_time" ||
			lowerName == "start_ts" ||
			lowerName == "end_ts" {

			indexName := fmt.Sprintf("idx_%s_%s", table.Name, col.Name)
			drops.WriteString(fmt.Sprintf("DROP INDEX IF EXISTS %s%s;\n", schemaPrefix, indexName))
		}
	}

	return drops.String()
}
