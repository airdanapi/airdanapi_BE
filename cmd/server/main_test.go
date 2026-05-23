package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"airdanapi-be/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(config.Config{
		Env:     "test",
		Port:    "8080",
		Name:    "airdanapi-integrator",
		Version: "0.1.0",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestReadyEndpoint(t *testing.T) {
	router := NewRouter(config.Config{
		Env:     "test",
		Port:    "8080",
		Name:    "airdanapi-integrator",
		Version: "0.1.0",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
