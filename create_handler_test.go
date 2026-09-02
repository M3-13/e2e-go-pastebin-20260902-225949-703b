package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func doCreate(t *testing.T, s *Store, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleCreatePaste(s)(rec, req)
	return rec
}

func TestCreatePasteReturns201(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"content":"hello","language":"text"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body, got error: %v", err)
	}
	for _, field := range []string{"id", "content", "language", "expires_at", "created_at"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected field %q in response, got %v", field, body)
		}
	}
}

func TestCreatePasteMissingContent(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"language":"text"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePasteEmptyContent(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"content":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePasteInvalidJSON(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"content":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePasteExpiresZero(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"content":"x","expires_in_seconds":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePasteExpiresNegative(t *testing.T) {
	s := NewStore()
	rec := doCreate(t, s, `{"content":"x","expires_in_seconds":-5}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePasteBodyTooLarge(t *testing.T) {
	s := NewStore()
	big := strings.Repeat("a", maxBodyBytes+1024)
	body := `{"content":"` + big + `"}`
	rec := doCreate(t, s, body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", rec.Code)
	}
}

func TestResolveTTLDefault(t *testing.T) {
	if got := resolveTTL(nil); got != defaultTTL {
		t.Fatalf("expected default ttl %v, got %v", defaultTTL, got)
	}
}

func TestResolveTTLExplicit(t *testing.T) {
	var secs int64 = 3600
	if got := resolveTTL(&secs); got != 3600*time.Second {
		t.Fatalf("expected ttl 3600s, got %v", got)
	}
}
