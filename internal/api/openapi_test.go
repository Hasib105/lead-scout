package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lead-scout/internal/config"
)

func TestOpenAPIJSONIsValid(t *testing.T) {
	var spec map[string]any
	if err := json.Unmarshal([]byte(openAPIJSON), &spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] != "3.1.0" {
		t.Fatalf("openapi version = %#v", spec["openapi"])
	}
}

func TestDocsRouteServesScalarReference(t *testing.T) {
	server := NewServer(config.Config{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Fatal("docs page does not load Scalar")
	}
	if !strings.Contains(body, "url: '/openapi.json'") {
		t.Fatal("docs page does not point at local OpenAPI spec")
	}
}

func TestHealthRoute(t *testing.T) {
	server := NewServer(config.Config{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
