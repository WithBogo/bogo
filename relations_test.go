package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSchema(t *testing.T, sql string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.sql")
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseSQLSchema_ForeignKeys(t *testing.T) {
	path := writeSchema(t, `
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    shipping_id BIGINT,
    FOREIGN KEY (shipping_id) REFERENCES addresses (id)
);
`)
	tables, err := parseSQLSchema(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	got := map[string][2]string{}
	for _, col := range tables[0].Columns {
		got[col.Name] = [2]string{col.RefTable, col.RefColumn}
	}
	if got["user_id"] != [2]string{"users", "id"} {
		t.Errorf("inline reference not parsed: %v", got["user_id"])
	}
	if got["shipping_id"] != [2]string{"addresses", "id"} {
		t.Errorf("table level foreign key not parsed: %v", got["shipping_id"])
	}
	if got["id"][0] != "" {
		t.Errorf("id must not reference anything: %v", got["id"])
	}
}

func relationTables() []Table {
	return []Table{
		{Name: "orders", Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "user_id", RefTable: "users", RefColumn: "id"},
		}},
		{Name: "users", Columns: []Column{{Name: "id", IsPrimaryKey: true}}},
	}
}

func TestFindRelations(t *testing.T) {
	relations := findRelations(relationTables())
	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	r := relations[0]
	if r.Parent.Name != "users" || r.Child.Name != "orders" || r.Column.Name != "user_id" {
		t.Errorf("unexpected relation: %+v", r)
	}
	if got := r.nestedMethodName(); got != "GetAllOrdersByUser" {
		t.Errorf("unexpected method name %q", got)
	}
	if got := r.nestedRouteSegment(); got != "orders" {
		t.Errorf("unexpected route segment %q", got)
	}
}

func TestFindRelations_IgnoresUnusableReferences(t *testing.T) {
	tables := []Table{
		{Name: "users", Columns: []Column{{Name: "id", IsPrimaryKey: true}, {Name: "email"}}},
		{Name: "orders", Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "owner_email", RefTable: "users", RefColumn: "email"}, // not the primary key
			{Name: "region_id", RefTable: "regions", RefColumn: "id"},    // table not in the schema
		}},
	}
	if relations := findRelations(tables); len(relations) != 0 {
		t.Errorf("expected no relations, got %+v", relations)
	}
}

func TestFindRelations_AmbiguousForeignKeys(t *testing.T) {
	tables := []Table{
		{Name: "users", Columns: []Column{{Name: "id", IsPrimaryKey: true}}},
		{Name: "messages", Columns: []Column{
			{Name: "id", IsPrimaryKey: true},
			{Name: "sender_id", RefTable: "users"},
			{Name: "receiver_id", RefTable: "users"},
		}},
	}

	relations := findRelations(tables)
	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}
	if relations[0].nestedRouteSegment() == relations[1].nestedRouteSegment() {
		t.Error("ambiguous foreign keys must get distinct routes")
	}
	if relations[0].nestedMethodName() == relations[1].nestedMethodName() {
		t.Error("ambiguous foreign keys must get distinct method names")
	}
}

func TestSortTablesByDependency(t *testing.T) {
	sorted := sortTablesByDependency(relationTables())
	if sorted[0].Name != "users" || sorted[1].Name != "orders" {
		t.Errorf("referenced table must come first, got %s then %s", sorted[0].Name, sorted[1].Name)
	}
}

func TestSortTablesByDependency_Cycle(t *testing.T) {
	tables := []Table{
		{Name: "a", Columns: []Column{{Name: "b_id", RefTable: "b"}}},
		{Name: "b", Columns: []Column{{Name: "a_id", RefTable: "a"}}},
	}
	if sorted := sortTablesByDependency(tables); len(sorted) != 2 {
		t.Errorf("cycles must not drop tables, got %d", len(sorted))
	}
}

func TestParseSQLSchema_QualifiedInlineAndActions(t *testing.T) {
	path := writeSchema(t, `
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE ON UPDATE SET NULL,
    code VARCHAR(10) UNIQUE,
    user_id BIGINT UNIQUE REFERENCES users(id) ON DELETE restrict,
    shipping_id BIGINT,
    CONSTRAINT fk_ship FOREIGN KEY (shipping_id) REFERENCES "public"."addresses" (id) ON DELETE SET NULL
);
`)
	tables, err := parseSQLSchema(path)
	if err != nil {
		t.Fatal(err)
	}
	cols := map[string]Column{}
	for _, c := range tables[0].Columns {
		cols[c.Name] = c
	}
	o := cols["owner_id"]
	if o.RefSchema != "auth" || o.RefTable != "users" || o.RefColumn != "id" || o.RefOnDelete != "CASCADE" || o.RefOnUpdate != "SET NULL" {
		t.Errorf("qualified inline reference wrong: %+v", o)
	}
	u, ok := cols["user_id"]
	if !ok || u.RefTable != "users" || u.RefOnDelete != "RESTRICT" {
		t.Errorf("a UNIQUE column with REFERENCES must keep its foreign key: %+v", u)
	}
	s := cols["shipping_id"]
	if s.RefSchema != "public" || s.RefTable != "addresses" || s.RefOnDelete != "SET NULL" {
		t.Errorf("qualified table level foreign key wrong: %+v", s)
	}
	if _, ok := cols["fk_ship"]; ok {
		t.Error("table level constraint must not become a column")
	}
}

func TestFindRelations_IgnoresSchemaQualifiedTarget(t *testing.T) {
	tables := []Table{
		{Name: "users", Columns: []Column{{Name: "id", IsPrimaryKey: true}}},
		{Name: "orders", Columns: []Column{{Name: "user_id", RefSchema: "auth", RefTable: "users", RefColumn: "id"}}},
	}
	if r := findRelations(tables); len(r) != 0 {
		t.Errorf("a reference into another schema is not a nested endpoint: %+v", r)
	}
}

func TestReferenceClause(t *testing.T) {
	col := Column{RefTable: "users", RefColumn: "id", RefOnDelete: "CASCADE", RefOnUpdate: "NO ACTION"}
	if got := referenceClause(col, "app"); got != "REFERENCES app.users(id) ON DELETE CASCADE ON UPDATE NO ACTION" {
		t.Errorf("got %q", got)
	}
	col = Column{RefSchema: "auth", RefTable: "users", RefColumn: "id"}
	if got := referenceClause(col, "app"); got != "REFERENCES auth.users(id)" {
		t.Errorf("got %q", got)
	}
}

func TestDeferredForeignKeys_Cycle(t *testing.T) {
	tables := sortTablesByDependency([]Table{
		{Name: "a", Columns: []Column{{Name: "b_id", RefTable: "b"}}},
		{Name: "b", Columns: []Column{{Name: "a_id", RefTable: "a"}}},
		{Name: "c", Columns: []Column{{Name: "c_id", RefTable: "c"}}}, // self reference stays inline
	})
	deferred := deferredForeignKeys(tables)
	if len(deferred) != 1 {
		t.Fatalf("exactly one constraint of the cycle must be deferred, got %v", deferred)
	}
	if len(deferred["b"]) != 1 || deferred["b"][0].Name != "a_id" {
		t.Errorf("unexpected deferred constraints: %v", deferred)
	}
}

func TestGenerateCreateTable_DeferredReference(t *testing.T) {
	table := Table{Name: "a", Columns: []Column{{Name: "b_id", Type: "BIGINT", RefTable: "b", RefColumn: "id"}}}
	if sql := generateCreateTable(table, "", nil); !strings.Contains(sql, "REFERENCES b(id)") {
		t.Errorf("reference should be inline: %s", sql)
	}
	if sql := generateCreateTable(table, "", table.Columns); strings.Contains(sql, "REFERENCES") {
		t.Errorf("deferred reference must not be inline: %s", sql)
	}
}

func TestRestHandler_NestedDropsFKFromQueryValidation(t *testing.T) {
	tables := relationTables()
	handler := generateRestHandler("svc", tables[0], findRelations(tables))
	if !strings.Contains(handler, `info.DBKey != "user_id"`) || !strings.Contains(handler, "ReadQuery(r, pathFilterQuery)") {
		t.Error("the nested handler must not validate the path-derived foreign key from the query string")
	}
}

func TestMigrationHasTable_AcceptsDeferredForeignKeys(t *testing.T) {
	tables := []Table{
		{Name: "a", Columns: []Column{{Name: "b_id", Type: "BIGINT", RefTable: "b", RefColumn: "id"}}},
		{Name: "b", Columns: []Column{{Name: "a_id", Type: "BIGINT", RefTable: "a", RefColumn: "id"}}},
	}
	sorted := sortTablesByDependency(tables)
	deferred := deferredForeignKeys(sorted)
	var text string
	for _, table := range sorted {
		text += generateCreateTable(table, "", deferred[table.Name]) + "\n"
	}
	applied := map[string]bool{"a": true, "b": true}
	for _, table := range tables {
		if !migrationHasTable(text, table, tables, applied, "") {
			t.Errorf("table %s of an unchanged cyclic schema must not be reported as changed", table.Name)
		}
	}
}
