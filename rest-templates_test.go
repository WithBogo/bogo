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
