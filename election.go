package dsync

import (
	"context"
	"fmt"
)

type (
	Election struct {
		driver    ElectionDriver
		candidate string // Dsync.instance
		err       error
		resource  Resource
	}
	ElectionDriver interface {
		GetLeader() string
		IsLeader() bool
		WhenElected(f func(context.Context))
		Stop()
	}
)

func NewElection(ctx context.Context, d Driver, task, pod string) Election {
	task = SanitizeName(task)
	e, err := d.GetElection(ctx, task, pod)
	return Election{
		driver:    e,
		resource:  d.Resource(task),
		candidate: pod,
		err:       err,
	}
}
func (e Election) GetLeader() string {
	if e.Err() != nil {
		return ""
	}
	return e.driver.GetLeader()
}
func (e Election) IsLeader() bool {
	if e.Err() != nil {
		return false
	}
	return e.driver.IsLeader()
}
func (e Election) WhenElected(f func(context.Context)) {
	if e.Err() != nil {
		return
	}
	e.driver.WhenElected(f)
}
func (e Election) Stop() {
	if e.Err() != nil {
		return
	}
	e.driver.Stop()
}
func (e Election) Err() error {
	if err := e.err; err != nil {
		return err
	}
	if err := e.resource.Validate(); err != nil {
		return err
	}
	if e.driver == nil {
		return fmt.Errorf("%w: Election.driver is nil", ErrInvalidState)
	}
	return nil
}
func (e Election) Resource() Resource { return e.resource }

type Term struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewTerm(ctx context.Context) Term {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	return Term{ctx: ctx, cancel: cancel}
}
func (t Term) Context() context.Context {
	if t.ctx == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx // if no ctx, then term is not active, so use canceled context
	}
	return t.ctx
}
func (t Term) Active() bool          { return t.Err() == nil }
func (t Term) Err() error            { return t.Context().Err() }
func (t Term) Done() <-chan struct{} { return t.Context().Done() }
func (t Term) Close() {
	if t.cancel != nil {
		t.cancel()
	}
}
