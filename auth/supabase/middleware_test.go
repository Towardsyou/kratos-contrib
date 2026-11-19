package supabase

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testIssuer = "https://example.supabase.co/auth/v1"
const testAudience = "authenticated"
const testSecret = "local-test-signing-key"
const testUser = "ebc19610-0f33-4ef2-bf35-60abe0c5c08e"

func TestStrictHMAC(t *testing.T) {
	verify, err := NewJWTMiddleware(JWTConfig{Algorithm: "HS256", Issuer: testIssuer, Audience: testAudience, Secret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	server := protectedServer(verify)
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		change func(jwt.MapClaims)
		want   int
	}{
		{name: "valid", want: http.StatusOK},
		{name: "sub has priority", change: func(c jwt.MapClaims) { c["user_id"] = uuid.NewString() }, want: http.StatusOK},
		{name: "wrong algorithm", method: jwt.SigningMethodHS512, want: http.StatusUnauthorized},
		{name: "wrong issuer", change: func(c jwt.MapClaims) { c["iss"] = "https://another.example" }, want: http.StatusUnauthorized},
		{name: "wrong audience", change: func(c jwt.MapClaims) { c["aud"] = []string{"anon"} }, want: http.StatusUnauthorized},
		{name: "missing issuer", change: func(c jwt.MapClaims) { delete(c, "iss") }, want: http.StatusUnauthorized},
		{name: "missing audience", change: func(c jwt.MapClaims) { delete(c, "aud") }, want: http.StatusUnauthorized},
		{name: "expired", change: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, want: http.StatusUnauthorized},
		{name: "missing expiration", change: func(c jwt.MapClaims) { delete(c, "exp") }, want: http.StatusUnauthorized},
		{name: "invalid subject", change: func(c jwt.MapClaims) { c["sub"] = "invalid"; c["user_id"] = testUser }, want: http.StatusUnauthorized},
		{name: "nil subject", change: func(c jwt.MapClaims) { c["sub"] = uuid.Nil.String() }, want: http.StatusUnauthorized},
		{name: "missing subject", change: func(c jwt.MapClaims) { delete(c, "sub"); c["user_id"] = testUser }, want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := validClaims(testIssuer)
			if tc.change != nil {
				tc.change(claims)
			}
			method := tc.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			assertProtected(t, server, sign(t, method, "", []byte(testSecret), claims), tc.want)
		})
	}
	assertProtected(t, server, "", http.StatusUnauthorized)
}

func TestLegacyHMACCompatibility(t *testing.T) {
	verify := NewAuthMiddleware(Config{JWTSecret: testSecret})
	server := protectedServer(verify)
	claims := jwt.MapClaims{"user_id": testUser}
	assertProtected(t, server, sign(t, jwt.SigningMethodHS384, "", []byte(testSecret), claims), http.StatusOK)
	claims["user_id"] = "bad"
	assertProtected(t, server, sign(t, jwt.SigningMethodHS256, "", []byte(testSecret), claims), http.StatusUnauthorized)
	_, err := verify(func(context.Context, any) (any, error) {
		t.Fatal("protected handler reached without a transport")
		return nil, nil
	})(context.Background(), nil)
	if !kerrors.IsUnauthorized(err) {
		t.Fatalf("request without transport got %v", err)
	}
}

func TestAsymmetricJWKS(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sets := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &ec.PublicKey, KeyID: "ec", Algorithm: "ES256", Use: "sig"},
		{Key: &rsaKey.PublicKey, KeyID: "rsa", Algorithm: "RS256", Use: "sig"},
	}}
	var fetches atomic.Int32
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			t.Errorf("unexpected JWKS path %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(sets); err != nil {
			t.Error(err)
		}
	}))
	defer jwks.Close()
	issuer := jwks.URL + "/auth/v1"
	for _, tc := range []struct {
		kid    string
		method jwt.SigningMethod
		key    any
	}{
		{kid: "ec", method: jwt.SigningMethodES256, key: ec},
		{kid: "rsa", method: jwt.SigningMethodRS256, key: rsaKey},
	} {
		t.Run(tc.method.Alg(), func(t *testing.T) {
			verify, err := NewJWTMiddleware(JWTConfig{Algorithm: tc.method.Alg(), Issuer: issuer, Audience: testAudience})
			if err != nil {
				t.Fatal(err)
			}
			server := protectedServer(verify)
			start := fetches.Load()
			assertProtected(t, server, sign(t, tc.method, tc.kid, tc.key, validClaims(issuer)), http.StatusOK)
			assertProtected(t, server, sign(t, tc.method, "unknown", tc.key, validClaims(issuer)), http.StatusUnauthorized)
			claims := validClaims(issuer)
			claims["sub"] = "invalid"
			assertProtected(t, server, sign(t, tc.method, tc.kid, tc.key, claims), http.StatusUnauthorized)
			assertProtected(t, server, sign(t, jwt.SigningMethodHS256, tc.kid, []byte(testSecret), validClaims(issuer)), http.StatusUnauthorized)
			if fetches.Load() != start+1 {
				t.Errorf("expected one JWKS fetch, got %d", fetches.Load()-start)
			}
		})
	}
}

func protectedServer(verify middleware.Middleware) http.Handler {
	server := khttp.NewServer(khttp.Middleware(verify))
	server.Route("/").GET("/protected", func(ctx khttp.Context) error {
		handle := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
			info, ok := FromContext(ctx)
			if !ok {
				return nil, kerrors.Unauthorized("UNAUTHORIZED", "missing identity")
			}
			return info.UserID.String(), nil
		})
		result, err := handle(ctx, nil)
		if err != nil {
			return err
		}
		return ctx.Result(http.StatusOK, result)
	})
	return server
}

func assertProtected(t *testing.T, handler http.Handler, rawToken string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if rawToken != "" {
		req.Header.Set("Authorization", "Bearer "+rawToken)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != want || (want == http.StatusOK && !strings.Contains(response.Body.String(), testUser)) {
		t.Fatalf("want %d, got %d: %s", want, response.Code, response.Body.String())
	}
}

func validClaims(issuer string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": issuer, "aud": []string{testAudience}, "sub": testUser, "exp": time.Now().Add(time.Minute).Unix(),
	}
}

func sign(t *testing.T, method jwt.SigningMethod, kid string, key any, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
