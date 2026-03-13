package supabase

import (
	"context"

	"github.com/google/uuid"
	gotruetypes "github.com/supabase-community/gotrue-go/types"
	supa "github.com/supabase-community/supabase-go"
)

// SignUpRequest holds the fields required to create a new user.
type SignUpRequest struct {
	Email    string
	Password string
	Username string
}

// SignUpResponse contains the newly created user's identity.
// When Supabase email auto-confirm is enabled the session fields are also populated,
// so the caller can skip a separate login step.
type SignUpResponse struct {
	ID       uuid.UUID
	Email    string
	Username string

	// Session fields — populated only when the project has email auto-confirm enabled.
	AccessToken  string
	TokenType    string
	ExpiresIn    int
	RefreshToken string
}

// AuthClient wraps a Supabase client and exposes auth operations.
type AuthClient struct {
	client *supa.Client
}

// NewAuthClient creates an AuthClient from the given Config.
// It is a thin convenience wrapper around [NewSupabaseClient].
func NewAuthClient(cfg Config) (*AuthClient, error) {
	client, err := NewSupabaseClient(cfg)
	if err != nil {
		return nil, err
	}
	return &AuthClient{client: client}, nil
}

// SignUp registers a new user via Supabase Auth.
// The Username is stored in Supabase user_metadata under the key "name".
// When email auto-confirm is enabled the returned response also carries session tokens.
func (a *AuthClient) SignUp(_ context.Context, req SignUpRequest) (*SignUpResponse, error) {
	resp, err := a.client.Auth.Signup(gotruetypes.SignupRequest{
		Email:    req.Email,
		Password: req.Password,
		Data: map[string]interface{}{
			"name": req.Username,
		},
	})
	if err != nil {
		return nil, err
	}

	username, _ := resp.UserMetadata["name"].(string)
	out := &SignUpResponse{
		ID:       resp.ID,
		Email:    resp.Email,
		Username: username,
	}

	// Populate session fields when auto-confirm is on.
	if resp.AccessToken != "" {
		out.AccessToken = resp.AccessToken
		out.TokenType = resp.TokenType
		out.ExpiresIn = resp.ExpiresIn
		out.RefreshToken = resp.RefreshToken
	}

	return out, nil
}

// UpdatePassword changes the password for an authenticated user.
// accessToken must be a valid Supabase access token for the user whose password is being changed.
// This is also used to set a new password with a recovery token obtained from a password-reset email.
func (a *AuthClient) UpdatePassword(_ context.Context, accessToken, newPassword string) error {
	_, err := a.client.Auth.WithToken(accessToken).UpdateUser(gotruetypes.UpdateUserRequest{
		Password: &newPassword,
	})
	return err
}

// RequestPasswordReset sends a password-reset email to the given address via Supabase Auth.
// The email contains a magic link that, when followed, provides a short-lived recovery token.
// The caller must then exchange that token for a new password using [UpdatePassword] (or the
// HTTP ChangePasswordHandler with the recovery token as the Bearer credential).
//
// GoTrue rate-limits this endpoint to one email per 60 seconds by default.
func (a *AuthClient) RequestPasswordReset(_ context.Context, email string) error {
	return a.client.Auth.Recover(gotruetypes.RecoverRequest{Email: email})
}
