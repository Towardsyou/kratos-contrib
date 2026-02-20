// Example shows how to serve Swagger UI and an OpenAPI spec on a Kratos HTTP server.
package main

import (
	swaggerui "github.com/towardsyou/kratos-contrib/swagger/ui"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

func main() {
	httpSrv := kratoshttp.NewServer(
		kratoshttp.Address(":8080"),
	)

	// Serve the OpenAPI spec file.
	// Accessible at: http://localhost:8080/swagger/openapi.yaml
	httpSrv.Handle("/swagger/openapi.yaml", swaggerui.SpecHandler("./openapi.yaml"))

	// Serve the Swagger UI page.
	// Accessible at: http://localhost:8080/swagger/
	// The UI fetches the spec from /swagger/openapi.yaml at runtime.
	// WithOAuth2 pre-configures the Authorize dialog for OAuth2 password grant:
	// the user enters email + password and Swagger UI calls /oauth/token automatically.
	httpSrv.Handle("/swagger/", swaggerui.UIHandler(
		"/swagger/openapi.yaml",
		swaggerui.WithTitle("Example API"),
		swaggerui.WithOAuth2("", ""), // clientID and scopes are empty for Supabase password grant
	))

	if err := httpSrv.Start(nil); err != nil {
		panic(err)
	}
}
