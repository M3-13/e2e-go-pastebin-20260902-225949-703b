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
	return Paste{}, ErrNotFound
}

func (s *Store) List() []Paste {
	return nil
}

func (s *Store) Delete(id string) error {
	return ErrNotFound
}

func (s *Store) Close() {}
