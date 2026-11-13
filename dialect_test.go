package main

import (
	"strings"
	"testing"
)

func useDialect(t *testing.T, name string) {
	t.Helper()
	previous := dialect
	dialect = dialects[name]
	t.Cleanup(func() { dialect = previous })
}

func TestParseArgs(t *testing.T) {
	positional, opts, err := parseArgs([]string{"svc", "--db=mysql", "schema.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if len(positional) != 2 || positional[0] != "svc" || positional[1] != "schema.sql" {
		t.Errorf("unexpected positional arguments %v", positional)
	}
	if opts.Dialect.Name != "mysql" {
		t.Errorf("expected mysql, got %s", opts.Dialect.Name)
	}
	if opts.DryRun || opts.Force {
		t.Error("--dry-run and --force must be off unless given")
	}

	if _, opts, _ := parseArgs([]string{"svc", "schema.sql"}); opts.Dialect.Name != "postgres" {
		t.Errorf("postgres must stay the default, got %s", opts.Dialect.Name)
	}

	_, opts, err = parseArgs([]string{"--dry-run", "svc", "--db=sqlite", "--force", "schema.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.DryRun || !opts.Force || opts.Dialect.Name != "sqlite" {
		t.Errorf("flags must combine in any order: %+v", opts)
	}
	if _, _, err := parseArgs([]string{"--db=oracle"}); err == nil || !strings.Contains(err.Error(), "supported") {
		t.Errorf("unknown databases must list the supported ones, got %v", err)
	}
	if _, _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Error("unknown flags must be rejected")
	}
}

func TestSQLType(t *testing.T) {
	cases := []struct{ dialect, in, want string }{
		{"postgres", "TIMESTAMPTZ", "TIMESTAMPTZ"},
		{"postgres", "JSONB", "JSONB"},
		{"mysql", "BIGSERIAL", "BIGINT AUTO_INCREMENT"},
		{"mysql", "TIMESTAMPTZ", "DATETIME(6)"},
		{"mysql", "JSONB", "JSON"},
		{"mysql", "UUID", "CHAR(36)"},
		{"mysql", "VARCHAR(255)", "VARCHAR(255)"},
		{"sqlite", "BIGSERIAL", "INTEGER"},
		{"sqlite", "TIMESTAMPTZ", "TIMESTAMP"},
		{"sqlite", "JSONB", "TEXT"},
		{"sqlite", "VARCHAR(255)", "VARCHAR(255)"},
	}
	for _, c := range cases {
		if got := dialects[c.dialect].sqlType(c.in); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.dialect, c.in, got, c.want)
		}
	}
}

func TestSQLDefault(t *testing.T) {
	if got := dialects["mysql"].sqlDefault("NOW()"); got != "CURRENT_TIMESTAMP(6)" {
		t.Errorf("mysql NOW(): %s", got)
	}
	if got := dialects["sqlite"].sqlDefault("now()"); got != "CURRENT_TIMESTAMP" {
		t.Errorf("sqlite now(): %s", got)
	}
	if got := dialects["postgres"].sqlDefault("NOW()"); got != "NOW()" {
		t.Errorf("postgres must keep NOW(): %s", got)
	}
	if got := dialects["mysql"].sqlDefault("'active'"); got != "'active'" {
		t.Errorf("other defaults must pass through: %s", got)
	}
}

func TestSQLDefault_DropsPostgresCasts(t *testing.T) {
	cases := []struct{ dialect, in, want string }{
		{"mysql", "'active'::text", "'active'"},
		{"mysql", "'active'::character", "'active'"},
		{"sqlite", "0::bigint", "0"},
		{"mysql", "NOW()::timestamp", "CURRENT_TIMESTAMP(6)"},
		{"postgres", "'active'::text", "'active'::text"},
	}
	for _, c := range cases {
		if got := dialects[c.dialect].sqlDefault(c.in); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.dialect, c.in, got, c.want)
		}
	}
}

func TestIndexStatements(t *testing.T) {
	plain := Column{Name: "x", Type: "BIGINT"}

	if got := dialects["mysql"].createIndex("", "idx_a", "a", plain); strings.Contains(got, "IF NOT EXISTS") {
		t.Errorf("MySQL has no IF NOT EXISTS for indexes: %s", got)
	}
	if got := dialects["mysql"].dropIndex("", "idx_a", "a", plain); got != "DROP INDEX idx_a ON a;\n" {
		t.Errorf("MySQL drop must name the table: %q", got)
	}
	if got := dialects["postgres"].dropIndex("s.", "idx_a", "s.a", plain); got != "DROP INDEX IF EXISTS s.idx_a;\n" {
		t.Errorf("unexpected postgres drop: %q", got)
	}
}

func TestIndexStatements_MySQLTextColumns(t *testing.T) {
	text := Column{Name: "session_id", Type: "TEXT"}
	if got := dialects["mysql"].createIndex("", "idx_a_session_id", "a", text); !strings.Contains(got, "session_id(191)") {
		t.Errorf("MySQL cannot index TEXT without a prefix length: %q", got)
	}
	if got := dialects["postgres"].createIndex("", "idx_a_session_id", "a", text); strings.Contains(got, "(191)") {
		t.Errorf("only MySQL needs a prefix length: %q", got)
	}

	json := Column{Name: "payload_id", Type: "JSONB"}
	if got := dialects["mysql"].createIndex("", "i", "a", json); got != "" {
		t.Errorf("JSON columns cannot be indexed on MySQL: %q", got)
	}
	if got := dialects["mysql"].dropIndex("", "i", "a", json); got != "" {
		t.Errorf("the drop must be skipped together with the create: %q", got)
	}
}

func TestSmallIntegerColumnsAreIntegers(t *testing.T) {
	for _, sqlType := range []string{"SMALLINT", "TINYINT", "MEDIUMINT", "SMALLSERIAL", "INT4", "BIGINT"} {
		col := Column{Name: "version", Type: sqlType, IsNullable: true}
		generateGoFieldInfo(&col)
		if col.GoType != "int64" {
			t.Errorf("%s must map to int64, got %s", sqlType, col.GoType)
		}
	}

	table := Table{Name: "docs", Columns: []Column{{Name: "version", Type: "SMALLINT", GoType: "int64"}}}
	if _, ok := versionColumn(table); !ok {
		t.Error("a SMALLINT version column must enable optimistic locking")
	}
}

func TestCreateTablePerDialect(t *testing.T) {
	table := Table{Name: "notes", Columns: []Column{
		{Name: "body", Type: "TEXT", IsNullable: true},
		{Name: "seen_at", Type: "TIMESTAMPTZ", DefaultValue: "NOW()", IsNullable: true},
	}}

	want := map[string][]string{
		"postgres": {"id BIGSERIAL PRIMARY KEY", "seen_at TIMESTAMPTZ DEFAULT NOW()", "created_at TIMESTAMPTZ DEFAULT NOW()"},
		"mysql":    {"id BIGINT AUTO_INCREMENT PRIMARY KEY", "seen_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6)", "deleted_at DATETIME(6) NULL"},
		"sqlite":   {"id INTEGER PRIMARY KEY AUTOINCREMENT", "seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP", "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"},
	}
	for name, fragments := range want {
		useDialect(t, name)
		sql := generateCreateTable(table, "", nil)
		for _, fragment := range fragments {
			if !strings.Contains(sql, fragment) {
				t.Errorf("%s: migration is missing %q\n%s", name, fragment, sql)
			}
		}
	}
}

func TestRepositoryUsesDialectPackage(t *testing.T) {
	table := Table{Name: "notes", Columns: []Column{{Name: "body", GoType: "string"}}}

	useDialect(t, "mysql")
	repo := generatePostgresRepository("svc", table)
	if !strings.HasPrefix(repo, "package mysql") {
		t.Errorf("repository must live in the dialect package:\n%.60s", repo)
	}
	if !strings.Contains(repo, "MySQL repository") {
		t.Error("repository comment must name the dialect")
	}
}
