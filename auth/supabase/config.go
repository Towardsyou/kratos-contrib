package supabase

// Config holds all configuration for the Supabase auth plugin.
type Config struct {
	// JWTSecret is the HMAC secret used to validate Supabase-issued JWTs.
	// Find it in Supabase dashboard → Project Settings → API → JWT Secret.
	JWTSecret string

	// SupabaseURL is the project REST endpoint, e.g. "https://xxx.supabase.co".
	SupabaseURL string

	// SupabaseKey is the publishable (anon) API key.
	SupabaseKey string

	// Whitelist contains operation names that bypass JWT validation.
	// For HTTP these are URL paths; for gRPC these are fully-qualified method names.
	// Example: []string{"/user.v1.UserService/Login", "/user.v1.UserService/Register"}
	Whitelist []string
}
