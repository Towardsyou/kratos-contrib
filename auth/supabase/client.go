package supabase

import (
	supa "github.com/supabase-community/supabase-go"
)

// NewSupabaseClient creates a Supabase client from the given Config.
// The client exposes the full Supabase API (Auth, Storage, PostgREST, etc.).
//
// Typical usage with Google Wire:
//
//	func NewSupabaseClient(cfg supabase.Config) (*supa.Client, error) {
//	    return supabase.NewSupabaseClient(cfg)
//	}
func NewSupabaseClient(cfg Config) (*supa.Client, error) {
	return supa.NewClient(cfg.SupabaseURL, cfg.SupabaseKey, nil)
}
