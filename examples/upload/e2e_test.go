//go:build e2e

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	v1 "github.com/towardsyou/kratos-contrib/examples/upload/api/upload/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestPrivateUpload(t *testing.T) {
	origin := os.Getenv("UPLOAD_BASE_URL")
	if origin == "" {
		t.Fatal("start Compose using sh scripts/with-compose.sh")
	}
	fixture, err := os.ReadFile("fixtures/山景.jpg")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := t.Context()
	session := expectHTTP(t, client, ctx, http.MethodPost, origin+"/demo/session", nil, nil, http.StatusOK)
	// Read the demo session token through the HTTP API before using protected routes.
	token := struct{ Token string }{}
	if err := json.Unmarshal(session, &token); err != nil || token.Token == "" {
		t.Fatalf("decode demo session: %v", err)
	}
	auth := http.Header{"Authorization": {"Bearer " + token.Token}, "Content-Type": {"application/json"}}

	var signed v1.PresignResponse
	decodeProto(t, expectHTTP(t, client, ctx, http.MethodPost, origin+"/v1/upload/presign", auth,
		[]byte(fmt.Sprintf(`{"filename":"山景.jpg","contentType":"image/jpeg","size":%d}`, len(fixture))), http.StatusOK), &signed)
	put := signed.GetRequest()
	if put.GetMethod() != "PUT" || requestHeaders(put).Get("Content-Type") != "image/jpeg" ||
		requestHeaders(put).Get("If-None-Match") != "*" || !put.GetExpiresAt().AsTime().After(time.Now()) {
		t.Fatalf("invalid signed PUT: %v", put)
	}
	completeURL := origin + "/v1/upload/complete"
	completeBody := []byte(fmt.Sprintf(`{"key":%q}`, signed.Key))
	expectHTTP(t, client, ctx, http.MethodPost, completeURL, auth, completeBody, http.StatusConflict)
	expectHTTP(t, client, ctx, put.Method, put.Url, requestHeaders(put), fixture, http.StatusOK)

	const confirmations = 8
	done := make(chan httpResult, confirmations)
	start := make(chan struct{})
	for range confirmations {
		go func() {
			<-start
			done <- requestHTTP(client, ctx, http.MethodPost, completeURL, auth, completeBody)
		}()
	}
	close(start)
	var first v1.CompleteResponse
	for i := range confirmations {
		result := <-done
		requireStatus(t, result, http.StatusOK)
		var response v1.CompleteResponse
		decodeProto(t, result.body, &response)
		if response.GetUpload().GetStatus() != "verified" || response.GetUpload().GetSize() != int64(len(fixture)) {
			t.Fatalf("invalid confirmation: %v", &response)
		}
		if i == 0 {
			proto.Merge(&first, &response)
		} else if !proto.Equal(&first, &response) {
			t.Fatalf("confirmation differs: got %v, want %v", &response, &first)
		}
	}
	var repeated v1.CompleteResponse
	decodeProto(t, expectHTTP(t, client, ctx, http.MethodPost, completeURL, auth, completeBody, http.StatusOK), &repeated)
	if !proto.Equal(&first, &repeated) {
		t.Fatalf("repeated confirmation differs: got %v, want %v", &repeated, &first)
	}
	t.Logf("%d concurrent confirmations and the repeated confirmation returned the same verified record", confirmations)

	expectHTTP(t, client, ctx, put.Method, put.Url, requestHeaders(put), []byte("overwrite"), http.StatusPreconditionFailed)
	var list v1.ListResponse
	decodeProto(t, expectHTTP(t, client, ctx, http.MethodGet, origin+"/v1/upload", auth, nil, http.StatusOK), &list)
	if len(list.GetUploads()) == 0 || list.GetUploads()[0].GetKey() != signed.Key || list.GetUploads()[0].GetFilename() != "山景.jpg" {
		t.Fatalf("verified file missing from list: %v", &list)
	}

	downloadURL := origin + "/v1/upload/" + url.PathEscape(signed.Key) + "/download"
	var download v1.DownloadResponse
	decodeProto(t, expectHTTP(t, client, ctx, http.MethodGet, downloadURL, auth, nil, http.StatusOK), &download)
	get := download.GetRequest()
	if get.GetMethod() != "GET" {
		t.Fatalf("invalid signed GET: %v", get)
	}
	anonymous, err := url.Parse(get.Url)
	if err != nil {
		t.Fatal(err)
	}
	anonymous.RawQuery = ""
	expectHTTP(t, client, ctx, http.MethodGet, anonymous.String(), nil, nil, http.StatusForbidden)
	t.Log("anonymous object GET returned HTTP 403")
	downloaded := expectHTTP(t, client, ctx, get.Method, get.Url, requestHeaders(get), nil, http.StatusOK)
	wantHash := sha256.Sum256(fixture)
	if got := sha256.Sum256(downloaded); got != wantHash {
		t.Fatalf("SHA-256 mismatch: got %x, want %x", got, wantHash)
	}
	t.Logf("signed download SHA-256 matched the fixture: %x", wantHash)

	for _, raw := range []string{"", createTestToken(t, origin, "not-a-uuid"), createTestToken(t, origin, "00000000-0000-0000-0000-000000000000")} {
		headers := http.Header{"Content-Type": {"application/json"}}
		if raw != "" {
			headers.Set("Authorization", "Bearer "+raw)
		}
		expectHTTP(t, client, ctx, http.MethodPost, origin+"/v1/upload/presign", headers, []byte("{"), http.StatusUnauthorized)
		expectHTTP(t, client, ctx, http.MethodPost, completeURL, headers, completeBody, http.StatusUnauthorized)
		expectHTTP(t, client, ctx, http.MethodGet, origin+"/v1/upload", headers, nil, http.StatusUnauthorized)
		expectHTTP(t, client, ctx, http.MethodGet, downloadURL, headers, nil, http.StatusUnauthorized)
	}
	other := http.Header{"Content-Type": {"application/json"}, "Authorization": {"Bearer " + createTestToken(t, origin, "d4506580-ee1d-4d1b-baad-9e8d25c5cefd")}}
	expectHTTP(t, client, ctx, http.MethodPost, completeURL, other, completeBody, http.StatusNotFound)
	expectHTTP(t, client, ctx, http.MethodGet, downloadURL, other, nil, http.StatusNotFound)
}

func createTestToken(t *testing.T, origin, sub string) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: origin + "/demo", Subject: sub, Audience: jwt.ClaimStrings{"authenticated"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}).SignedString([]byte("local-demo-signing-secret-for-upload-example"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decodeProto(t *testing.T, body []byte, msg proto.Message) {
	t.Helper()
	if err := protojson.Unmarshal(body, msg); err != nil {
		t.Fatalf("decode %T: %v: %s", msg, err, body)
	}
}

func requestHeaders(signed *v1.SignedRequest) http.Header {
	headers := make(http.Header, len(signed.Headers))
	for name, values := range signed.Headers {
		headers[name] = values.GetValues()
	}
	return headers
}

type httpResult struct {
	code int
	body []byte
	err  error
}

func requestHTTP(client *http.Client, ctx context.Context, method, endpoint string, headers http.Header, body []byte) httpResult {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return httpResult{err: err}
	}
	req.Header = headers.Clone()
	response, err := client.Do(req)
	if err != nil {
		return httpResult{err: err}
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	return httpResult{code: response.StatusCode, body: data, err: err}
}

func expectHTTP(t *testing.T, client *http.Client, ctx context.Context, method, endpoint string, headers http.Header, body []byte, want int) []byte {
	t.Helper()
	result := requestHTTP(client, ctx, method, endpoint, headers, body)
	if result.err != nil || result.code != want {
		t.Logf("request: %s %s", method, strings.SplitN(endpoint, "?", 2)[0])
	}
	requireStatus(t, result, want)
	return result.body
}

func requireStatus(t *testing.T, result httpResult, want int) {
	t.Helper()
	if result.err != nil || result.code != want {
		t.Fatalf("want HTTP %d, got %d: %s (%v)", want, result.code, result.body, result.err)
	}
}
