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
type SignUpResponse struct {
	ID       uuid.UUID
	Email    string
	Username string
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

	username, _ := resp.User.UserMetadata["name"].(string)
	return &SignUpResponse{
		ID:       resp.User.ID,
		Email:    resp.User.Email,
		Username: username,
	}, nil
}
