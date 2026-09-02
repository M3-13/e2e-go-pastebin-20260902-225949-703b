package main

import (
	"errors"
	"net/http"
	"sort"
	"time"
)

// pasteMeta is the wire representation of a paste's metadata: everything the
// paste carries except its content.
type pasteMeta struct {
	ID        string    `json:"id"`
	Language  string    `json:"language"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// validID reports whether id is a well-formed paste identifier. IDs are
// generated with crypto/rand and encoded URL-safe, so the accepted alphabet is
// alphanumerics plus '-' and '_'.
func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}

func handleGetPaste(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		p, err := s.Get(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, p)
	}
}

func handleListPastes(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pastes := s.List()

		now := time.Now()
		active := make([]Paste, 0, len(pastes))
		for _, p := range pastes {
			if p.ExpiresAt.After(now) {
				active = append(active, p)
			}
		}

		sort.Slice(active, func(i, j int) bool {
			return active[i].CreatedAt.Before(active[j].CreatedAt)
		})

		meta := make([]pasteMeta, 0, len(active))
		for _, p := range active {
			meta = append(meta, pasteMeta{
				ID:        p.ID,
				Language:  p.Language,
				ExpiresAt: p.ExpiresAt,
				CreatedAt: p.CreatedAt,
			})
		}

		writeJSON(w, http.StatusOK, meta)
	}
}
