package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Table represents a database table
type Table struct {
	Name    string
	Columns []Column
}

// Column represents a database column
type Column struct {
	Name         string
	Type         string
	IsPrimaryKey bool
	IsNullable   bool
	DefaultValue string
	GoType       string
	GormTag      string
	JSONTag      string

	// Foreign key target, set when the column REFERENCES another table
	RefTable  string
	RefColumn string
}

// parseSQLSchema parses SQL CREATE TABLE statements and extracts table information
func parseSQLSchema(filename string) ([]Table, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQL file: %v", err)
	}
	defer file.Close()

	var tables []Table
	var currentTable *Table
	var inTableDefinition bool
	var pendingForeignKeys []foreignKey

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "/*") || line == "" {
			continue
		}

		// Detect CREATE TABLE statement
		createTableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(\w+)\s*\(`)
		if matches := createTableRegex.FindStringSubmatch(line); matches != nil {
			tableName := matches[1]
			currentTable = &Table{
				Name:    tableName,
				Columns: []Column{},
			}
			inTableDefinition = true
			continue
		}

		// End of table definition
		if inTableDefinition && strings.Contains(line, ");") {
			if currentTable != nil {
				applyForeignKeys(currentTable, pendingForeignKeys)
				pendingForeignKeys = nil
				tables = append(tables, *currentTable)
				currentTable = nil
			}
			inTableDefinition = false
			continue
		}

		// Parse column definitions
		if inTableDefinition && currentTable != nil {
			if fk, ok := parseTableForeignKey(line); ok {
				pendingForeignKeys = append(pendingForeignKeys, fk)
				continue
			}

			column := parseColumnDefinition(line)
			if column != nil {
				currentTable.Columns = append(currentTable.Columns, *column)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SQL file: %v", err)
	}

	// Generate Go field information for each column
	for i := range tables {
		for j := range tables[i].Columns {
			generateGoFieldInfo(&tables[i].Columns[j])
		}
	}

	return tables, nil
}

// parseColumnDefinition parses a single column definition line
func parseColumnDefinition(line string) *Column {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ",")

	// Skip constraint definitions
	constraintKeywords := []string{"CONSTRAINT", "PRIMARY KEY", "FOREIGN KEY", "UNIQUE", "CHECK", "INDEX"}
	lineUpper := strings.ToUpper(line)
	for _, keyword := range constraintKeywords {
		if strings.Contains(lineUpper, keyword) {
			return nil
		}
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	// Handle multi-word types like "DOUBLE PRECISION"
	columnType := parts[1]
	if len(parts) > 2 && strings.ToUpper(parts[1]) == "DOUBLE" && strings.ToUpper(parts[2]) == "PRECISION" {
		columnType = "DOUBLE PRECISION"
	}

	column := &Column{
		Name:       parts[0],
		Type:       columnType,
		IsNullable: true, // Default to nullable
	}

	// Parse column attributes
	fullLine := strings.ToUpper(line)
	if strings.Contains(fullLine, "PRIMARY KEY") {
		column.IsPrimaryKey = true
	}
	if strings.Contains(fullLine, "NOT NULL") {
		column.IsNullable = false
	}

	parseInlineReference(column, line)

	// Extract default value
	defaultRegex := regexp.MustCompile(`(?i)DEFAULT\s+([^,\s]+)`)
	if matches := defaultRegex.FindStringSubmatch(line); matches != nil {
		column.DefaultValue = matches[1]
	}

	return column
}

// generateGoFieldInfo generates Go field information based on SQL column type
func generateGoFieldInfo(column *Column) {
	// Map SQL types to Go types
	sqlType := strings.ToUpper(column.Type)

	// Extract base type (remove size specifications)
	baseType := regexp.MustCompile(`^([A-Z]+)`).FindString(sqlType)

	switch baseType {
	case "INT", "INTEGER", "SERIAL", "BIGSERIAL", "BIGINT":
		if column.IsPrimaryKey {
			column.GoType = "int64"
		} else {
			column.GoType = "int64"
		}
	case "VARCHAR", "TEXT", "CHAR":
		column.GoType = "string"
	case "JSONB", "JSON":
		column.GoType = "[]string"
	case "BOOLEAN", "BOOL":
		column.GoType = "bool"
	case "TIMESTAMP", "DATETIME":
		column.GoType = "time.Time"
	case "DATE":
		column.GoType = "time.Time"
	case "DECIMAL", "NUMERIC", "FLOAT", "REAL", "DOUBLE", "DOUBLE PRECISION":
		column.GoType = "float64"
	case "UUID":
		column.GoType = "string"
	default:
		column.GoType = "string" // Default to string for unknown types
	}

	// Generate GORM and JSON tags
	columnNameLower := strings.ToLower(column.Name)

	// Build GORM tag
	gormParts := []string{fmt.Sprintf("column:%s", columnNameLower)}
	if column.IsPrimaryKey {
		gormParts = append(gormParts, "primarykey")
	}
	if !column.IsNullable {
		gormParts = append(gormParts, "not null")
	}

	// Add type specification for JSONB fields
	sqlTypeUpper := strings.ToUpper(column.Type)
	if strings.Contains(sqlTypeUpper, "JSONB") {
		gormParts = append(gormParts, "type:jsonb")
	} else if strings.Contains(sqlTypeUpper, "JSON") {
		gormParts = append(gormParts, "type:json")
	}

	column.GormTag = fmt.Sprintf(`gorm:"%s"`, strings.Join(gormParts, ";"))

	// Build JSON tag using original column name to preserve proper snake_case
	column.JSONTag = fmt.Sprintf(`json:"%s,omitempty"`, column.Name)
}

// toCamelCase converts snake_case to CamelCase with proper ID handling
func toCamelCase(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			// Handle common ID patterns
			if parts[i] == "id" {
				parts[i] = "ID"
			} else {
				parts[i] = strings.ToUpper(string(parts[i][0])) + parts[i][1:]
			}
		}
	}
	return strings.Join(parts, "")
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && strings.ToUpper(string(r)) == string(r) {
			result.WriteString("_")
		}
		result.WriteString(strings.ToLower(string(r)))
	}
	return result.String()
}
