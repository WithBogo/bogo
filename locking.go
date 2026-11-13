package main

import (
	"fmt"
	"strings"
)

// versionColumn returns the optimistic locking column of a table. A table opts in by
// declaring an integer column named "version".
func versionColumn(table Table) (Column, bool) {
	for _, col := range table.Columns {
		if strings.EqualFold(col.Name, "version") && col.GoType == "int64" {
			return col, true
		}
	}
	return Column{}, false
}

// repositoryCreateHook returns code that runs before Create. A new row starts at version 1.
func repositoryCreateHook(table Table, entityParam string) string {
	if _, ok := versionColumn(table); !ok {
		return ""
	}
	return fmt.Sprintf("\n\t%s.Version = 1", entityParam)
}

// repositoryUpdateBody returns the body of the repository Update method.
//
// Without a version column an update only touches rows that are not soft deleted.
// With one, the update succeeds only if the caller sent the current version, and the
// version is bumped in the same statement so concurrent writers cannot both win.
func repositoryUpdateBody(table Table, structName, entityParam string) string {
	if _, ok := versionColumn(table); !ok {
		return fmt.Sprintf(`
	result := repo.db.WithContext(ctx).
		Where("id = ? AND is_deleted = ?", %[1]s.ID, false).
		Updates(&%[1]s)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("%[2]s not found")
	}
	return nil`, entityParam, structName)
	}

	return fmt.Sprintf(`
	expected := %[1]s.Version
	%[1]s.Version = expected + 1

	result := repo.db.WithContext(ctx).
		Where("id = ? AND is_deleted = ? AND version = ?", %[1]s.ID, false, expected).
		Updates(&%[1]s)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Either the row is gone or somebody else updated it first
		var count int64
		result = repo.db.WithContext(ctx).Model(&model.%[2]s{}).
			Where("id = ? AND is_deleted = ?", %[1]s.ID, false).
			Count(&count)
		if result.Error != nil {
			return result.Error
		}
		if count == 0 {
			return errors.New("%[2]s not found")
		}
		return errors.New("%[2]s version conflict")
	}
	return nil`, entityParam, structName)
}

// restVersionConflictBlock returns the handler code that maps a version conflict to 409
func restVersionConflictBlock(table Table, singularName, entitySingular string) string {
	if _, ok := versionColumn(table); !ok {
		return ""
	}
	return fmt.Sprintf(`
		if err.Error() == "%[1]s version conflict" {
			wrapper := &responsewrapper.Wrapper{
				Error:   "The %[2]s was modified by someone else, fetch the latest version and retry",
				Message: "%[1]s version conflict",
				Code:    http.StatusConflict,
			}
			wrapper.Respond(w)
			return
		}
`, singularName, entitySingular)
}

func restVersionUpdateBlock(table Table, entityVar string) string {
	if _, ok := versionColumn(table); !ok {
		return ""
	}
	return fmt.Sprintf("\n\t%s.Version++", entityVar)
}
