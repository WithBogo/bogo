package main

import (
	"strings"
	"testing"
)

func lockedTable() Table {
	return Table{Name: "accounts", Columns: []Column{
		{Name: "name", GoType: "string"},
		{Name: "version", GoType: "int64"},
	}}
}

func plainTable() Table {
	return Table{Name: "accounts", Columns: []Column{{Name: "name", GoType: "string"}}}
}

func TestVersionColumn(t *testing.T) {
	if _, ok := versionColumn(lockedTable()); !ok {
		t.Error("an integer version column must enable optimistic locking")
	}
	if _, ok := versionColumn(plainTable()); ok {
		t.Error("tables without a version column must not use locking")
	}

	textVersion := Table{Name: "docs", Columns: []Column{{Name: "version", GoType: "string"}}}
	if _, ok := versionColumn(textVersion); ok {
		t.Error("a non integer version column (for example a semver string) must not enable locking")
	}
}

func TestGeneratePostgresRepository_OptimisticLocking(t *testing.T) {
	repo := generatePostgresRepository("svc", lockedTable())

	for _, want := range []string{
		"version = ?",
		"expected + 1",
		`errors.New("Account version conflict")`,
		"account.Version = 1",
	} {
		if !strings.Contains(repo, want) {
			t.Errorf("locked repository is missing %q", want)
		}
	}
}

func TestGeneratePostgresRepository_WithoutLocking(t *testing.T) {
	repo := generatePostgresRepository("svc", plainTable())

	if strings.Contains(repo, "version") {
		t.Error("tables without a version column must not mention versions")
	}
	if !strings.Contains(repo, "is_deleted = ?") {
		t.Error("updates must skip soft deleted rows")
	}
}

func TestGeneratePostgresRepository_SoftDeleteAndNotFound(t *testing.T) {
	repo := generatePostgresRepository("svc", plainTable())

	for _, want := range []string{
		`"deleted_at": time.Now()`,
		`errors.New("Account not found")`,
		"gorm.ErrRecordNotFound",
	} {
		if !strings.Contains(repo, want) {
			t.Errorf("repository is missing %q", want)
		}
	}
}

func TestRestVersionConflictBlock(t *testing.T) {
	block := restVersionConflictBlock(lockedTable(), "Account", "account")
	if !strings.Contains(block, "http.StatusConflict") {
		t.Error("a version conflict must map to 409")
	}
	if restVersionConflictBlock(plainTable(), "Account", "account") != "" {
		t.Error("tables without locking must not get a conflict branch")
	}
}

func TestGenerateRestHandler_VersionedUpdateReturnsNewVersion(t *testing.T) {
	handler := generateRestHandler("svc", lockedTable(), nil)
	if !strings.Contains(handler, "account.Version++") {
		t.Error("a successful versioned update must return the incremented version")
	}

	handler = generateRestHandler("svc", plainTable(), nil)
	if strings.Contains(handler, "Version++") {
		t.Error("tables without a version column must not increment the response version")
	}
}
