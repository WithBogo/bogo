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
	positional, selected, err := parseArgs([]string{"svc", "--db=mysql", "schema.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if len(positional) != 2 || positional[0] != "svc" || positional[1] != "schema.sql" {
		t.Errorf("unexpected positional arguments %v", positional)
	}
	if selected.Name != "mysql" {
		t.Errorf("expected mysql, got %s", selected.Name)
	}

	if _, selected, _ := parseArgs([]string{"svc", "schema.sql"}); selected.Name != "postgres" {
		t.Errorf("postgres must stay the default, got %s", selected.Name)
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

func TestIndexStatements(t *testing.T) {
	if got := dialects["mysql"].createIndex("", "idx_a", "a", "x"); strings.Contains(got, "IF NOT EXISTS") {
		t.Errorf("MySQL has no IF NOT EXISTS for indexes: %s", got)
	}
	if got := dialects["mysql"].dropIndex("", "idx_a", "a"); got != "DROP INDEX idx_a ON a;\n" {
		t.Errorf("MySQL drop must name the table: %q", got)
	}
	if got := dialects["postgres"].dropIndex("s.", "idx_a", "s.a"); got != "DROP INDEX IF EXISTS s.idx_a;\n" {
		t.Errorf("unexpected postgres drop: %q", got)
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
		sql := generateCreateTable(table, "")
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
