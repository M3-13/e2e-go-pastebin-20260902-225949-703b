package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const maxBodyBytes = 1 << 20 // 1 MiB

const defaultTTL = 24 * time.Hour

type createPasteRequest struct {
	Content          string `json:"content"`
	Language         string `json:"language"`
	ExpiresInSeconds *int64 `json:"expires_in_seconds"`
}

func resolveTTL(expiresInSeconds *int64) time.Duration {
	if expiresInSeconds == nil {
		return defaultTTL
	}
	return time.Duration(*expiresInSeconds) * time.Second
}

func handleCreatePaste(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

		var req createPasteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if req.Content == "" {
			writeError(w, http.StatusBadRequest, "content must not be empty")
			return
		}

		if req.ExpiresInSeconds != nil && *req.ExpiresInSeconds <= 0 {
			writeError(w, http.StatusBadRequest, "expires_in_seconds must be a positive integer")
			return
		}

		paste, err := s.Create(req.Content, req.Language, resolveTTL(req.ExpiresInSeconds))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusCreated, paste)
	}
}
