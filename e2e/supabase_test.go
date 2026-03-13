//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	supabaseauth "github.com/towardsyou/kratos-contrib/auth/supabase"
)

// newAuthClient creates an AuthClient pointed at the local docker GoTrue
// instance (via the nginx proxy).
func newAuthClient(t *testing.T) *supabaseauth.AuthClient {
	t.Helper()
	cfg := supabaseauth.Config{
		JWTSecret:   jwtSecret,
		SupabaseURL: supabaseURL,
		SupabaseKey: supabaseKey,
	}
	client, err := supabaseauth.NewAuthClient(cfg)
	if err != nil {
		t.Fatalf("NewAuthClient: %v", err)
	}
	return client
}

// TestSignUp verifies that a new user can be registered and, because the local
// GoTrue has GOTRUE_MAILER_AUTOCONFIRM=true, the response includes an access token.
func TestSignUp(t *testing.T) {
	requireGoTrue(t)
	client := newAuthClient(t)

	// Test via AuthClient method directly.
	resp, err := client.SignUp(t.Context(), supabaseauth.SignUpRequest{
		Email:    randomEmail(),
		Password: "Password1!",
		Username: "alice",
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if resp.ID.String() == "" || resp.ID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Errorf("expected non-zero ID, got %s", resp.ID)
	}
	if resp.Email == "" {
		t.Error("expected non-empty Email")
	}
	// With autoconfirm on, access_token should be present.
	if resp.AccessToken == "" {
		t.Error("expected non-empty AccessToken (GOTRUE_MAILER_AUTOCONFIRM=true)")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty RefreshToken")
	}

	// Test via the HTTP handler.
	srv := httptest.NewServer(supabaseauth.SignUpHandler(client))
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{
		"email":    randomEmail(),
		"password": "Password1!",
		"username": "bob",
	})
	httpResp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/signup: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("expected 201, got %d: %s", httpResp.StatusCode, raw)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] == nil || result["id"] == "" {
		t.Errorf("expected id in response, got %v", result)
	}
	if result["access_token"] == nil {
		t.Error("expected access_token in response (GOTRUE_MAILER_AUTOCONFIRM=true)")
	}
}

// TestLogin verifies the password grant flow through TokenHandler.
func TestLogin(t *testing.T) {
	requireGoTrue(t)
	client := newAuthClient(t)

	email := randomEmail()
	password := "Password1!"

	// Pre-register the user.
	if _, err := client.SignUp(t.Context(), supabaseauth.SignUpRequest{
		Email: email, Password: password,
	}); err != nil {
		t.Fatalf("setup SignUp: %v", err)
	}

	srv := httptest.NewServer(supabaseauth.TokenHandler(client))
	defer srv.Close()

	body := fmt.Sprintf("grant_type=password&username=%s&password=%s", email, password)
	httpResp, err := http.Post(srv.URL, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /oauth/token: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("expected 200, got %d: %s", httpResp.StatusCode, raw)
	}

	var tokenResp map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenResp["access_token"] == nil || tokenResp["access_token"] == "" {
		t.Errorf("expected access_token in response, got %v", tokenResp)
	}
	if tokenResp["refresh_token"] == nil || tokenResp["refresh_token"] == "" {
		t.Errorf("expected refresh_token in response, got %v", tokenResp)
	}
	if tokenResp["token_type"] != "bearer" {
		t.Errorf("expected token_type=bearer, got %v", tokenResp["token_type"])
	}
}

// TestRefreshToken verifies the refresh_token grant flow through TokenHandler.
func TestRefreshToken(t *testing.T) {
	requireGoTrue(t)
	client := newAuthClient(t)

	email := randomEmail()
	password := "Password1!"

	// Pre-register and log in to obtain a refresh token.
	signUpResp, err := client.SignUp(t.Context(), supabaseauth.SignUpRequest{
		Email: email, Password: password,
	})
	if err != nil {
		t.Fatalf("setup SignUp: %v", err)
	}
	initialRefreshToken := signUpResp.RefreshToken
	if initialRefreshToken == "" {
		t.Fatal("setup: expected refresh token from signup (GOTRUE_MAILER_AUTOCONFIRM=true)")
	}

	srv := httptest.NewServer(supabaseauth.TokenHandler(client))
	defer srv.Close()

	body := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s", initialRefreshToken)
	httpResp, err := http.Post(srv.URL, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /oauth/token (refresh): %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("expected 200, got %d: %s", httpResp.StatusCode, raw)
	}

	var tokenResp map[string]interface{}
	if err := json.NewDecoder(httpResp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if tokenResp["access_token"] == nil || tokenResp["access_token"] == "" {
		t.Errorf("expected new access_token, got %v", tokenResp)
	}
	newRefreshToken, _ := tokenResp["refresh_token"].(string)
	if newRefreshToken == "" {
		t.Error("expected new refresh_token in response")
	}
}

// TestForgotPassword verifies that ForgotPasswordHandler always returns 200,
// both for registered and unregistered email addresses.
func TestForgotPassword(t *testing.T) {
	requireGoTrue(t)
	client := newAuthClient(t)

	// Pre-register a user so we have a known address.
	email := randomEmail()
	if _, err := client.SignUp(t.Context(), supabaseauth.SignUpRequest{
		Email: email, Password: "Password1!",
	}); err != nil {
		t.Fatalf("setup SignUp: %v", err)
	}

	srv := httptest.NewServer(supabaseauth.ForgotPasswordHandler(client))
	defer srv.Close()

	for _, tc := range []struct {
		name  string
		email string
	}{
		{"registered email", email},
		{"unregistered email", randomEmail()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"email": tc.email})
			resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("POST /auth/forgot-password: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

// TestChangePassword verifies the full change-password flow:
// sign up → log in → change password → verify old password fails → verify new password works.
func TestChangePassword(t *testing.T) {
	requireGoTrue(t)
	client := newAuthClient(t)

	email := randomEmail()
	oldPassword := "OldPassword1!"
	newPassword := "NewPassword2@"

	// Sign up (autoconfirm on → access token returned immediately).
	signUpResp, err := client.SignUp(t.Context(), supabaseauth.SignUpRequest{
		Email: email, Password: oldPassword,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	accessToken := signUpResp.AccessToken
	if accessToken == "" {
		t.Fatal("expected access token from signup")
	}

	// Change password via the HTTP handler.
	pwSrv := httptest.NewServer(supabaseauth.ChangePasswordHandler(client))
	defer pwSrv.Close()

	body, _ := json.Marshal(map[string]string{"new_password": newPassword})
	req, _ := http.NewRequest(http.MethodPost, pwSrv.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /auth/password: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(httpResp.Body)
		t.Fatalf("expected 204, got %d: %s", httpResp.StatusCode, raw)
	}

	// Verify old password no longer works.
	tokenSrv := httptest.NewServer(supabaseauth.TokenHandler(client))
	defer tokenSrv.Close()

	oldBody := fmt.Sprintf("grant_type=password&username=%s&password=%s", email, oldPassword)
	badResp, err := http.Post(tokenSrv.URL, "application/x-www-form-urlencoded", strings.NewReader(oldBody))
	if err != nil {
		t.Fatalf("login with old password: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for old password, got %d", badResp.StatusCode)
	}

	// Verify new password works.
	newBody := fmt.Sprintf("grant_type=password&username=%s&password=%s", email, newPassword)
	goodResp, err := http.Post(tokenSrv.URL, "application/x-www-form-urlencoded", strings.NewReader(newBody))
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	defer goodResp.Body.Close()
	if goodResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(goodResp.Body)
		t.Fatalf("expected 200 for new password, got %d: %s", goodResp.StatusCode, raw)
	}
}
