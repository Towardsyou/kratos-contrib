// Package swaggerui provides HTTP handlers for serving an OpenAPI spec file
// and the corresponding Swagger UI, designed to be registered on a Kratos HTTP server.
//
// Usage:
//
//	// Serve the UI at /swagger/ (HTML page)
//	httpSrv.Handle("/swagger/", swaggerui.UIHandler(
//	    "/swagger/openapi.yaml",
//	    swaggerui.WithTitle("My API"),
//	))
//
//	// Serve the spec file at /swagger/openapi.yaml
//	httpSrv.Handle("/swagger/openapi.yaml", swaggerui.SpecHandler("./configs/openapi.yaml"))
package swaggerui

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultTitle      = "API Documentation"
	defaultCDNVersion = "5"
)

type config struct {
	title      string
	cdnVersion string
}

// Option configures the Swagger UI handler.
type Option func(*config)

// WithTitle sets the HTML page title shown in the browser tab.
// Default: "API Documentation".
func WithTitle(title string) Option {
	return func(c *config) { c.title = title }
}

// WithCDNVersion pins the swagger-ui-dist version loaded from unpkg.com.
// Default: "5" (latest v5.x).
//
// Example: swaggerui.WithCDNVersion("5.17.14")
func WithCDNVersion(version string) Option {
	return func(c *config) { c.cdnVersion = version }
}

// UIHandler returns an http.Handler that serves the Swagger UI HTML page.
//
// specURL is the URL at which the OpenAPI spec is accessible by the browser,
// e.g. "/swagger/openapi.yaml". It is embedded in the page so the Swagger UI
// JavaScript can fetch it at runtime.
//
// The UI assets (CSS, JS) are loaded from the unpkg CDN — no local files needed.
func UIHandler(specURL string, opts ...Option) http.Handler {
	cfg := &config{title: defaultTitle, cdnVersion: defaultCDNVersion}
	for _, opt := range opts {
		opt(cfg)
	}

	tmpl := template.Must(template.New("swagger-ui").Parse(swaggerUIHTML))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, map[string]string{
			"Title":      cfg.title,
			"SpecURL":    specURL,
			"CDNVersion": cfg.cdnVersion,
		})
	})
}

// SpecHandler returns an http.Handler that reads an OpenAPI spec file from
// the filesystem on every request and serves it with the appropriate
// Content-Type (application/yaml or application/json).
//
// The file is read on each request so that live-reload workflows work
// without restarting the server.
//
// CORS header Access-Control-Allow-Origin: * is set so browser-based tools
// can fetch the spec across origins.
func SpecHandler(specFilePath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(specFilePath) // #nosec G304 -- path is caller-supplied
		if err != nil {
			http.Error(w, "spec file not found", http.StatusNotFound)
			return
		}

		ct := contentTypeForSpec(specFilePath)
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func contentTypeForSpec(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "application/yaml; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
