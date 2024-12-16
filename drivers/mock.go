package drivers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
)

type (
	MockDriver struct {
		*mockDriver
		ns    string
		scope string
	}
	mockDriver struct {
		mu        sync.RWMutex
		elections map[string]*mockElection
		locks     map[string]*mockLock
	}
	mockElection struct {
		id         string
		pod        string
		resource   dsync.Resource
		ctx        context.Context
		termCtx    context.Context
		termCancel context.CancelFunc
		isLeader   atomic.Bool
		onStartFns []func(context.Context)
		mu         sync.RWMutex
	}
	mockLock struct {
		mu     sync.Mutex
		locked bool
		ctx    context.Context
		r      dsync.Resource
		owner  string
	}
)

var _md = &mockDriver{
	elections: make(map[string]*mockElection),
	locks:     make(map[string]*mockLock),
}

func NewMockDriver(conf configs.Config) *MockDriver {
	conf = configs.WithDefaults(conf, configs.Defaults)
	return &MockDriver{
		mockDriver: _md,
		ns:         conf(configs.KeyNamespace),
		scope:      conf(configs.KeyLeaseScope),
	}
}
func (d *MockDriver) Resource(name string) dsync.Resource {
	return dsync.NewScopedResource(d.ns, d.scope, name)
}

func (d *MockDriver) GetLeader(task string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r := d.Resource(task)
	for _, e := range d.elections {
		if e.resource.Equal(r) && e.IsLeader() {
			return e.pod
		}
	}
	return ""
}
func (d *MockDriver) RunElection(ctx context.Context, task, candidate string) (string, error) {
	r := d.Resource(task)
	e := d.addElection(ctx, r, candidate)
	if d.GetLeader(task) == candidate {
		e.elect()
	} else {
		d.runElection(r)
	}
	return e.id, nil
}
func (d *MockDriver) StopElection(id string) {
	r := d.getElection(id).resource
	d.delElection(id)
	d.runElection(r)
}
func (d *MockDriver) IsLeader(eID string) bool {
	if e := d.getElection(eID); e != nil {
		return e.IsLeader()
	}
	return false
}
func (d *MockDriver) WhenElected(eID string, f func(context.Context)) {
	if e := d.getElection(eID); e != nil {
		e.whenElected(f)
	}
}

func (d *MockDriver) NewLock(ctx context.Context, name, _ string) (string, error) {
	r := d.Resource(name)
	lock := d.getLockByResource(r)
	if lock == nil {
		lock = &mockLock{ctx: ctx, r: r}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	id := uuid.NewString()
	if d.locks == nil {
		d.locks = make(map[string]*mockLock)
	}
	d.locks[id] = lock
	return id, nil
}
func (d *MockDriver) Lock(id string) error {
	return d.LockContext(context.Background(), id)
}
func (d *MockDriver) LockContext(ctx context.Context, id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	var wait time.Duration
	if ctx == nil {
		ctx = lock.ctx
	}
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
func (d *MockDriver) TryLock(id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	if ok := lock.tryLock(id); !ok {
		return dsync.ErrAlreadyLocked
	}
	return nil
}
func (d *MockDriver) Unlock(id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	return lock.unlock(id)
}

func (d *MockDriver) ForceLeader(r dsync.Resource, pod string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var eIDs []string
	for _, e := range d.elections {
		if e.resource.Equal(r) {
			if !e.IsLeader() && e.pod == pod {
				eIDs = append(eIDs, e.id)
			}
			if e.IsLeader() && e.pod != pod {
				e.stop()
			}
		}
	}
	for _, id := range eIDs {
		d.elections[id].elect()
	}
}

func (d *MockDriver) addElection(ctx context.Context, r dsync.Resource, pod string) *mockElection {
	id := uuid.NewString()
	e := mockElection{
		id:         id,
		pod:        pod,
		resource:   r,
		ctx:        ctx,
		termCtx:    nil,
		termCancel: nil,
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.elections[id] = &e
	return &e
}
func (d *MockDriver) delElection(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.elections[id]; ok {
		e.stop()
		delete(d.elections, id)
	}
}
func (d *MockDriver) runElection(r dsync.Resource) {
	if d.GetLeader(r.Name) != "" {
		return
	}
	pod := func() string {
		d.mu.RLock()
		defer d.mu.RUnlock()
		for _, e := range d.elections {
			if r.Equal(e.resource) {
				return e.pod
			}
		}
		return ""
	}()
	if pod != "" {
		d.ForceLeader(r, pod)
	}
}
func (d *MockDriver) getElection(id string) *mockElection {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if e, ok := d.elections[id]; ok {
		return e
	}
	return nil
}
func (d *MockDriver) getLock(id string) (*mockLock, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if lock, ok := d.locks[id]; ok {
		return lock, nil
	}
	return nil, dsync.ErrLockNotFound
}
func (d *MockDriver) getLockByResource(r dsync.Resource) *mockLock {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, lock := range d.locks {
		if lock.r.Equal(r) {
			return lock
		}
	}
	return nil
}

func (e *mockElection) whenElected(f func(context.Context)) {
	if e == nil {
		return
	}
	if e.IsLeader() {
		go f(e.termCtx)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStartFns = append(e.onStartFns, f)
}
func (e *mockElection) IsLeader() bool {
	return e != nil && e.isLeader.Load()
}
func (e *mockElection) elect() {
	if e == nil || e.isLeader.Load() {
		return
	}
	ctx, cancel := context.WithCancel(e.ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.termCtx = ctx
	e.termCancel = cancel
	e.isLeader.Store(true)
	for _, f := range e.onStartFns {
		go f(e.termCtx)
	}
}
func (e *mockElection) stop() {
	if e == nil {
		return
	}
	e.isLeader.Store(false)
	if e.termCancel != nil {
		e.termCancel()
	}
}

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
