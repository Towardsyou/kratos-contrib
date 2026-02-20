package supabase

import (
	"encoding/json"
	"net/http"
	"strings"
)

// TokenResponse is the RFC 6749 §5.1 successful token response.
// Field names match the spec exactly so they serialize correctly over the wire.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// tokenErrorResponse is the RFC 6749 §5.2 error response.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// TokenHandler returns a [net/http.Handler] that implements the OAuth2
// Resource Owner Password Credentials grant (RFC 6749 §4.3).
//
// The handler accepts POST requests with Content-Type
// application/x-www-form-urlencoded and the following fields:
//
//	grant_type  (required) must be "password"
//	username    (required) user's email address
//	password    (required)
//	scope       (optional, ignored — Supabase manages scopes internally)
//
// On success it returns HTTP 200 with a JSON [TokenResponse].
// On failure it returns the appropriate 4xx with a JSON error body per §5.2.
//
// Register it on a Kratos HTTP server:
//
//	srv.Handle("/oauth/token", supabase.TokenHandler(authClient))
func TokenHandler(client *AuthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeTokenError(w, http.StatusMethodNotAllowed, "invalid_request", "only POST is accepted")
			return
		}

		// RFC 6749 §4.3.1: request MUST use application/x-www-form-urlencoded.
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/x-www-form-urlencoded") {
			writeTokenError(w, http.StatusUnsupportedMediaType, "invalid_request",
				"Content-Type must be application/x-www-form-urlencoded")
			return
		}

		if err := r.ParseForm(); err != nil {
			writeTokenError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}

		grantType := r.FormValue("grant_type")
		switch grantType {
		case "":
			writeTokenError(w, http.StatusBadRequest, "invalid_request", "grant_type is required")
			return
		case "password":
			// handled below
		default:
			writeTokenError(w, http.StatusBadRequest, "unsupported_grant_type",
				"only grant_type=password is supported")
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == "" || password == "" {
			writeTokenError(w, http.StatusBadRequest, "invalid_request", "username and password are required")
			return
		}

		resp, err := client.client.SignInWithEmailPassword(username, password)
		if err != nil {
			// Treat any Supabase auth failure as invalid_grant (wrong credentials).
			writeTokenError(w, http.StatusUnauthorized, "invalid_grant", err.Error())
			return
		}

		// RFC 6749 §5.1: disable caching on the token response.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  resp.AccessToken,
			TokenType:    strings.ToLower(resp.TokenType), // spec requires lowercase "bearer"
			ExpiresIn:    resp.ExpiresIn,
			RefreshToken: resp.RefreshToken,
		})
	})
}

func writeTokenError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(tokenErrorResponse{
		Error:            code,
		ErrorDescription: desc,
	})
}
