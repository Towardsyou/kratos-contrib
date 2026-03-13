//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	swaggerui "github.com/towardsyou/kratos-contrib/swagger/ui"
)

// TestSwaggerUI verifies that UIHandler returns a valid Swagger UI HTML page.
func TestSwaggerUI(t *testing.T) {
	const specURL = "/swagger/openapi.yaml"

	mux := http.NewServeMux()
	mux.Handle("/swagger/", swaggerui.UIHandler(
		specURL,
		swaggerui.WithTitle("E2E Test API"),
		swaggerui.WithOAuth2("", ""),
	))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/swagger/")
	if err != nil {
		t.Fatalf("GET /swagger/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)

	for _, want := range []string{
		"E2E Test API",          // custom title injected
		"swagger-ui",            // swagger-ui div id
		"swagger-ui-bundle.js", // CDN script
		"openapi.yaml",          // spec filename embedded in page (path may be JS-escaped)
	} {
		if !strings.Contains(html, want) {
			t.Errorf("response HTML missing %q", want)
		}
	}
}

// TestSpecHandler verifies that SpecHandler reads and serves an OpenAPI YAML file.
func TestSpecHandler(t *testing.T) {
	// Write a minimal spec to a temp file.
	dir := t.TempDir()
	specFile := filepath.Join(dir, "openapi.yaml")
	specContent := `openapi: "3.0.3"
info:
  title: E2E Spec
  version: "0.0.1"
paths: {}
`
	if err := os.WriteFile(specFile, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	srv := httptest.NewServer(swaggerui.SpecHandler(specFile))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET spec: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "yaml") {
		t.Errorf("expected yaml Content-Type, got %q", ct)
	}

	// CORS header required for browser-based tools.
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", got)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !strings.Contains(string(body), "E2E Spec") {
		t.Errorf("spec body does not contain expected content: %s", body)
	}
}

// TestSpecHandlerNotFound verifies that SpecHandler returns 404 for missing files.
func TestSpecHandlerNotFound(t *testing.T) {
	srv := httptest.NewServer(swaggerui.SpecHandler("/nonexistent/path.yaml"))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET missing spec: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
