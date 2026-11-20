package upload

import (
	"context"
	"database/sql"
	"errors"
	"mime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	apierrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"github.com/towardsyou/kratos-contrib/storage/s3"
)

const (
	maxBytes  = 10 * 1024 * 1024
	signedTTL = 5 * time.Minute
)

// User is the application's current user.
type User struct {
	ID uuid.UUID
}

type Record struct {
	Key          string
	Filename     string
	ExpectedSize int64
	Status       string
	Size         int64
	ETag         string
}

type ObjectStore interface {
	PresignPut(context.Context, string, string, s3.PutOptions, time.Duration) (s3.SignedRequest, error)
	PresignGet(context.Context, string, string, time.Duration) (s3.SignedRequest, error)
	HeadObject(context.Context, string, string) (s3.Metadata, error)
}

type Service struct {
	records *Store
	objects ObjectStore
	bucket  string
}

func NewService(records *Store, objects ObjectStore, bucket string) *Service {
	return &Service{records: records, objects: objects, bucket: bucket}
}

func (s *Service) Presign(ctx context.Context, user User, filename, contentType string, size int64) (string, s3.SignedRequest, error) {
	if strings.TrimSpace(filename) == "" || !utf8.ValidString(filename) || len(filename) > 255 ||
		strings.ContainsAny(filename, "/\\") || strings.IndexFunc(filename, unicode.IsControl) >= 0 ||
		filename == "." || filename == ".." {
		return "", s3.SignedRequest{}, apierrors.BadRequest("INVALID_FILENAME", "invalid filename")
	}
	if size < 1 || size > maxBytes {
		return "", s3.SignedRequest{}, apierrors.BadRequest("INVALID_SIZE", "file size must be between 1 byte and 10 MiB")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return "", s3.SignedRequest{}, apierrors.BadRequest("INVALID_CONTENT_TYPE", "invalid content type")
	}

	key := uuid.NewString()
	request, err := s.objects.PresignPut(ctx, s.bucket, key, s3.PutOptions{
		ContentType: contentType,
		IfAbsent:    true,
	}, signedTTL)
	if err != nil {
		return "", s3.SignedRequest{}, storageUnavailable(err)
	}
	record := Record{Key: key, Filename: filename, ExpectedSize: size}
	if err := s.records.create(ctx, user, record); err != nil {
		return "", s3.SignedRequest{}, err
	}
	return key, request, nil
}

func (s *Service) Complete(ctx context.Context, user User, key string) (Record, error) {
	id, record, err := s.find(ctx, user, key)
	if err != nil {
		return Record{}, err
	}
	if record.Status != "pending" {
		return confirmationResult(record)
	}

	metadata, err := s.objects.HeadObject(ctx, s.bucket, record.Key)
	if errors.Is(err, s3.ErrObjectNotFound) {
		return Record{}, apierrors.Conflict("UPLOAD_PENDING", "object has not been uploaded")
	}
	if err != nil {
		return Record{}, storageUnavailable(err)
	}

	record.Status = "verified"
	record.Size, record.ETag = metadata.Size, metadata.ETag
	if metadata.Size < 1 || metadata.Size > maxBytes || metadata.Size != record.ExpectedSize {
		record.Status = "rejected"
	}
	saved, err := s.records.finish(ctx, user, id, record)
	if err != nil {
		return Record{}, err
	}
	return confirmationResult(saved)
}

func confirmationResult(record Record) (Record, error) {
	if record.Status == "rejected" {
		return Record{}, apierrors.New(422, "SIZE_MISMATCH", "object size does not match the requested upload size")
	}
	return record, nil
}

func (s *Service) List(ctx context.Context, user User) ([]Record, error) {
	return s.records.list(ctx, user)
}

func (s *Service) Download(ctx context.Context, user User, key string) (s3.SignedRequest, error) {
	_, record, err := s.find(ctx, user, key)
	if err != nil {
		return s3.SignedRequest{}, err
	}
	if record.Status != "verified" {
		return s3.SignedRequest{}, apierrors.Conflict("UPLOAD_UNVERIFIED", "upload is not verified")
	}
	request, err := s.objects.PresignGet(ctx, s.bucket, record.Key, signedTTL)
	if err != nil {
		return s3.SignedRequest{}, storageUnavailable(err)
	}
	return request, nil
}

func (s *Service) find(ctx context.Context, user User, key string) (uuid.UUID, Record, error) {
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, Record{}, apierrors.NotFound("UPLOAD_NOT_FOUND", "upload not found")
	}
	record, err := s.records.find(ctx, user, id)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, Record{}, apierrors.NotFound("UPLOAD_NOT_FOUND", "upload not found")
	}
	return id, record, err
}

func storageUnavailable(cause error) error {
	return apierrors.ServiceUnavailable("STORAGE_UNAVAILABLE", "object storage temporarily unavailable").WithCause(cause)
}
