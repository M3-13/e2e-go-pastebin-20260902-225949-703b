package main

import (
	"sync"
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p, err := s.Create("hello", "text", time.Hour)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if p.Content != "hello" {
		t.Fatalf("content = %q, want %q", p.Content, "hello")
	}
	if p.Language != "text" {
		t.Fatalf("language = %q, want %q", p.Language, "text")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("CreatedAt not set")
	}
	if !p.ExpiresAt.After(p.CreatedAt) {
		t.Fatal("ExpiresAt must be after CreatedAt")
	}

	got, err := s.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "hello" || got.ID != p.ID {
		t.Fatalf("got %+v, want id %q content %q", got, p.ID, "hello")
	}
}

func TestGetNotFound(t *testing.T) {
	s := NewStore()
	defer s.Close()

	if _, err := s.Get("does-not-exist"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListSortedByCreatedAt(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p1, _ := s.Create("a", "", time.Hour)
	time.Sleep(time.Millisecond)
	p2, _ := s.Create("b", "", time.Hour)
	time.Sleep(time.Millisecond)
	p3, _ := s.Create("c", "", time.Hour)

	ps := s.List()
	if len(ps) != 3 {
		t.Fatalf("List len = %d, want 3", len(ps))
	}

	want := []string{p1.ID, p2.ID, p3.ID}
	for i := range ps {
		if ps[i].ID != want[i] {
			t.Fatalf("List order wrong at %d: got %q, want %q", i, ps[i].ID, want[i])
		}
	}
	for i := 1; i < len(ps); i++ {
		if ps[i].CreatedAt.Before(ps[i-1].CreatedAt) {
			t.Fatal("List not ascending by CreatedAt")
		}
	}
}

func TestExpiry(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p, err := s.Create("secret", "", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := s.Get(p.ID); err != nil {
		t.Fatalf("expected readable before expiry, got %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	if _, err := s.Get(p.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after expiry, got %v", err)
	}

	s.mu.Lock()
	_, ok := s.pastes[p.ID]
	s.mu.Unlock()
	if ok {
		t.Fatal("expired paste still present in map after Get")
	}
}

func TestListRemovesExpired(t *testing.T) {
	s := NewStore()
	defer s.Close()

	expired, _ := s.Create("x", "", 10*time.Millisecond)
	active, _ := s.Create("y", "", time.Hour)

	time.Sleep(20 * time.Millisecond)

	ps := s.List()
	if len(ps) != 1 {
		t.Fatalf("List len = %d, want 1", len(ps))
	}
	if ps[0].ID != active.ID {
		t.Fatalf("List returned %q, want %q", ps[0].ID, active.ID)
	}
	if ps[0].ID == expired.ID {
		t.Fatal("expired paste still listed")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p, _ := s.Create("x", "", time.Hour)

	if err := s.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(p.ID); err != ErrNotFound {
		t.Fatalf("second Delete = %v, want ErrNotFound", err)
	}
	if err := s.Delete("missing"); err != ErrNotFound {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}

func TestDeleteExpired(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p, _ := s.Create("x", "", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	if err := s.Delete(p.ID); err != ErrNotFound {
		t.Fatalf("Delete expired = %v, want ErrNotFound", err)
	}
}

func TestIDUniqueness(t *testing.T) {
	s := NewStore()
	defer s.Close()

	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		p, err := s.Create("x", "", time.Hour)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[p.ID] {
			t.Fatal("duplicate id generated")
		}
		seen[p.ID] = true
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := newID()
		if err != nil {
			t.Fatalf("newID: %v", err)
		}
		if id == "" {
			t.Fatal("empty id")
		}
		if seen[id] {
			t.Fatal("duplicate id from newID")
		}
		seen[id] = true
	}
}

func TestPurgeExpiredLocked(t *testing.T) {
	s := NewStore()
	defer s.Close()

	active, _ := s.Create("a", "", time.Hour)
	expired, _ := s.Create("b", "", time.Hour)

	s.mu.Lock()
	p := s.pastes[expired.ID]
	p.ExpiresAt = time.Now().Add(-time.Hour)
	s.pastes[expired.ID] = p
	s.purgeExpiredLocked(time.Now())
	s.mu.Unlock()

	s.mu.Lock()
	_, okActive := s.pastes[active.ID]
	_, okExpired := s.pastes[expired.ID]
	s.mu.Unlock()

	if !okActive {
		t.Fatal("active paste was purged")
	}
	if okExpired {
		t.Fatal("expired paste was not purged")
	}
}

func TestExpiryRemovesPromptly(t *testing.T) {
	s := NewStore()
	defer s.Close()

	p, _ := s.Create("secret", "", 50*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		_, ok := s.pastes[p.ID]
		s.mu.Unlock()
		if !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("expired paste still in map after ttl elapsed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStore()
	defer s.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				p, _ := s.Create("x", "", time.Hour)
				_, _ = s.Get(p.ID)
				_ = s.List()
				_ = s.Delete(p.ID)
			}
		}()
	}
	wg.Wait()
}
