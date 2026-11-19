package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/towardsyou/kratos-contrib/auth/supabase"
	v1 "github.com/towardsyou/kratos-contrib/examples/upload/api/upload/v1"
	"github.com/towardsyou/kratos-contrib/examples/upload/internal/upload"
	"github.com/towardsyou/kratos-contrib/storage/s3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed web
var web embed.FS

type currentUserKey struct{}

func authenticateUploads(verifier *supabase.JWTVerifier) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/upload" || strings.HasPrefix(r.URL.Path, "/v1/upload/") {
				info, err := verifier.VerifyBearer(r.Context(), r.Header.Get("Authorization"))
				if err != nil {
					khttp.DefaultErrorEncoder(w, r, err)
					return
				}
				ctx := context.WithValue(r.Context(), currentUserKey{}, upload.User{ID: info.UserID})
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func currentUser(ctx context.Context) (upload.User, error) {
	user, ok := ctx.Value(currentUserKey{}).(upload.User)
	if !ok || user.ID == uuid.Nil {
		return upload.User{}, apierrors.Unauthorized("UNAUTHORIZED", "valid user required")
	}
	return user, nil
}

func newHTTPServer(address string, service *upload.Service, verifier *supabase.JWTVerifier, demoLogin bool, auth supabase.JWTConfig) (*khttp.Server, error) {
	server := khttp.NewServer(khttp.Address(address), khttp.Timeout(10*time.Second),
		khttp.Filter(authenticateUploads(verifier)))
	v1.RegisterUploadServiceHTTPServer(server, &uploadHTTP{service: service})
	server.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if demoLogin {
		server.Handle("/demo/session", demoSession(auth))
	}
	files, err := fs.Sub(web, "web")
	if err != nil {
		return nil, err
	}
	server.HandlePrefix("/", http.FileServer(http.FS(files)))
	return server, nil
}

const demoUserID = "ebc19610-0f33-4ef2-bf35-60abe0c5c08e"

func demoSession(cfg supabase.JWTConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		now := time.Now()
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Audience:  jwt.ClaimStrings{cfg.Audience},
			Subject:   demoUserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}).SignedString([]byte(cfg.Secret))
		if err != nil {
			http.Error(w, "could not create test session", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Token string `json:"token"`
		}{Token: token})
	})
}

type uploadHTTP struct {
	service *upload.Service
}

func (h *uploadHTTP) Presign(ctx context.Context, req *v1.PresignRequest) (*v1.PresignResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	key, signed, err := h.service.Presign(ctx, user, req.GetFilename(), req.GetContentType(), req.GetSize())
	if err != nil {
		return nil, err
	}
	return &v1.PresignResponse{Key: key, Request: signedResponse(signed)}, nil
}

func (h *uploadHTTP) Complete(ctx context.Context, req *v1.CompleteRequest) (*v1.CompleteResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	record, err := h.service.Complete(ctx, user, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &v1.CompleteResponse{Upload: uploadResponse(record)}, nil
}

func (h *uploadHTTP) List(ctx context.Context, _ *v1.ListRequest) (*v1.ListResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	records, err := h.service.List(ctx, user)
	if err != nil {
		return nil, err
	}
	reply := &v1.ListResponse{Uploads: make([]*v1.Upload, 0, len(records))}
	for _, record := range records {
		reply.Uploads = append(reply.Uploads, uploadResponse(record))
	}
	return reply, nil
}

func (h *uploadHTTP) Download(ctx context.Context, req *v1.DownloadRequest) (*v1.DownloadResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	signed, err := h.service.Download(ctx, user, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &v1.DownloadResponse{Request: signedResponse(signed)}, nil
}

func signedResponse(signed s3.SignedRequest) *v1.SignedRequest {
	headers := make(map[string]*v1.HeaderValues, len(signed.Headers))
	for key, values := range signed.Headers {
		headers[key] = &v1.HeaderValues{Values: values}
	}
	return &v1.SignedRequest{
		Url: signed.URL, Method: signed.Method, Headers: headers,
		ExpiresAt: timestamppb.New(signed.ExpiresAt),
	}
}

func uploadResponse(record upload.Record) *v1.Upload {
	return &v1.Upload{Key: record.Key, Filename: record.Filename, Size: record.Size, Status: record.Status}
}
