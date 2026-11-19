package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSeparateEndpointsAndSignedHeaders(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/uploads/photos/山景.jpg" ||
			!strings.Contains(r.Header.Get("Authorization"), "Credential=example/") {
			t.Errorf("unexpected HEAD request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Length", "42")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", `"test-etag"`)
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
	}))
	defer internal.Close()
	client, err := New(Config{
		Endpoint: internal.URL, PublicEndpoint: "https://storage.example.test:8443",
		Region: "us-east-1", AccessKeyID: "example", SecretKey: "secret", UsePathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	put, err := client.PresignPut(ctx, "uploads", "photos/山景.jpg", PutOptions{
		ContentType: "image/jpeg", IfAbsent: true,
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(put.URL)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	if u.Host != "storage.example.test:8443" || u.Path != "/uploads/photos/山景.jpg" ||
		put.Method != "PUT" || query.Get("X-Amz-SignedHeaders") != "content-type;host;if-none-match" ||
		len(put.Headers) != 2 || put.Headers.Get("Content-Type") != "image/jpeg" || put.Headers.Get("If-None-Match") != "*" {
		t.Fatalf("unexpected signed PUT: %+v", put)
	}
	issued, err := time.Parse("20060102T150405Z", query.Get("X-Amz-Date"))
	if err != nil || !put.ExpiresAt.Equal(issued.Add(time.Minute)) {
		t.Fatalf("unexpected expiration: %v", put.ExpiresAt)
	}

	get, err := client.PresignGet(ctx, "uploads", "photos/山景.jpg", time.Second)
	if err != nil || get.Method != "GET" || len(get.Headers) != 0 || !strings.HasPrefix(get.URL, "https://storage.example.test:8443/") {
		t.Fatalf("unexpected signed GET: %+v, %v", get, err)
	}

	metadata, err := client.HeadObject(ctx, "uploads", "photos/山景.jpg")
	if err != nil || metadata.Size != 42 || metadata.ETag != `"test-etag"` ||
		metadata.ContentType != "image/jpeg" || metadata.LastModified.Year() != 2006 {
		t.Fatalf("unexpected metadata: %+v, %v", metadata, err)
	}
}

func TestR2SigningConfiguration(t *testing.T) {
	client, err := New(Config{
		Endpoint: "https://account.r2.cloudflarestorage.com", Region: "auto",
		AccessKeyID: "example", SecretKey: "secret", SessionToken: "example-session",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := client.PresignGet(context.Background(), "uploads", "file.jpg", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(request.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "uploads.account.r2.cloudflarestorage.com" || u.Path != "/file.jpg" ||
		!strings.Contains(u.Query().Get("X-Amz-Credential"), "/auto/s3/aws4_request") ||
		u.Query().Get("X-Amz-Security-Token") != "example-session" {
		t.Fatalf("unexpected R2 signature: %s", u)
	}
}

func TestInvalidLifetimeAndMissingObject(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client, err := New(Config{
		Endpoint: server.URL, Region: "us-east-1",
		AccessKeyID: "example", SecretKey: "secret", UsePathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, ttl := range []time.Duration{0, time.Second + time.Nanosecond, 8 * 24 * time.Hour} {
		if _, err := client.PresignGet(context.Background(), "uploads", "unknown", ttl); err == nil {
			t.Errorf("accepted invalid lifetime %v", ttl)
		}
	}
	if _, err := client.HeadObject(context.Background(), "uploads", "unknown"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}
}
