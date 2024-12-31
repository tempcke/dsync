package drivers

import (
	"context"
	"sync"

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
		locks     map[string]*mockLock
		elections []*mockElection
	}
)

var _md = &mockDriver{
	locks:     make(map[string]*mockLock),
	elections: make([]*mockElection, 0),
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
	name = dsync.SanitizeName(name)
	return dsync.NewScopedResource(d.ns, d.scope, name)
}
func (d *MockDriver) GetLeader(task string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getLeader(task)
}
func (d *MockDriver) getLeader(task string) string {
	r := d.Resource(task)
	for _, e := range d.elections {
		if e.resource.Equal(r) {
			return e.GetLeader()
		}
	}
	return ""
}

func (d *MockDriver) GetLock(ctx context.Context, name, pod string) (dsync.LockDriver, error) {
	r := d.Resource(name)
	lock := d.getLockByResource(r)
	if lock == nil {
		lock = &mockLock{ctx: ctx, r: r}
		d.mu.Lock()
		defer d.mu.Unlock()
		id := uuid.NewString()
		if d.locks == nil {
			d.locks = make(map[string]*mockLock)
		}
		d.locks[id] = lock
	}
	return &mockPodLock{
		lock: lock,
		pod:  pod,
	}, nil
}
func (d *MockDriver) GetElection(ctx context.Context, task, pod string) (dsync.ElectionDriver, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := mockElection{
		ctx:       ctx,
		resource:  d.Resource(task),
		candidate: pod,
		leader:    d.getLeader(task),
	}
	if e.leader == pod || e.leader == "" {
		e.elect(pod)
	}
	d.elections = append(d.elections, &e)
	return &e, nil
}

func (d *MockDriver) ForceLeader(r dsync.Resource, pod string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range d.elections {
		if e.resource.Equal(r) {
			e.elect(pod)
		}
	}
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
