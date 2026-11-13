package main

import (
	"fmt"
	"sort"
	"strings"
)

// Dialect describes everything that differs between the supported databases
type Dialect struct {
	Name    string // value of the --db flag
	Package string // Go package and folder under internal/repository/implementor
	Display string // name used in comments and docs

	// TemplateSuffix selects dialect specific variants of the db-connection and
	// docker-compose templates; the PostgreSQL ones have no suffix
	TemplateSuffix string
	// MigrationTemplate wraps the migration. PostgreSQL runs it as one statement block, the
	// others rely on goose splitting statements because MySQL drivers refuse multi statement execs.
	MigrationTemplate string

	DriverRequire   string // go.mod line for the GORM driver
	UserDefault     string
	PasswordDefault string
	PortDefault     string
	ShellCommand    string // how to open a database shell in the compose container; <module_name> is replaced

	FileBased bool // SQLite keeps everything in one file, no server, no credentials

	// meta column definitions appended to every table
	idColumn, createdAtColumn, updatedAtColumn, deletedAtColumn, isDeletedColumn string
}

var dialects = map[string]Dialect{
	"postgres": {
		Name:              "postgres",
		Package:           "postgres",
		Display:           "PostgreSQL",
		MigrationTemplate: "goose-migration",
		DriverRequire:     "gorm.io/driver/postgres v1.5.4",
		UserDefault:       "postgres",
		PasswordDefault:   "postgres",
		PortDefault:       "5432",
		ShellCommand:      "docker-compose exec db psql -U postgres -d <module_name>",
		idColumn:          "id BIGSERIAL PRIMARY KEY",
		createdAtColumn:   "created_at TIMESTAMPTZ DEFAULT NOW()",
		updatedAtColumn:   "updated_at TIMESTAMPTZ DEFAULT NOW()",
		deletedAtColumn:   "deleted_at TIMESTAMPTZ",
		isDeletedColumn:   "is_deleted BOOLEAN DEFAULT FALSE",
	},
	"mysql": {
		Name:              "mysql",
		Package:           "mysql",
		Display:           "MySQL",
		TemplateSuffix:    "-mysql",
		MigrationTemplate: "goose-migration-plain",
		DriverRequire:     "gorm.io/driver/mysql v1.5.2",
		UserDefault:       "root",
		PasswordDefault:   "mysql",
		PortDefault:       "3306",
		ShellCommand:      "docker-compose exec db mysql -uroot -pmysql <module_name>",
		idColumn:          "id BIGINT AUTO_INCREMENT PRIMARY KEY",
		createdAtColumn:   "created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6)",
		updatedAtColumn:   "updated_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6)",
		deletedAtColumn:   "deleted_at DATETIME(6) NULL",
		isDeletedColumn:   "is_deleted BOOLEAN DEFAULT FALSE",
	},
	"sqlite": {
		Name:              "sqlite",
		Package:           "sqlite",
		Display:           "SQLite",
		TemplateSuffix:    "-sqlite",
		MigrationTemplate: "goose-migration-plain",
		DriverRequire:     "github.com/glebarez/sqlite v1.11.0",
		PortDefault:       "0",
		ShellCommand:      "sqlite3 data/<module_name>.db",
		FileBased:         true,
		idColumn:          "id INTEGER PRIMARY KEY AUTOINCREMENT",
		createdAtColumn:   "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP",
		updatedAtColumn:   "updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP",
		deletedAtColumn:   "deleted_at TIMESTAMP",
		isDeletedColumn:   "is_deleted BOOLEAN DEFAULT FALSE",
	},
}

// dialect is the database the current run generates code for
var dialect = dialects["postgres"]

// lookupDialect resolves the value of --db
func lookupDialect(name string) (Dialect, error) {
	if d, ok := dialects[strings.ToLower(name)]; ok {
		return d, nil
	}
	return Dialect{}, fmt.Errorf("unknown database %q, supported: %s", name, strings.Join(dialectNames(), ", "))
}

func dialectNames() []string {
	names := make([]string, 0, len(dialects))
	for name := range dialects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// templateName returns the dialect specific variant of a template
func (d Dialect) templateName(base string) string {
	return base + d.TemplateSuffix
}

// dbNameDefault is the default value of DB_NAME: a database name, or a file for SQLite
func (d Dialect) dbNameDefault(moduleName string) string {
	if d.FileBased {
		return "data/" + moduleName + ".db"
	}
	return moduleName
}

// sqlType translates the type written in the schema file to the dialect.
// PostgreSQL is the reference, so its types pass through untouched.
func (d Dialect) sqlType(sqlType string) string {
	if d.Name == "postgres" {
		return sqlType
	}

	base := strings.ToUpper(sqlType)
	if i := strings.Index(base, "("); i >= 0 {
		base = base[:i]
	}

	switch d.Name {
	case "mysql":
		switch base {
		case "BIGSERIAL":
			return "BIGINT AUTO_INCREMENT"
		case "SERIAL":
			return "INT AUTO_INCREMENT"
		case "TIMESTAMPTZ", "TIMESTAMP", "DATETIME":
			return "DATETIME(6)"
		case "JSONB", "JSON":
			return "JSON"
		case "UUID":
			return "CHAR(36)"
		case "BYTEA":
			return "BLOB"
		}
	case "sqlite":
		switch base {
		case "BIGSERIAL", "SERIAL":
			return "INTEGER"
		case "TIMESTAMPTZ", "DATETIME":
			return "TIMESTAMP"
		case "JSONB", "JSON", "UUID":
			return "TEXT"
		case "BYTEA":
			return "BLOB"
		}
	}
	return sqlType
}

// sqlDefault translates a DEFAULT expression. PostgreSQL casts such as 'active'::text are
// invalid on the other databases, so the cast is dropped and the value is kept.
func (d Dialect) sqlDefault(expr string) string {
	if d.Name != "postgres" {
		if i := strings.Index(expr, "::"); i >= 0 {
			expr = expr[:i]
		}
	}

	if strings.EqualFold(expr, "NOW()") {
		switch d.Name {
		case "mysql":
			return "CURRENT_TIMESTAMP(6)"
		case "sqlite":
			return "CURRENT_TIMESTAMP"
		}
	}
	return expr
}

// mysqlPrefixLength is the key prefix used to index TEXT and BLOB columns on MySQL, which
// cannot index them in full. 191 characters fit the 767 byte key limit of older row formats.
const mysqlPrefixLength = 191

// indexExpr returns what to put between the parentheses of CREATE INDEX for a column,
// or "" when the column cannot be indexed on this dialect
func (d Dialect) indexExpr(col Column) string {
	if d.Name != "mysql" {
		return col.Name
	}

	switch baseSQLType(d.sqlType(col.Type)) {
	case "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB":
		return fmt.Sprintf("%s(%d)", col.Name, mysqlPrefixLength)
	case "JSON":
		return ""
	}
	return col.Name
}

// baseSQLType returns the upper case type name without size or modifiers
func baseSQLType(sqlType string) string {
	base := strings.ToUpper(strings.TrimSpace(sqlType))
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	return base
}

// createIndex renders a CREATE INDEX statement, or "" for a column that cannot be indexed.
// MySQL has no IF NOT EXISTS for indexes.
func (d Dialect) createIndex(schemaPrefix, indexName, tableName string, col Column) string {
	expr := d.indexExpr(col)
	if expr == "" {
		return ""
	}
	if d.Name == "mysql" {
		return fmt.Sprintf("CREATE INDEX %s ON %s(%s);\n", indexName, tableName, expr)
	}
	return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s%s ON %s(%s);\n", schemaPrefix, indexName, tableName, expr)
}

// dropIndex renders a DROP INDEX statement, or "" for a column createIndex skipped.
// MySQL needs the table and has no IF EXISTS.
func (d Dialect) dropIndex(schemaPrefix, indexName, tableName string, col Column) string {
	if d.indexExpr(col) == "" {
		return ""
	}
	if d.Name == "mysql" {
		return fmt.Sprintf("DROP INDEX %s ON %s;\n", indexName, tableName)
	}
	return fmt.Sprintf("DROP INDEX IF EXISTS %s%s;\n", schemaPrefix, indexName)
}

// templateVariables are shared by the templates that mention the database
func (d Dialect) templateVariables(moduleName string) map[string]string {
	return map[string]string{
		"db_package":          d.Package,
		"db_display":          d.Display,
		"db_driver_require":   d.DriverRequire,
		"db_user_default":     d.UserDefault,
		"db_password_default": d.PasswordDefault,
		"db_port_default":     d.PortDefault,
		"db_name_default":     d.dbNameDefault(moduleName),
		"db_shell_command":    strings.ReplaceAll(d.ShellCommand, "<module_name>", moduleName),
		"db_init_line":        d.initLine(),
		"db_migrate_note":     d.migrateNote(moduleName),
	}
}

// initLine is the README bullet describing how the compose database gets its schema
func (d Dialect) initLine() string {
	if d.Name == "postgres" {
		return "- 📋 Initialized with your SQL schema via migration files  "
	}
	return "- 📋 Created empty, apply the migrations with goose (see below)  "
}

// migrateNote tells how to apply the migrations for dialects whose container does not do it
func (d Dialect) migrateNote(moduleName string) string {
	const fence = "```"
	intro := "\n\nApply the migrations with [goose](https://github.com/pressly/goose):\n" + fence + "bash\n"

	switch d.Name {
	case "mysql":
		return intro + "goose -dir migrations mysql \"root:mysql@tcp(localhost:3306)/" + moduleName + "\" up\n" + fence
	case "sqlite":
		return intro + "mkdir -p data\ngoose -dir migrations sqlite3 data/" + moduleName + ".db up\n" + fence
	}
	return ""
}
