package syncdetcd

import (
	"context"
	"sync"
)

// EtcdManager implements Manager with in-memory sync.RWMutexes. Each token
// has it's own RWMutex
type EtcdManager struct {
	rw   sync.RWMutex
	acqu map[string]*sync.RWMutex
}

// NewEtcdManager initializes a new EtcdManager which implements the Manager
// interface.
func NewEtcdManager() *EtcdManager {
	return &EtcdManager{
		acqu: make(map[string]*sync.RWMutex),
	}
}

func (m *EtcdManager) setTokenMutex(token string, mutex *sync.RWMutex) bool {
	// Acquire lock for map writing
	m.rw.Lock()
	// release write lock for map
	defer m.rw.Unlock()

	// check if lock was set in meantime
	if _, ok := m.acqu[token]; ok {
		// if so return false
		return false
	}

	// noone set a mutex for our token yet -> set it
	m.acqu[token] = mutex

	return true
}

// AllocateMutexes allocates mutexes for all passed tokens. This reduces
// mutex contention, because read-locks are cheap and share parallel
// access.
func (m *EtcdManager) AllocateMutexes(tokens ...string) {
	m.rw.Lock()
	defer m.rw.Unlock()

	for _, t := range tokens {
		var mutex sync.RWMutex
		m.acqu[t] = &mutex
	}
}

// NewRequest returns a new Request based on the InMemManager
func (m *EtcdManager) NewRequest() *Request {
	return NewRequest(m)
}

// Lock locks the write-lock for the token if not already locked.
func (m *EtcdManager) Lock(ctx context.Context, token string) error {
	m.rw.RLock()
	rw, ok := m.acqu[token]
	m.rw.RUnlock()

	if ok {
		rw.Lock()
		return nil
	}

	var mutex sync.RWMutex
	mutex.Lock()

	if !m.setTokenMutex(token, &mutex) {
		// set operation failed, because the token now exists. Recursively call
		// ourself again. Next time we should end in OK and try to acquire the
		// correct mutex's lock.
		return m.Lock(ctx, token)
	}

	return nil
}

// RLock locks the read-lock for the token if not already locked.
func (m *EtcdManager) RLock(ctx context.Context, token string) error {
	m.rw.RLock()
	rw, ok := m.acqu[token]
	m.rw.RUnlock()

	if ok {
		rw.RLock()
		return nil
	}

	var mutex sync.RWMutex
	mutex.RLock()

	if !m.setTokenMutex(token, &mutex) {
		// set operation failed, because the token now exists. Recursively call
		// ourself again. Next time we should end in OK and try to acquire the
		// correct mutex's lock.
		return m.RLock(ctx, token)
	}

	return nil
}

// Unlock unlocks the write-lock for the token if not already unlocked.
func (m *EtcdManager) Unlock(ctx context.Context, token string) error {
	m.rw.RLock()
	rw, ok := m.acqu[token]
	m.rw.RUnlock()

	if ok {
		rw.Unlock()
	}

	return nil
}

// RUnlock unlocks the read-lock for the token if not already unlocked.
func (m *EtcdManager) RUnlock(ctx context.Context, token string) error {
	m.rw.RLock()
	rw, ok := m.acqu[token]
	m.rw.RUnlock()

	if ok {
		rw.RUnlock()
	}

	return nil
}
