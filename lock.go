package dsync

import (
	"context"
	"fmt"
)

type (
	Lock struct {
		ctx      context.Context
		d        Driver
		resource Resource
		id       string
		err      error
	}
)

func NewLock(ctx context.Context, d Driver, name, pod string) Lock {
	id, err := d.NewLock(ctx, name, pod)
	return Lock{
		ctx:      ctx,
		d:        d,
		resource: d.Resource(name),
		id:       id,
		err:      err,
	}
}

func (l Lock) Resource() Resource { return l.resource }
func (l Lock) Err() error {
	if err := l.err; err != nil {
		return err
	}
	if l.d == nil || l.id == "" {
		return fmt.Errorf("dsync.Lock %w", ErrInvalidState)
	}
	return nil
}
func (l Lock) Lock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.d.Lock(l.id)
}
func (l Lock) LockContext(ctx context.Context) error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.d.LockContext(ctx, l.id)
}
func (l Lock) TryLock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.d.TryLock(l.id)
}
func (l Lock) Unlock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.d.Unlock(l.id)
}
func (l Lock) DoWithLock(ctx context.Context, f func() error) error {
	if err := l.LockContext(ctx); err != nil {
		return err
	}
	defer func() { _ = l.Unlock() }()
	return f()
}
func (l Lock) DoWithTryLock(ctx context.Context, f func() error) error {
	if err := l.TryLock(); err != nil {
		return err
	}
	defer func() { _ = l.Unlock() }()
	return f()
}
