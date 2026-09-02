package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthReturns200(t *testing.T) {
	mux := newMux(NewStore())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body, got error: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestWithRecoveryCatchesPanic(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	mux := withRecovery(panicHandler)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body, got error: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("expected error message, got %q", body["error"])
	}
}

func TestWriteErrorForm(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, "not found")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body, got error: %v", err)
	}

	if len(body) != 1 {
		t.Fatalf("expected exactly one field, got %d: %v", len(body), body)
	}
	if msg, ok := body["error"]; !ok {
		t.Fatalf("expected field error, got %v", body)
	} else if msg != "not found" {
		t.Fatalf("expected error message not found, got %q", msg)
	}
}
