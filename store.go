package main

import (
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu     sync.Mutex
	pastes map[string]Paste
}

func NewStore() *Store {
	return &Store{
		pastes: make(map[string]Paste),
	}
}

func (s *Store) Create(content, language string, ttl time.Duration) (Paste, error) {
	return Paste{}, nil
}

func (s *Store) Get(id string) (Paste, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pastes[id]
	if !ok {
		return Paste{}, ErrNotFound
	}
	if !p.ExpiresAt.After(time.Now()) {
		delete(s.pastes, id)
		return Paste{}, ErrNotFound
	}
	return p, nil
}

func (s *Store) List() []Paste {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make([]Paste, 0, len(s.pastes))
	for id, p := range s.pastes {
		if p.ExpiresAt.After(now) {
			out = append(out, p)
		} else {
			delete(s.pastes, id)
		}
	}
	return out
}

func (s *Store) Delete(id string) error {
	return ErrNotFound
}

func (s *Store) Close() {}
