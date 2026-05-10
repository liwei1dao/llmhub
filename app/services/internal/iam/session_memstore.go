package iam

import (
	"sync"
	"time"
)

// MemSessionIndex is a process-local session lookup used by the
// account server until the Redis-backed index lands in M4.
//
// Not safe for multi-process deployments — only useful in dev where
// everything runs in one process.
type MemSessionIndex struct {
	mu   sync.RWMutex
	data map[string]memEntry
}

type memEntry struct {
	userID    int64
	expiresAt time.Time
}

// NewMemSessionIndex returns an empty in-memory index.
func NewMemSessionIndex() *MemSessionIndex {
	return &MemSessionIndex{data: make(map[string]memEntry)}
}

// Put stores a token → user binding with a TTL.
func (m *MemSessionIndex) Put(token string, userID int64, expiresAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[token] = memEntry{userID: userID, expiresAt: expiresAt}
}

// Get returns the user id for a token, or 0 if unknown / expired.
func (m *MemSessionIndex) Get(token string) int64 {
	m.mu.RLock()
	e, ok := m.data[token]
	m.mu.RUnlock()
	if !ok {
		return 0
	}
	if time.Now().After(e.expiresAt) {
		m.mu.Lock()
		delete(m.data, token)
		m.mu.Unlock()
		return 0
	}
	return e.userID
}

// Delete revokes a token.
func (m *MemSessionIndex) Delete(token string) {
	m.mu.Lock()
	delete(m.data, token)
	m.mu.Unlock()
}
