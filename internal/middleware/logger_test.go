package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"
)

type fakeLogRepository struct {
	entries []domain.RequestLog
}

func (f *fakeLogRepository) Create(ctx context.Context, entry domain.RequestLog) (int64, error) {
	f.entries = append(f.entries, entry)
	return int64(len(f.entries)), nil
}

func (f *fakeLogRepository) List(ctx context.Context, filter repository.RequestLogFilter) ([]domain.RequestLog, error) {
	return f.entries, nil
}

func TestLifecycleLoggerCompleted(t *testing.T) {
	logs := &fakeLogRepository{}
	handler := RequestID(LifecycleLogger(logs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})))

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", stringsReader(`{"service":"marketplace"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if len(logs.entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs.entries))
	}
	if logs.entries[0].Lifecycle != "STARTED" || logs.entries[1].Lifecycle != "COMPLETED" {
		t.Fatalf("unexpected lifecycles: %+v", logs.entries)
	}
	if logs.entries[1].StatusCode == nil || *logs.entries[1].StatusCode != http.StatusCreated {
		t.Fatalf("expected final status 201, got %+v", logs.entries[1].StatusCode)
	}
	if logs.entries[0].RequestHash == nil || len(*logs.entries[0].RequestHash) != 64 {
		t.Fatalf("expected request hash, got %+v", logs.entries[0].RequestHash)
	}
	if logs.entries[1].ResponseHash == nil || len(*logs.entries[1].ResponseHash) != 64 {
		t.Fatalf("expected response hash, got %+v", logs.entries[1].ResponseHash)
	}
}

func TestLifecycleLoggerFailed(t *testing.T) {
	logs := &fakeLogRepository{}
	handler := RequestID(LifecycleLogger(logs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if len(logs.entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs.entries))
	}
	if logs.entries[1].Lifecycle != "FAILED" {
		t.Fatalf("expected FAILED lifecycle, got %s", logs.entries[1].Lifecycle)
	}
	if logs.entries[1].ErrorMessage == nil {
		t.Fatal("expected error message")
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
