package main

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

const reaperInterval = time.Minute

type Store struct {
	mu     sync.Mutex
	pastes map[string]Paste

	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func NewStore() *Store {
	s := &Store{
		pastes: make(map[string]Paste),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go s.reaper()
	return s
}

func (s *Store) reaper() {
	defer close(s.done)
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			s.purgeExpiredLocked(time.Now())
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}

// purgeExpiredLocked deletes every paste whose ExpiresAt is not after now.
// The caller must hold s.mu.
func (s *Store) purgeExpiredLocked(now time.Time) {
	for id, p := range s.pastes {
		if !p.ExpiresAt.After(now) {
			delete(s.pastes, id)
		}
	}
}

// scheduleExpiryLocked arranges for the paste to be removed from the map the
// moment its ttl elapses, so its content does not linger in memory past
// ExpiresAt (AC-13). The reaper stays as a safety net for timers that are
// lost. The caller must hold s.mu.
func (s *Store) scheduleExpiryLocked(id string, ttl time.Duration) {
	time.AfterFunc(ttl, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if p, ok := s.pastes[id]; ok && !p.ExpiresAt.After(time.Now()) {
			delete(s.pastes, id)
		}
	})
}

func (s *Store) Create(content, language string, ttl time.Duration) (Paste, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		id, err := newID()
		if err != nil {
			return Paste{}, err
		}
		if _, exists := s.pastes[id]; exists {
			continue
		}
		p := Paste{
			ID:        id,
			Content:   content,
			Language:  language,
			CreatedAt: now,
			ExpiresAt: now.Add(ttl),
		}
		s.pastes[id] = p
		s.scheduleExpiryLocked(id, ttl)
		return p, nil
	}
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

	s.purgeExpiredLocked(time.Now())

	ps := make([]Paste, 0, len(s.pastes))
	for _, p := range s.pastes {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].CreatedAt.Before(ps[j].CreatedAt)
	})
	return ps
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pastes[id]
	if !ok {
		return ErrNotFound
	}
	if !p.ExpiresAt.After(time.Now()) {
		delete(s.pastes, id)
		return ErrNotFound
	}
	delete(s.pastes, id)
	return nil
}

func (s *Store) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		<-s.done
	})
}
