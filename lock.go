package dsync

import (
	"context"
	"fmt"
)

type (
	Lock struct {
		ld       LockDriver
		resource Resource
		err      error
	}
	LockDriver interface {
		Lock() error
		LockContext(ctx context.Context) error
		Unlock() error
		TryLock() error
	}
)

func NewLock(ctx context.Context, d Driver, name, pod string) Lock {
	lock, err := d.GetLock(ctx, name, pod)
	return Lock{
		ld:       lock,
		resource: d.Resource(name),
		err:      err,
	}
}

func (l Lock) Resource() Resource { return l.resource }
func (l Lock) Err() error {
	if err := l.err; err != nil {
		return err
	}
	if l.ld == nil {
		return fmt.Errorf("dsync.Lock %w", ErrInvalidState)
	}
	if err := l.resource.Validate(); err != nil {
		return err
	}
	return nil
}
func (l Lock) Lock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.ld.Lock()
}
func (l Lock) LockContext(ctx context.Context) error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.ld.LockContext(ctx)
}
func (l Lock) TryLock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.ld.TryLock()
}
func (l Lock) Unlock() error {
	if err := l.Err(); err != nil {
		return err
	}
	return l.ld.Unlock()
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
