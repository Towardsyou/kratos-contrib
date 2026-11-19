package upload

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	apierrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/google/uuid"
	"github.com/towardsyou/kratos-contrib/storage/s3"
)

type fakeObjects struct {
	head func(context.Context) (s3.Metadata, error)
}

func (*fakeObjects) PresignPut(context.Context, string, string, s3.PutOptions, time.Duration) (s3.SignedRequest, error) {
	return s3.SignedRequest{Method: http.MethodPut}, nil
}

func (*fakeObjects) PresignGet(context.Context, string, string, time.Duration) (s3.SignedRequest, error) {
	return s3.SignedRequest{Method: http.MethodGet}, nil
}

func (f *fakeObjects) HeadObject(ctx context.Context, _, _ string) (s3.Metadata, error) {
	return f.head(ctx)
}

func TestConfirmRetryAndOwnership(t *testing.T) {
	objects := &fakeObjects{head: func(context.Context) (s3.Metadata, error) {
		return s3.Metadata{}, errors.New("temporary I/O error")
	}}
	service, owner, key := testUpload(t, objects, 12)
	ctx := t.Context()

	other := User{ID: uuid.New()}
	_, err := service.Complete(ctx, other, key)
	assertCode(t, err, http.StatusNotFound)
	_, err = service.Download(ctx, other, key)
	assertCode(t, err, http.StatusNotFound)
	_, err = service.Complete(ctx, owner, key)
	assertCode(t, err, http.StatusServiceUnavailable)
	_, pending, err := service.find(ctx, owner, key)
	if err != nil || pending.Status != "pending" {
		t.Fatalf("temporary failure should leave pending: %+v, %v", pending, err)
	}

	objects.head = func(context.Context) (s3.Metadata, error) {
		return s3.Metadata{Size: 12, ETag: "original"}, nil
	}
	first, err := service.Complete(ctx, owner, key)
	if err != nil || first.Status != "verified" || first.Size != 12 {
		t.Fatalf("retry confirmation: %+v, %v", first, err)
	}
	objects.head = func(context.Context) (s3.Metadata, error) {
		t.Error("already verified objects should use the saved result")
		return s3.Metadata{Size: 13}, nil
	}
	next, err := service.Complete(ctx, owner, key)
	if next != first || err != nil {
		t.Fatalf("repeated confirmation changed: %+v, %v", next, err)
	}
}

func TestConcurrentConfirmKeepsPersistedResult(t *testing.T) {
	calls := make(chan chan s3.Metadata, 2)
	objects := &fakeObjects{head: func(ctx context.Context) (s3.Metadata, error) {
		send := make(chan s3.Metadata, 1)
		calls <- send
		select {
		case data := <-send:
			return data, nil
		case <-ctx.Done():
			return s3.Metadata{}, ctx.Err()
		}
	}}
	service, owner, key := testUpload(t, objects, 12)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	type result struct {
		record Record
		err    error
	}
	out := make(chan result, 2)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait()
	}()
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			data, err := service.Complete(ctx, owner, key)
			out <- result{data, err}
		}()
	}
	var pending []chan s3.Metadata
	for range 2 {
		select {
		case call := <-calls:
			pending = append(pending, call)
		case <-ctx.Done():
			t.Fatal("confirmations did not both reach metadata lookup")
		}
	}
	pending[0] <- s3.Metadata{Size: 12, ETag: "first"}
	first := <-out
	if first.err != nil || first.record.Status != "verified" {
		t.Fatalf("first confirmation: %+v", first)
	}
	pending[1] <- s3.Metadata{Size: 13, ETag: "second"}
	second := <-out
	if second != first {
		t.Fatalf("concurrent confirmation changed the result: first=%+v second=%+v", first, second)
	}
}

func TestConfirmRejectsWrongSize(t *testing.T) {
	objects := &fakeObjects{head: func(context.Context) (s3.Metadata, error) {
		return s3.Metadata{Size: 12}, nil
	}}
	service, owner, key := testUpload(t, objects, 11)
	ctx := t.Context()
	for range 2 {
		_, err := service.Complete(ctx, owner, key)
		assertCode(t, err, http.StatusUnprocessableEntity)
	}
	_, record, err := service.find(ctx, owner, key)
	if err != nil || record.Status != "rejected" || record.Size != 12 {
		t.Fatalf("want persisted rejection: %+v, %v", record, err)
	}
	records, err := service.List(ctx, owner)
	if err != nil || len(records) != 0 {
		t.Fatalf("rejected upload found in list: %+v, %v", records, err)
	}
	_, err = service.Download(ctx, owner, key)
	assertCode(t, err, http.StatusConflict)
}

func testUpload(t *testing.T, objects ObjectStore, size int64) (*Service, User, string) {
	t.Helper()
	store, err := OpenStore(t.Context(), filepath.Join(t.TempDir(), "uploads.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := NewService(store, objects, "uploads")
	owner := User{ID: uuid.New()}
	key, _, err := service.Presign(t.Context(), owner, "山景.jpg", "image/jpeg", size)
	if err != nil {
		t.Fatal(err)
	}
	return service, owner, key
}

func assertCode(t *testing.T, err error, want int) {
	t.Helper()
	if code := apierrors.Code(err); code != want {
		t.Fatalf("want HTTP %d, got %d (%v)", want, code, err)
	}
}
