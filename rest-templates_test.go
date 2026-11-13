package main

import (
	"strings"
	"testing"
)

func TestGenerateRestAPIMain_DocsTableRoute(t *testing.T) {
	content := generateRestAPIMain("service", []Table{{Name: "docs"}})

	if !strings.Contains(content, `router.GET("/openapi/docs"`) {
		t.Error("Swagger UI route should use the OpenAPI namespace")
	}
	if !strings.Contains(content, `router.GET("/docs"`) {
		t.Error("docs table CRUD route should remain available")
	}
}

func TestGenerateRestHandler_ParsesIDAsInt64(t *testing.T) {
	content := generateRestHandler("service", Table{Name: "users", Columns: []Column{{Name: "name", GoType: "string", Type: "VARCHAR(255)"}}})

	if strings.Contains(content, "ParseInt(idStr, 10, 32)") {
		t.Error("ids are int64 (BIGSERIAL): parsing them as 32 bit rejects valid ids")
	}
	if got := strings.Count(content, "ParseInt(idStr, 10, 64)"); got != 3 {
		t.Errorf("expected get, update and delete to parse the id as 64 bit, found %d", got)
	}
}
