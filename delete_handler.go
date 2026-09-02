package main

import (
	"errors"
	"net/http"
)

func handleDeletePaste(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !isValidID(id) {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}

		if err := s.Delete(id); err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// isValidID reports whether id is a well-formed paste identifier. IDs are
// generated with crypto/rand and encoded URL-safe, so the accepted alphabet is
// alphanumerics plus '-' and '_'.
func isValidID(id string) bool {
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
