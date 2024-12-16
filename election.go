package dsync

import (
	"context"
)

type Election struct {
	driver    Driver
	task      string
	candidate string // Dsync.instance
	id        string
	err       error
}

func (e Election) GetLeader() string {
	if e.Err() != nil {
		return ""
	}
	return e.driver.GetLeader(e.task)
}
func (e Election) IsLeader() bool {
	if e.Err() != nil {
		return false
	}
	return e.driver.IsLeader(e.id)
}
func (e Election) WhenElected(f func(context.Context)) {
	if e.Err() != nil {
		return
	}
	e.driver.WhenElected(e.id, f)
}
func (e Election) Stop() {
	if e.Err() != nil {
		return
	}
	e.driver.StopElection(e.id)
}
func (e Election) Err() error {
	if err := e.err; err != nil {
		return err
	}
	if err := e.driver.Resource(e.task).Validate(); err != nil {
		return err
	}
	return nil
}
func (e Election) Resource() Resource {
	return e.driver.Resource(e.task)
}

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
