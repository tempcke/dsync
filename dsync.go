package dsync

import (
	"context"
	"log/slog"

	"github.com/tempcke/dsync/configs"
)

type (
	PodID        = string
	LockID       = string
	ElectionID   = string
	ResourceName = string
	Dsync        struct {
		driver   Driver
		instance string // recommend you use pod name
	}
	Driver interface {
		Resource(string) Resource

		RunElection(context.Context, ResourceName, PodID) (ElectionID, error)
		GetLeader(ResourceName) string
		IsLeader(ElectionID) bool
		WhenElected(ElectionID, func(context.Context))
		StopElection(ElectionID)

		NewLock(context.Context, ResourceName, PodID) (LockID, error)
		Lock(LockID) error
		LockContext(ctx context.Context, id string) error
		TryLock(LockID) error
		Unlock(LockID) error
	}
	Interface interface {
		Election(context.Context, string) Election
		NewLock(context.Context, string) Lock
	}
)

const (
	DefaultNamespace = configs.DefNamespace
	DefaultScope     = configs.DefLeaseScope
	MasterTask       = "master"
)

func New(driver Driver, instance string) *Dsync {
	d := Dsync{
		driver:   driver,
		instance: instance,
	}
	return &d
}
func (d Dsync) Resource(name string) Resource {
	return d.driver.Resource(name)
}
func (d Dsync) Election(ctx context.Context, task string) Election {
	id, err := d.driver.RunElection(ctx, task, d.instance)
	if err != nil {
		// TODO: consider using an injected logger, or logging only if one is injected
		slog.Error("Dsync.Election error", "error", err.Error())
		return Election{err: err}
	}
	e := Election{
		driver:    d.driver,
		task:      task,
		candidate: d.instance,
		id:        id,
	}
	return e
}
func (d Dsync) NewLock(ctx context.Context, name string) Lock {
	return NewLock(ctx, d.driver, name, d.instance)
}
