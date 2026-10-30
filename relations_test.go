package main

import (
	"os"
	"path/filepath"
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
