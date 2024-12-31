package drivers

import (
	"context"
	"sync"
	"time"

	"github.com/tempcke/dsync"
)

type (
	mockPodLock struct {
		lock *mockLock
		pod  string
	}

	mockLock struct {
		mu     sync.Mutex
		locked bool
		ctx    context.Context
		r      dsync.Resource
		owner  string
	}
)

func (m *mockPodLock) Lock() error {
	return m.LockContext(context.Background())
}

func (m *mockPodLock) LockContext(ctx context.Context) error {
	var (
		id   = m.pod
		lock = m.lock
		wait time.Duration
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			wait = time.Second / 10
			if ok := lock.tryLock(id); ok {
				return nil // we have the lock
			}
		}
	}
}

func (m *mockPodLock) Unlock() error {
	return m.lock.unlock(m.pod)
}

func (m *mockPodLock) TryLock() error {
	if ok := m.lock.tryLock(m.pod); !ok {
		return dsync.ErrAlreadyLocked
	}
	return nil
}

var _ dsync.LockDriver = (*mockPodLock)(nil)

func (m *mockLock) tryLock(id string) bool {
	if m.mu.TryLock() {
		m.locked = true
		m.owner = id
		return true
	}
	return false
}
func (m *mockLock) unlock(id string) error {
	if !m.locked { // used to prevent panic from mu.Unlock when not locked
		return dsync.ErrNotLocked
	}
	if m.owner != id {
		return dsync.ErrNotLockHolder
	}
	m.locked = false
	m.mu.Unlock()
	return nil
}
