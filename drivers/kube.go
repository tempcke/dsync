package drivers

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/internal/k8s"
)

type (
	KubeDriver struct {
		k         k8s.Kube
		ns        string
		scope     string
		mu        sync.RWMutex
		elections map[string]*k8s.Election
		locks     map[string]*k8s.Lock
	}
)

func NewKubeDriver(conf configs.Config) (*KubeDriver, error) {
	k, err := k8s.New()
	if err != nil {
		return nil, err
	}
	return NewKubeWithK(conf, *k), nil
}
func NewKubeWithK(conf configs.Config, k k8s.Kube) *KubeDriver {
	conf = configs.WithDefaults(conf, configs.Defaults)
	return &KubeDriver{
		k:         k,
		ns:        conf(configs.KeyNamespace),
		scope:     conf(configs.KeyLeaseScope),
		elections: make(map[string]*k8s.Election),
		locks:     make(map[string]*k8s.Lock),
	}
}
func (d *KubeDriver) GetLeader(task string) string {
	out, err := d.k.GetLeader(context.TODO(), d.Resource(task))
	if err != nil {
		return ""
	}
	return out
}
func (d *KubeDriver) Resource(name string) dsync.Resource {
	return d.resource(dsync.NewResource(name))
}
func (d *KubeDriver) RunElection(ctx context.Context, task, candidate string) (string, error) {
	r := d.Resource(task)

	e, err := d.k.NewElection(ctx, r, candidate)
	if err != nil {
		return "", err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	id := uuid.NewString()
	if d.elections == nil {
		d.elections = make(map[string]*k8s.Election)
	}
	d.elections[id] = e
	return id, nil
}
func (d *KubeDriver) StopElection(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.elections != nil {
		if e, ok := d.elections[id]; ok {
			e.Stop()
			delete(d.elections, id)
		}
	}
}
func (d *KubeDriver) IsLeader(electionID string) bool {
	if e := d.getElection(electionID); e != nil {
		return e.IsLeader()
	}
	return false
}
func (d *KubeDriver) WhenElected(electionID string, f func(context.Context)) {
	if e := d.getElection(electionID); e != nil {
		e.WhenElected(f)
	}
}

func (d *KubeDriver) NewLock(ctx context.Context, name, pod string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.Resource(name)
	if d.locks == nil {
		d.locks = make(map[string]*k8s.Lock)
	}
	id := uuid.NewString()
	lock, err := d.k.GetLock(ctx, r, pod)
	if err != nil {
		return "", err
	}
	d.locks[id] = lock
	return id, nil
}
func (d *KubeDriver) Lock(id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	return lock.Lock()
}
func (d *KubeDriver) LockContext(ctx context.Context, id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	return lock.LockContext(ctx)
}
func (d *KubeDriver) TryLock(id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	return lock.TryLock()
}
func (d *KubeDriver) Unlock(id string) error {
	lock, err := d.getLock(id)
	if err != nil {
		return err
	}
	return lock.Unlock()
}

func (d *KubeDriver) resource(r dsync.Resource) dsync.Resource {
	if r.Namespace == "" {
		r.Namespace = d.ns
	}
	if r.Scope == "" {
		r.Scope = d.scope
	}
	return r
}
func (d *KubeDriver) getElection(id string) *k8s.Election {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if e, ok := d.elections[id]; ok {
		return e
	}
	return nil
}

func (d *KubeDriver) getLock(id string) (*k8s.Lock, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if lock, ok := d.locks[id]; ok {
		return lock, nil
	}
	return nil, dsync.ErrLockNotFound
}
