package drivers

import (
	"context"
	"sync"

	"github.com/tempcke/dsync"
)

type mockElection struct {
	ctx        context.Context
	resource   dsync.Resource
	candidate  string
	leader     string
	termCtx    context.Context
	termCancel context.CancelFunc
	mu         sync.RWMutex
	electedFns []func(context.Context)
}

var _ dsync.ElectionDriver = (*mockElection)(nil)

func (e *mockElection) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.leader == e.candidate
}
func (e *mockElection) GetLeader() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.leader
}
func (e *mockElection) WhenElected(f func(context.Context)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.electedFns = append(e.electedFns, f)
	if e.leader == e.candidate {
		go f(e.termCtx)
	}
}
func (e *mockElection) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leader == e.candidate {
		e.termCancel()
		e.leader = ""
	}
}
func (e *mockElection) elect(pod string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.leader = pod
	if e.termCtx != nil && e.termCtx.Err() == nil && e.termCancel != nil {
		e.termCancel()
	}
	if e.leader == e.candidate {
		e.termCtx, e.termCancel = context.WithCancel(e.ctx)
		for _, f := range e.electedFns {
			go f(e.termCtx)
		}
	}
}
