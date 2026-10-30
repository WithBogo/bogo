package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	inlineReferenceRegex = regexp.MustCompile(`(?i)\bREFERENCES\s+(\w+)\s*(?:\(\s*(\w+)\s*\))?`)
	tableForeignKeyRegex = regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\s*\(\s*(\w+)\s*\)\s*REFERENCES\s+(\w+)\s*(?:\(\s*(\w+)\s*\))?`)
)

// foreignKey is a table level FOREIGN KEY clause waiting to be attached to its column
type foreignKey struct {
	Column    string
	RefTable  string
	RefColumn string
}

// parseInlineReference reads "REFERENCES parent(col)" on a column definition line
func parseInlineReference(column *Column, line string) {
	if m := inlineReferenceRegex.FindStringSubmatch(line); m != nil {
		column.RefTable = m[1]
		column.RefColumn = m[2]
	}
}

// parseTableForeignKey reads "[CONSTRAINT name] FOREIGN KEY (col) REFERENCES parent(col)"
func parseTableForeignKey(line string) (foreignKey, bool) {
	m := tableForeignKeyRegex.FindStringSubmatch(line)
	if m == nil {
		return foreignKey{}, false
	}
	return foreignKey{Column: m[1], RefTable: m[2], RefColumn: m[3]}, true
}

// applyForeignKeys attaches table level foreign keys to the columns they constrain
func applyForeignKeys(table *Table, keys []foreignKey) {
	for _, key := range keys {
		for i := range table.Columns {
			if strings.EqualFold(table.Columns[i].Name, key.Column) {
				table.Columns[i].RefTable = key.RefTable
				table.Columns[i].RefColumn = key.RefColumn
			}
		}
	}
}

// Relation is a one-to-many link: Child.Column references the primary key of Parent
type Relation struct {
	Parent Table
	Child  Table
	Column Column // the foreign key column on Child
	// Ambiguous is set when Child has several foreign keys to Parent, which then need
	// distinct routes and method names
	Ambiguous bool
}

// primaryKeyName returns the name of the primary key column of a table
func primaryKeyName(table Table) string {
	for _, col := range table.Columns {
		if col.IsPrimaryKey {
			return col.Name
		}
	}
	return "id"
}

// findRelations resolves foreign keys between the tables of the schema. A foreign key is
// only exposed as a nested endpoint when it points at the primary key of a table that is
// part of the schema, because the nested route identifies the parent by that key.
func findRelations(tables []Table) []Relation {
	byName := make(map[string]Table, len(tables))
	for _, table := range tables {
		byName[strings.ToLower(table.Name)] = table
	}

	var relations []Relation
	for _, child := range tables {
		perParent := map[string]int{}
		var found []Relation

		for _, col := range child.Columns {
			if col.RefTable == "" {
				continue
			}
			parent, ok := byName[strings.ToLower(col.RefTable)]
			if !ok {
				continue
			}
			if col.RefColumn != "" && !strings.EqualFold(col.RefColumn, primaryKeyName(parent)) {
				continue
			}
			found = append(found, Relation{Parent: parent, Child: child, Column: col})
			perParent[strings.ToLower(parent.Name)]++
		}

		for _, relation := range found {
			relation.Ambiguous = perParent[strings.ToLower(relation.Parent.Name)] > 1
			relations = append(relations, relation)
		}
	}
	return relations
}

// childRelations returns the relations where the given table is the child
func childRelations(relations []Relation, table Table) []Relation {
	var result []Relation
	for _, relation := range relations {
		if strings.EqualFold(relation.Child.Name, table.Name) {
			result = append(result, relation)
		}
	}
	return result
}

// parentRelations returns the relations where the given table is the parent
func parentRelations(relations []Relation, table Table) []Relation {
	var result []Relation
	for _, relation := range relations {
		if strings.EqualFold(relation.Parent.Name, table.Name) {
			result = append(result, relation)
		}
	}
	return result
}

// nestedMethodName is the handler method that lists children of one parent,
// for example GetAllOrdersByUser
func (r Relation) nestedMethodName() string {
	name := fmt.Sprintf("GetAll%sBy%s", relationStructName(r.Child)+"s", relationStructName(r.Parent))
	if r.Ambiguous {
		name += "Via" + toCamelCase(r.Column.Name)
	}
	return name
}

// nestedRouteSegment is the path segment after /<parents>/:id/, for example orders
func (r Relation) nestedRouteSegment() string {
	segment := strings.ToLower(relationStructName(r.Child)) + "s"
	if r.Ambiguous {
		segment += "_by_" + strings.ToLower(r.Column.Name)
	}
	return segment
}

// sortTablesByDependency orders tables so that referenced tables come before the tables
// that reference them. The order of unrelated tables is kept, and cycles fall back to
// schema order instead of failing.
func sortTablesByDependency(tables []Table) []Table {
	index := make(map[string]int, len(tables))
	for i, table := range tables {
		index[strings.ToLower(table.Name)] = i
	}

	const (
		unvisited = iota
		visiting
		done
	)
	state := make([]int, len(tables))
	ordered := make([]Table, 0, len(tables))

	var visit func(i int)
	visit = func(i int) {
		if state[i] != unvisited {
			return
		}
		state[i] = visiting

		var deps []int
		for _, col := range tables[i].Columns {
			if j, ok := index[strings.ToLower(col.RefTable)]; ok && j != i && col.RefTable != "" {
				deps = append(deps, j)
			}
		}
		sort.Ints(deps)
		for _, j := range deps {
			visit(j)
		}

		state[i] = done
		ordered = append(ordered, tables[i])
	}

	for i := range tables {
		visit(i)
	}
	return ordered
}

// relationStructName returns the Go struct name used for a table (singular, CamelCase)
func relationStructName(table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}
	return structName
}
