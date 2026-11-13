package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	// A reference target is "table" or "schema.table", each part optionally "quoted"
	inlineReferenceRegex = regexp.MustCompile(`(?i)\bREFERENCES\s+(?:"?(\w+)"?\s*\.\s*)?"?(\w+)"?\s*(?:\(\s*"?(\w+)"?\s*\))?`)
	tableForeignKeyRegex = regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\s*\(\s*"?(\w+)"?\s*\)\s*REFERENCES\s+(?:"?(\w+)"?\s*\.\s*)?"?(\w+)"?\s*(?:\(\s*"?(\w+)"?\s*\))?`)
	onDeleteRegex        = regexp.MustCompile(`(?i)\bON\s+DELETE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION)`)
	onUpdateRegex        = regexp.MustCompile(`(?i)\bON\s+UPDATE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION)`)
)

// foreignKey is a table level FOREIGN KEY clause waiting to be attached to its column
type foreignKey struct {
	Column    string
	RefSchema string
	RefTable  string
	RefColumn string
	OnDelete  string
	OnUpdate  string
}

// normalizeAction upper-cases a referential action and collapses its whitespace
func normalizeAction(action string) string {
	return strings.ToUpper(strings.Join(strings.Fields(action), " "))
}

// referentialActions reads ON DELETE / ON UPDATE from a column or constraint line
func referentialActions(line string) (onDelete, onUpdate string) {
	if m := onDeleteRegex.FindStringSubmatch(line); m != nil {
		onDelete = normalizeAction(m[1])
	}
	if m := onUpdateRegex.FindStringSubmatch(line); m != nil {
		onUpdate = normalizeAction(m[1])
	}
	return onDelete, onUpdate
}

// hasInlineReference reports whether a column definition line carries a REFERENCES clause
func hasInlineReference(line string) bool {
	return inlineReferenceRegex.MatchString(line)
}

// parseInlineReference reads "REFERENCES parent(col)" on a column definition line
func parseInlineReference(column *Column, line string) {
	if m := inlineReferenceRegex.FindStringSubmatch(line); m != nil {
		column.RefSchema = m[1]
		column.RefTable = m[2]
		column.RefColumn = m[3]
		column.RefOnDelete, column.RefOnUpdate = referentialActions(line)
	}
}

// parseTableForeignKey reads "[CONSTRAINT name] FOREIGN KEY (col) REFERENCES parent(col)"
func parseTableForeignKey(line string) (foreignKey, bool) {
	m := tableForeignKeyRegex.FindStringSubmatch(line)
	if m == nil {
		return foreignKey{}, false
	}
	onDelete, onUpdate := referentialActions(line)
	return foreignKey{Column: m[1], RefSchema: m[2], RefTable: m[3], RefColumn: m[4], OnDelete: onDelete, OnUpdate: onUpdate}, true
}

// applyForeignKeys attaches table level foreign keys to the columns they constrain
func applyForeignKeys(table *Table, keys []foreignKey) {
	for _, key := range keys {
		for i := range table.Columns {
			if strings.EqualFold(table.Columns[i].Name, key.Column) {
				table.Columns[i].RefSchema = key.RefSchema
				table.Columns[i].RefTable = key.RefTable
				table.Columns[i].RefColumn = key.RefColumn
				table.Columns[i].RefOnDelete = key.OnDelete
				table.Columns[i].RefOnUpdate = key.OnUpdate
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
			if col.RefTable == "" || col.RefSchema != "" {
				// A schema-qualified target is not a table of this schema, so it is
				// only a constraint, not a nested endpoint
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
			if j, ok := index[strings.ToLower(col.RefTable)]; ok && j != i && col.RefTable != "" && col.RefSchema == "" {
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

// referenceClause renders "REFERENCES [schema.]table(col) [ON DELETE ..] [ON UPDATE ..]".
// projectSchema qualifies targets that were written without a schema.
func referenceClause(col Column, projectSchema string) string {
	refSchema := col.RefSchema
	if refSchema == "" {
		refSchema = projectSchema
	}
	refTable := col.RefTable
	if refSchema != "" {
		refTable = fmt.Sprintf("%s.%s", refSchema, refTable)
	}
	clause := "REFERENCES " + refTable
	if col.RefColumn != "" {
		clause += fmt.Sprintf("(%s)", col.RefColumn)
	}
	if col.RefOnDelete != "" {
		clause += " ON DELETE " + col.RefOnDelete
	}
	if col.RefOnUpdate != "" {
		clause += " ON UPDATE " + col.RefOnUpdate
	}
	return clause
}

// deferredForeignKeys returns, per table name, the foreign key columns that cannot be
// declared inline because their target table is created later in the migration. That
// only happens with cycles; those constraints are added with ALTER TABLE once all tables
// exist. Self references are fine inline. tables must already be in creation order.
func deferredForeignKeys(tables []Table) map[string][]Column {
	position := make(map[string]int, len(tables))
	for i, table := range tables {
		position[strings.ToLower(table.Name)] = i
	}
	deferred := map[string][]Column{}
	for i, table := range tables {
		for _, col := range table.Columns {
			if col.RefTable == "" || col.RefSchema != "" {
				continue
			}
			j, ok := position[strings.ToLower(col.RefTable)]
			if ok && j > i {
				deferred[table.Name] = append(deferred[table.Name], col)
			}
		}
	}
	return deferred
}

// relationStructName returns the Go struct name used for a table (singular, CamelCase)
func relationStructName(table Table) string {
	structName := toCamelCase(table.Name)
	if strings.HasSuffix(structName, "s") {
		structName = structName[:len(structName)-1]
	}
	return structName
}
