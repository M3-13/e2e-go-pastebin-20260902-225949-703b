package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPasteExisting(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/pastes/abc123", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body Paste
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body, got error: %v", err)
	}
	if body.ID != "abc123" {
		t.Fatalf("expected id abc123, got %q", body.ID)
	}
	if body.Content != "hello world" {
		t.Fatalf("expected content hello world, got %q", body.Content)
	}
}

func TestGetPasteUnknownID(t *testing.T) {
	s := NewStore()

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodGet, "/pastes/doesnotexist", nil)
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

func TestGetPasteExpired(t *testing.T) {
	s := NewStore()
	s.pastes["expired"] = Paste{
		ID:        "expired",
		Content:   "gone",
		Language:  "text",
		ExpiresAt: time.Now().Add(-time.Minute),
		CreatedAt: time.Now().Add(-time.Hour),
	}

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodGet, "/pastes/expired", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetPasteInvalidID(t *testing.T) {
	s := NewStore()
	h := handleGetPaste(s)

	for _, id := range []string{"bad!", "bad@chars", "bad~chars"} {
		req := httptest.NewRequest(http.MethodGet, "/pastes/"+id, nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id %q: expected status 400, got %d", id, rec.Code)
		}
	}
}

func TestGetPasteEmptyID(t *testing.T) {
	s := NewStore()
	h := handleGetPaste(s)

	req := httptest.NewRequest(http.MethodGet, "/pastes/", nil)
	req.SetPathValue("id", "")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestListPastesMetadata(t *testing.T) {
	s := NewStore()
	now := time.Now()
	s.pastes["a"] = Paste{
		ID:        "a",
		Content:   "secret-a",
		Language:  "text",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now.Add(10 * time.Second),
	}
	s.pastes["b"] = Paste{
		ID:        "b",
		Content:   "secret-b",
		Language:  "go",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}
	s.pastes["expired"] = Paste{
		ID:        "expired",
		Content:   "secret-expired",
		Language:  "text",
		ExpiresAt: now.Add(-time.Minute),
		CreatedAt: now.Add(-time.Hour),
	}

	mux := newMux(s)
	req := httptest.NewRequest(http.MethodGet, "/pastes", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}

	if len(body) != 2 {
		t.Fatalf("expected 2 active pastes, got %d: %v", len(body), body)
	}

	if body[0]["id"] != "b" || body[1]["id"] != "a" {
		t.Fatalf("expected ascending order [b a], got [%v %v]", body[0]["id"], body[1]["id"])
	}

	for _, item := range body {
		if _, ok := item["content"]; ok {
			t.Fatalf("content field must be absent, got %v", item)
		}
		if _, ok := item["expires_at"]; !ok {
			t.Fatalf("expected expires_at field, got %v", item)
		}
		if _, ok := item["created_at"]; !ok {
			t.Fatalf("expected created_at field, got %v", item)
		}
		if _, ok := item["language"]; !ok {
			t.Fatalf("expected language field, got %v", item)
		}
	}
}
