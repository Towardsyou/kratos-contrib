// Example shows how to wire the supabase auth plugin into a Kratos application.
package main

import (
	"context"
	"fmt"

	supabaseauth "github.com/towardsyou/kratos-contrib/auth/supabase"

	"github.com/go-kratos/kratos/v2/middleware/selector"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

func main() {
	cfg := supabaseauth.Config{
		JWTSecret:   "<your-jwt-secret>",
		SupabaseURL: "https://xxx.supabase.co",
		SupabaseKey: "sb_publishable_xxx",
		Whitelist: []string{
			// Public endpoints — skip JWT middleware.
			"/oauth/token",
			"/auth/signup",
			"/auth/forgot-password",
		},
	}

	authClient, err := supabaseauth.NewAuthClient(cfg)
	if err != nil {
		panic(err)
	}

	// --- Sign up a new user via AuthClient directly ---
	user, err := authClient.SignUp(context.Background(), supabaseauth.SignUpRequest{
		Email:    "user@example.com",
		Password: "secret",
		Username: "alice",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("registered: %s (%s)\n", user.Username, user.ID)
	// When email auto-confirm is enabled, user.AccessToken is already populated.

	// --- HTTP server wiring ---
	httpSrv := kratoshttp.NewServer(
		kratoshttp.Address(":8080"),
		kratoshttp.Middleware(
			selector.Server(
				supabaseauth.NewAuthMiddleware(cfg),
			).Match(
				supabaseauth.NewWhitelistMatcher(cfg.Whitelist),
			).Build(),
		),
	)

	// POST /auth/signup  {"email","password","username"}  → 201 {id,email,...}
	// Public — no JWT required.
	httpSrv.Handle("/auth/signup", supabaseauth.SignUpHandler(authClient))

	// POST /oauth/token  grant_type=password  → 200 {access_token,refresh_token,...}
	// POST /oauth/token  grant_type=refresh_token  → 200 {access_token,refresh_token,...}
	// Public — no JWT required.
	httpSrv.Handle("/oauth/token", supabaseauth.TokenHandler(authClient))

	// POST /auth/forgot-password  {"email":"..."}  → 200 (always, to prevent user enumeration)
	// Public — no JWT required. Supabase sends a reset email with a recovery token.
	httpSrv.Handle("/auth/forgot-password", supabaseauth.ForgotPasswordHandler(authClient))

	// POST /auth/password  Authorization: Bearer <token_or_recovery_token>  {"new_password":"..."}  → 204
	// Accepts both a regular access token (change password) and a recovery token from the reset email.
	// Protected — JWT middleware validates the token.
	httpSrv.Handle("/auth/password", supabaseauth.ChangePasswordHandler(authClient))

	_ = httpSrv

	// --- Inside a protected handler ---
	exampleHandler := func(ctx context.Context) {
		info, ok := supabaseauth.FromContext(ctx)
		if !ok {
			fmt.Println("no auth info")
			return
		}
		fmt.Printf("user id: %s, username: %s\n", info.UserID, info.Username)
	}
	_ = exampleHandler
}
