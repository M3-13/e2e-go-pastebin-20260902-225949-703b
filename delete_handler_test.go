package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeletePaste(t *testing.T) {
	s := NewStore()
	now := time.Now()
	s.pastes["abc123"] = Paste{
		ID:        "abc123",
		Content:   "hello world",
		Language:  "text",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodDelete, "/pastes/abc123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/pastes/abc123", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: expected status 404, got %d", rec.Code)
	}
}

func TestDeletePasteUnknownID(t *testing.T) {
	s := NewStore()

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodDelete, "/pastes/doesnotexist", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body, got error: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("expected non-empty error message, got %v", body)
	}
}

func TestDeletePasteExpired(t *testing.T) {
	s := NewStore()
	s.pastes["expired"] = Paste{
		ID:        "expired",
		Content:   "gone",
		Language:  "text",
		ExpiresAt: time.Now().Add(-time.Minute),
		CreatedAt: time.Now().Add(-time.Hour),
	}

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodDelete, "/pastes/expired", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestDeletePasteInvalidID(t *testing.T) {
	s := NewStore()
	h := handleDeletePaste(s)

	for _, id := range []string{"bad!", "bad@chars", "bad~chars"} {
		req := httptest.NewRequest(http.MethodDelete, "/pastes/"+id, nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id %q: expected status 400, got %d", id, rec.Code)
		}
	}
}

func TestDeletePasteEmptyID(t *testing.T) {
	s := NewStore()
	h := handleDeletePaste(s)

	req := httptest.NewRequest(http.MethodDelete, "/pastes/", nil)
	req.SetPathValue("id", "")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
