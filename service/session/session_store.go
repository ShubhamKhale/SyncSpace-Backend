// Package session manages per-user AES-256 session keys in memory.
// Keys are never persisted to the database; they live only for the
// duration of the server process.
package session

import (
	"crypto/rand"
	"fmt"
	"sync"
)

// Store is a thread-safe in-memory map of userID → 32-byte AES key.
type Store struct {
	mu   sync.RWMutex
	keys map[string][]byte
}

// NewStore creates an empty Store ready for use.
func NewStore() *Store {
	return &Store{keys: make(map[string][]byte)}
}

// GenerateKey creates a new random 32-byte key for the given userID,
// stores it, and returns it. Any existing key for the user is replaced.
func (s *Store) GenerateKey(userID string) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("session: key generation failed: %w", err)
	}
	s.mu.Lock()
	s.keys[userID] = key
	s.mu.Unlock()
	return key, nil
}

// GetKey returns the session key for userID and true, or nil and false if
// no key exists (user not logged in, or key expired/evicted).
func (s *Store) GetKey(userID string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[userID]
	return k, ok
}

// DeleteKey removes the session key for userID (e.g. on logout).
func (s *Store) DeleteKey(userID string) {
	s.mu.Lock()
	delete(s.keys, userID)
	s.mu.Unlock()
}
