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
// Resource Owner Password Credentials grant (RFC 6749 §4.3) and the
// Refresh Token grant (RFC 6749 §6).
//
// The handler accepts POST requests with Content-Type
// application/x-www-form-urlencoded and the following fields:
//
// For grant_type=password:
//
//	grant_type  (required) must be "password"
//	username    (required) user's email address
//	password    (required)
//	scope       (optional, ignored — Supabase manages scopes internally)
//
// For grant_type=refresh_token:
//
//	grant_type     (required) must be "refresh_token"
//	refresh_token  (required)
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
			handlePasswordGrant(w, r, client)
		case "refresh_token":
			handleRefreshTokenGrant(w, r, client)
		default:
			writeTokenError(w, http.StatusBadRequest, "unsupported_grant_type",
				"only grant_type=password and grant_type=refresh_token are supported")
		}
	})
}

func handlePasswordGrant(w http.ResponseWriter, r *http.Request, client *AuthClient) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "username and password are required")
		return
	}

	resp, err := client.client.SignInWithEmailPassword(username, password)
	if err != nil {
		writeTokenError(w, http.StatusUnauthorized, "invalid_grant", err.Error())
		return
	}

	writeTokenSuccess(w, TokenResponse{
		AccessToken:  resp.AccessToken,
		TokenType:    strings.ToLower(resp.TokenType),
		ExpiresIn:    resp.ExpiresIn,
		RefreshToken: resp.RefreshToken,
	})
}

func handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request, client *AuthClient) {
	refreshToken := r.FormValue("refresh_token")
	if refreshToken == "" {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}

	resp, err := client.client.Auth.RefreshToken(refreshToken)
	if err != nil {
		writeTokenError(w, http.StatusUnauthorized, "invalid_grant", err.Error())
		return
	}

	writeTokenSuccess(w, TokenResponse{
		AccessToken:  resp.AccessToken,
		TokenType:    strings.ToLower(resp.TokenType),
		ExpiresIn:    resp.ExpiresIn,
		RefreshToken: resp.RefreshToken,
	})
}

// signUpRequest is the JSON body expected by [SignUpHandler].
type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// signUpResponse is the JSON body returned by [SignUpHandler].
type signUpResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// SignUpHandler returns an [net/http.Handler] that registers a new user.
//
// The handler accepts POST requests with Content-Type application/json and body:
//
//	{"email": "...", "password": "...", "username": "..."}
//
// On success it returns HTTP 201 with a JSON body containing the new user's
// id, email, and username. When the Supabase project has email auto-confirm
// enabled the response also includes access_token, token_type, expires_in, and
// refresh_token so the caller can skip a separate login step.
//
// Register it on a Kratos HTTP server:
//
//	srv.Handle("/auth/signup", supabase.SignUpHandler(authClient))
func SignUpHandler(client *AuthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "only POST is accepted")
			return
		}

		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			writeJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		var req signUpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed JSON body")
			return
		}
		if req.Email == "" || req.Password == "" {
			writeJSONError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		result, err := client.SignUp(r.Context(), SignUpRequest(req))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp := signUpResponse{
			ID:           result.ID.String(),
			Email:        result.Email,
			Username:     result.Username,
			AccessToken:  result.AccessToken,
			TokenType:    strings.ToLower(result.TokenType),
			ExpiresIn:    result.ExpiresIn,
			RefreshToken: result.RefreshToken,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// changePasswordRequest is the JSON body expected by [ChangePasswordHandler].
type changePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ChangePasswordHandler returns an [net/http.Handler] that changes the
// authenticated user's password.
//
// The handler accepts POST requests with:
//   - Authorization: Bearer <access_token> header
//   - Content-Type: application/json
//   - Body: {"new_password": "..."}
//
// On success it returns HTTP 204 No Content.
//
// Register it on a Kratos HTTP server (behind the auth middleware):
//
//	srv.Handle("/auth/password", supabase.ChangePasswordHandler(authClient))
func ChangePasswordHandler(client *AuthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "only POST is accepted")
			return
		}

		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			writeJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSONError(w, http.StatusUnauthorized, "invalid Authorization format, expected: Bearer <token>")
			return
		}
		accessToken := parts[1]

		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed JSON body")
			return
		}
		if req.NewPassword == "" {
			writeJSONError(w, http.StatusBadRequest, "new_password is required")
			return
		}

		if err := client.UpdatePassword(r.Context(), accessToken, req.NewPassword); err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// forgotPasswordRequest is the JSON body expected by [ForgotPasswordHandler].
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPasswordHandler returns an [net/http.Handler] that triggers a
// password-reset email for the given address.
//
// The handler accepts POST requests with Content-Type application/json and body:
//
//	{"email": "user@example.com"}
//
// On success it returns HTTP 200. The response body is intentionally empty so
// that the endpoint cannot be used to enumerate registered email addresses —
// the same 200 is returned regardless of whether the address exists.
//
// The client must then:
//  1. Wait for the user to click the link in the reset email.
//  2. Extract the short-lived recovery token from the link's query parameters.
//  3. Call [ChangePasswordHandler] (POST /auth/password) with that recovery
//     token as the Bearer credential and the desired new password in the body.
//
// Register it on a Kratos HTTP server (public endpoint, no JWT required):
//
//	srv.Handle("/auth/forgot-password", supabase.ForgotPasswordHandler(authClient))
func ForgotPasswordHandler(client *AuthClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "only POST is accepted")
			return
		}

		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			writeJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		var req forgotPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed JSON body")
			return
		}
		if req.Email == "" {
			writeJSONError(w, http.StatusBadRequest, "email is required")
			return
		}

		// Always return 200 even if the email is not registered, to prevent
		// user enumeration attacks.
		_ = client.RequestPasswordReset(r.Context(), req.Email)
		w.WriteHeader(http.StatusOK)
	})
}

func writeTokenSuccess(w http.ResponseWriter, resp TokenResponse) {
	// RFC 6749 §5.1: disable caching on the token response.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
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

// jsonErrorResponse is used by non-token handlers.
type jsonErrorResponse struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonErrorResponse{Error: msg})
}
