// Example shows how to wire the supabase auth middleware into a Kratos HTTP/gRPC server.
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
			"/user.v1.UserService/Login",
			"/user.v1.UserService/Register",
		},
	}

	// Create Supabase client (for use in repositories/data layer).
	client, err := supabaseauth.NewSupabaseClient(cfg)
	if err != nil {
		panic(err)
	}
	_ = client // pass to your data layer via DI

	// Build the HTTP server with conditional JWT auth middleware.
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
	_ = httpSrv

	// In a handler, retrieve the authenticated user from context:
	exampleHandler := func(ctx context.Context) {
		info, ok := supabaseauth.FromContext(ctx)
		if !ok {
			fmt.Println("no auth info (public endpoint)")
			return
		}
		fmt.Printf("user id: %s, username: %s\n", info.UserID, info.Username)
	}
	_ = exampleHandler
}
