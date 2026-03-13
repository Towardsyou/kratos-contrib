//go:build e2e

// Package e2e contains end-to-end tests for all plugins in this repository.
//
// # Running all tests (docker required for Supabase tests)
//
//	make e2e
//
// # Running only the docker-free tests
//
//	cd e2e && go test -v -tags e2e -run "TestSwagger|TestOTel" -count=1 ./...
//
// Supabase tests skip automatically when the local GoTrue service
// (docker-compose) is not reachable.
package e2e

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

const (
	// supabaseURL is the nginx proxy URL; supabase-go appends /auth/v1 to this
	// and nginx strips that prefix before forwarding to GoTrue at :9999.
	supabaseURL = "http://localhost:8000"
	supabaseKey = "test-anon-key"
	jwtSecret   = "super-secret-jwt-token-with-at-least-32-characters-long"
	gotrueURL   = "http://localhost:9999"
)

// requireGoTrue calls t.Skip when the local GoTrue instance is not reachable.
// Call this at the start of any test that depends on docker-compose services.
func requireGoTrue(t *testing.T) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(gotrueURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("GoTrue not reachable — start docker-compose services with: docker compose -f e2e/docker-compose.yml up -d --wait")
	}
	if resp != nil {
		resp.Body.Close()
	}
}

// randomEmail returns a unique email address suitable for use in tests.
func randomEmail() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return fmt.Sprintf("%s@e2e-test.invalid", string(b))
}
