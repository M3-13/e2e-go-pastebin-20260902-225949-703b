package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s := NewStore()
	defer s.Close()

	addr := ":" + port
	log.Printf("pastebin listening on %s", addr)
	if err := http.ListenAndServe(addr, newMux(s)); err != nil {
		log.Fatal(err)
	}
}

func newMux(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pastes", handleCreatePaste(s))
	mux.HandleFunc("GET /pastes", handleListPastes(s))
	mux.HandleFunc("GET /pastes/{id}", handleGetPaste(s))
	mux.HandleFunc("DELETE /pastes/{id}", handleDeletePaste(s))
	mux.HandleFunc("GET /health", handleHealth)
	return withRecovery(mux)
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
