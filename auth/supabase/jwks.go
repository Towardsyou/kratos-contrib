package supabase

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

const jwksMaxBytes = 1 << 20

var jwksHTTP = &http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type jwksCache struct {
	endpoint string

	mu          sync.Mutex
	keys        jose.JSONWebKeySet
	expires     time.Time
	nextRefresh time.Time
}

func (c *jwksCache) key(ctx context.Context, token *jwt.Token) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("JWT has no key ID")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Before(c.expires) {
		if candidate := c.find(kid, token.Method.Alg()); candidate != nil {
			return candidate, nil
		}
	}
	// Bound refreshes triggered by unrecognized key IDs while allowing rotation.
	if now.Before(c.nextRefresh) {
		return nil, fmt.Errorf("JWT verification key unavailable")
	}
	c.nextRefresh = now.Add(10 * time.Second)
	keys, err := fetchJWKS(ctx, c.endpoint)
	if err != nil {
		return nil, err
	}
	c.keys = keys
	c.expires = time.Now().Add(5 * time.Minute)
	if candidate := c.find(kid, token.Method.Alg()); candidate != nil {
		return candidate, nil
	}
	return nil, fmt.Errorf("JWT verification key unavailable")
}

func (c *jwksCache) find(kid, algorithm string) crypto.PublicKey {
	var candidate crypto.PublicKey
	for _, key := range c.keys.Key(kid) {
		if !key.IsPublic() || !key.Valid() || (key.Use != "" && key.Use != "sig") ||
			(key.Algorithm != "" && key.Algorithm != algorithm) {
			continue
		}
		switch public := key.Key.(type) {
		case *ecdsa.PublicKey:
			if algorithm != "ES256" || public.Curve != elliptic.P256() {
				continue
			}
		case *rsa.PublicKey:
			if algorithm != "RS256" || public.N.BitLen() < 2048 {
				continue
			}
		default:
			continue
		}
		if candidate != nil {
			return nil // Ambiguous key ID.
		}
		candidate = key.Key
	}
	return candidate
}

func fetchJWKS(ctx context.Context, endpoint string) (jose.JSONWebKeySet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return jose.JSONWebKeySet{}, err
	}
	resp, err := jwksHTTP.Do(req)
	if err != nil {
		return jose.JSONWebKeySet{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return jose.JSONWebKeySet{}, fmt.Errorf("JWKS returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, jwksMaxBytes+1))
	if err != nil {
		return jose.JSONWebKeySet{}, err
	}
	if len(body) > jwksMaxBytes {
		return jose.JSONWebKeySet{}, fmt.Errorf("JWKS response exceeds size limit")
	}
	var keys jose.JSONWebKeySet
	err = json.Unmarshal(body, &keys)
	return keys, err
}
