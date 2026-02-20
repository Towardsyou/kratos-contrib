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
			// The token endpoint and registration are public.
			"/oauth/token",
			"/user.v1.UserService/Register",
		},
	}

	authClient, err := supabaseauth.NewAuthClient(cfg)
	if err != nil {
		panic(err)
	}

	// --- Sign up a new user ---
	user, err := authClient.SignUp(context.Background(), supabaseauth.SignUpRequest{
		Email:    "user@example.com",
		Password: "secret",
		Username: "alice",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("registered: %s (%s)\n", user.Username, user.ID)

	// --- HTTP server wiring ---
	//
	// TokenHandler exposes POST /oauth/token (RFC 6749 password grant).
	// Protected routes use the JWT middleware; /oauth/token is in the whitelist.
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

	// Register the RFC 6749 token endpoint.
	httpSrv.Handle("/oauth/token", supabaseauth.TokenHandler(authClient))

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
