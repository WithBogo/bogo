package main

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// columnFrom parses one column definition line the way the schema parser does
func columnFrom(t *testing.T, line string) Column {
	t.Helper()
	col := parseColumnDefinition(line)
	if col == nil {
		t.Fatalf("not a column definition: %s", line)
	}
	generateGoFieldInfo(col)
	return *col
}

func TestCheck_IntegerColumnsGetIntegralLimits(t *testing.T) {
	cases := []struct{ line, want string }{
		{"rank INT CHECK (rank >= 1.5)", "omitempty,gte=2"},
		{"rank INT CHECK (rank > 1.5)", "omitempty,gt=1"},
		{"rank INT CHECK (rank <= 2.5)", "omitempty,lte=2"},
		{"rank INT CHECK (rank < 2.5)", "omitempty,lt=3"},
		{"rank INT CHECK (rank > 1)", "omitempty,gt=1"},
		{"score DECIMAL CHECK (score >= 1.5)", "omitempty,gte=1.5"},
	}
	for _, c := range cases {
		if got := columnFrom(t, c.line).Validate; got != c.want {
			t.Errorf("%s: got %q, want %q", c.line, got, c.want)
		}
	}
}

func TestCheck_OnlyFullySupportedExpressionsAreMirrored(t *testing.T) {
	cases := []struct {
		line      string
		wantRules bool
	}{
		{"age INT CHECK (age >= 18 AND age <= 120)", true},
		{"age INT CHECK ((age >= 18) AND (age <= 120))", true},
		{"age INT CHECK (age >= 18 OR age = 0)", false},
		{"age INT CHECK (age >= 18 AND other > 2)", false},
		{"age INT CHECK (age >= 1 AND age <= 5 OR age = 99)", false},
		{"age INT CHECK (age BETWEEN 1 AND 5)", false},
		{"age INT CHECK (age >= 18) DEFAULT 20", true},
	}
	for _, c := range cases {
		got := columnFrom(t, c.line).Validate != ""
		if got != c.wantRules {
			t.Errorf("%s: rules mirrored = %v, want %v", c.line, got, c.wantRules)
		}
	}
}

func TestCheck_StrictestBoundWins(t *testing.T) {
	cases := []struct{ line, want string }{
		{"age INT CHECK (age >= 1 AND age >= 5)", "omitempty,gte=5"},
		{"age INT CHECK (age >= 5 AND age >= 1)", "omitempty,gte=5"},
		{"age INT CHECK (age <= 50 AND age <= 20)", "omitempty,lte=20"},
		{"age INT CHECK (age >= 1) CHECK (age >= 7)", "omitempty,gte=7"},
	}
	for _, c := range cases {
		if got := columnFrom(t, c.line).Validate; got != c.want {
			t.Errorf("%s: got %q, want %q", c.line, got, c.want)
		}
	}
}

func TestCheck_InListsRespectQuotes(t *testing.T) {
	col := columnFrom(t, "label VARCHAR(20) CHECK (label IN ('a,b', 'it''s no', 'c'))")
	// the value with the quote is not representable in a tag, so the whole rule is skipped
	if col.Validate != "" && strings.Contains(col.Validate, "oneof") {
		t.Errorf("a list with an unrepresentable value must not become a partial oneof: %q", col.Validate)
	}

	col = columnFrom(t, "label VARCHAR(20) CHECK (label IN ('a,b', 'c d', 'e'))")
	if len(col.OneOf) != 3 || col.OneOf[0] != "a,b" || col.OneOf[1] != "c d" {
		t.Fatalf("commas and spaces inside quotes belong to the value: %q", col.OneOf)
	}
	if want := "omitempty,max=20,oneof=a0x2Cb 'c d' e"; col.Validate != want {
		t.Errorf("got %q, want %q", col.Validate, want)
	}
}

func TestCheck_ValuesThatCannotBeWrittenIntoATagSkipTheRule(t *testing.T) {
	for _, line := range []string{
		"label VARCHAR(20) CHECK (label IN ('say \"hi\"', 'x'))", // double quote breaks the struct tag
		"label VARCHAR(20) CHECK (label IN ('a`b', 'x'))",        // backtick ends the raw string
		"label VARCHAR(20) CHECK (label IN ('a\\b', 'x'))",       // backslash
	} {
		col := columnFrom(t, line)
		if len(col.OneOf) != 0 || strings.Contains(col.Validate, "oneof") {
			t.Errorf("%s: unsafe values must disable the rule, got %q %q", line, col.OneOf, col.Validate)
		}
	}
}

func TestCheck_PipesAreEscaped(t *testing.T) {
	col := columnFrom(t, "label VARCHAR(20) CHECK (label IN ('a|b', 'c'))")
	if !strings.Contains(col.Validate, "oneof=a0x7Cb c") {
		t.Errorf("a pipe must use the validator escape, got %q", col.Validate)
	}
}

func TestZeroValues_OptionalNumbersWhoseRulesRejectZeroUsePointers(t *testing.T) {
	optional := columnFrom(t, "age INT CHECK (age >= 18)")
	if !optional.usesPointer() {
		t.Error("an optional number that rejects 0 must be a pointer so an explicit 0 is validated")
	}
	if optional.Validate != "omitempty,gte=18" {
		t.Errorf("unexpected tag %q", optional.Validate)
	}

	mandatory := columnFrom(t, "age INT NOT NULL CHECK (age >= 18)")
	if mandatory.usesPointer() {
		t.Error("a mandatory number needs no pointer")
	}
	if mandatory.Validate != "gte=18" {
		t.Errorf("a mandatory number that rejects 0 must be validated even when it is 0, got %q", mandatory.Validate)
	}

	for _, line := range []string{
		"age INT CHECK (age >= 0)",
		"age INT CHECK (age >= -5 AND age <= 5)",
		"age INT CHECK (age IN (0, 1, 2))",
		"age INT",
	} {
		if columnFrom(t, line).usesPointer() {
			t.Errorf("%s: 0 is valid, so a plain field is enough", line)
		}
	}
	if !columnFrom(t, "age INT CHECK (age IN (1, 2, 3))").usesPointer() {
		t.Error("an IN list without 0 rejects 0")
	}
}

func TestZeroValues_DTOUsesPointerWhereNeeded(t *testing.T) {
	table := Table{Name: "people", Columns: []Column{
		columnFrom(t, "name VARCHAR(50) NOT NULL"),
		columnFrom(t, "age INT CHECK (age >= 18)"),
	}}
	dtoSource := generateDTO("svc", table)

	for _, want := range []string{
		"Age *int64",
		"if d.Age != nil {",
		"domainModel.Age = *d.Age",
		"d.Age = &v",
	} {
		if !strings.Contains(dtoSource, want) {
			t.Errorf("DTO is missing %q", want)
		}
	}
	if _, err := format.Source([]byte(dtoSource)); err != nil {
		t.Errorf("generated DTO does not parse: %v", err)
	}
}

func TestSamples_SatisfyEveryRule(t *testing.T) {
	cases := []string{
		"label VARCHAR(3) CHECK (label IN ('toolong', 'ok'))",
		"rank INT CHECK (rank >= 5)",
		"rank INT CHECK (rank > 5)",
		"rank INT CHECK (rank < 0)",
		"rank INT CHECK (rank > 1.5 AND rank < 2.5)",
		"score DECIMAL CHECK (score > 1.1 AND score < 1.2)",
		"score DECIMAL CHECK (score > 0.5)",
		"score DECIMAL CHECK (score < 0.5)",
		"score DECIMAL CHECK (score >= 7.25 AND score <= 7.25)",
		"score DECIMAL CHECK (score > -3 AND score < -2.5)",
		"level INT CHECK (level IN (3, 4))",
	}
	for _, line := range cases {
		col := columnFrom(t, line)
		if col.GoType == "string" {
			sample := sampleJSONValue(col)
			if len(col.OneOf) > 0 && !strings.Contains(sample, "ok") {
				t.Errorf("%s: sample %s ignores the column length", line, sample)
			}
			continue
		}
		v := sampleNumber(col)
		gte, gt, lte, lt := col.effectiveBounds()
		if !within(v, gte, gt, lte, lt) {
			t.Errorf("%s: sample %v violates the rules", line, v)
		}
	}
}

func TestGeneratedHandlerTestIsValidGo(t *testing.T) {
	table := Table{Name: "people", Columns: []Column{
		columnFrom(t, "name VARCHAR(50) NOT NULL"),
		columnFrom(t, "nickname VARCHAR(10)"),
		columnFrom(t, "age INT CHECK (age >= 18)"),
		columnFrom(t, "label VARCHAR(20) CHECK (label IN ('a|b', 'café', '<x>'))"),
	}}
	source := generateRestHandlerTest("svc", table)

	if _, err := format.Source([]byte(source)); err != nil {
		t.Fatalf("generated handler test does not parse: %v", err)
	}
	if !strings.Contains(source, "rejects an explicit zero for age") {
		t.Error("the explicit zero case must be tested")
	}
}

func TestWriteManagedFile_UnreadablePathIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	startTracking(dir, genOptions{Force: true})

	// a directory where the file should be: it exists but cannot be read as a file
	path := filepath.Join(dir, "main.go")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeManagedFile(path, "package main\n", 0o644); err == nil {
		t.Error("a path that exists but cannot be read must be reported, not treated as missing")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Error("the existing path must be left alone")
	}
}

func TestRebaseline_OnlyForFilesWrittenInThisRun(t *testing.T) {
	dir := t.TempDir()
	startTracking(dir, genOptions{})

	skipped := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(skipped, []byte("user edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracker.manifest["go.mod"] = "hash of the generated content"

	tracker.rebaseline(skipped)

	if tracker.manifest["go.mod"] != "hash of the generated content" {
		t.Error("a skipped file must keep its old baseline, its content is the user's")
	}

	tracker.written = append(tracker.written, skipped)
	tracker.rebaseline(skipped)
	if tracker.manifest["go.mod"] != hashContent([]byte("user edit")) {
		t.Error("a file written in this run takes the content left by go mod tidy as its baseline")
	}
}

func inDir(t *testing.T, dir string) {
	t.Helper()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
}

func migrationFiles(t *testing.T, service string) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(service, "migrations", "*.sql"))
	return files
}

func TestGooseMigration_NewTablesGetANewMigration(t *testing.T) {
	templates, _ := filepath.Abs("templates")
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "templates"), os.DirFS(templates)); err != nil {
		t.Fatal(err)
	}
	inDir(t, dir)
	startTracking("svc", genOptions{})

	users := Table{Name: "users", Columns: []Column{{Name: "name", Type: "TEXT", GoType: "string", IsNullable: true}}}
	orders := Table{Name: "orders", Columns: []Column{{Name: "total", Type: "BIGINT", GoType: "int64", IsNullable: true}}}

	// first run: one migration with everything
	if err := generateGooseMigration("svc", []Table{users}, ""); err != nil {
		t.Fatal(err)
	}
	files := migrationFiles(t, "svc")
	if len(files) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(files))
	}
	first, _ := os.ReadFile(files[0])

	// same schema again: nothing new
	if err := generateGooseMigration("svc", []Table{users}, ""); err != nil {
		t.Fatal(err)
	}
	if len(migrationFiles(t, "svc")) != 1 {
		t.Error("an unchanged schema must not add a migration")
	}

	// a table was added: the applied migration stays, a new one creates only the new table
	time.Sleep(1100 * time.Millisecond) // migration names carry a timestamp in seconds
	if err := generateGooseMigration("svc", []Table{users, orders}, ""); err != nil {
		t.Fatal(err)
	}
	files = migrationFiles(t, "svc")
	if len(files) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(files))
	}

	again, _ := os.ReadFile(files[0])
	if string(again) != string(first) {
		t.Error("an existing migration must never be rewritten, Goose will not run it again")
	}

	added, _ := os.ReadFile(files[1])
	if !strings.Contains(string(added), "orders") || strings.Contains(string(added), "users") {
		t.Errorf("the new migration must create only the new table:\n%s", added)
	}
	if !strings.Contains(files[1], "_add_svc_tables.sql") {
		t.Errorf("unexpected migration name %s", files[1])
	}
}
