package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tempcke/dsync"
)

type Election struct {
	r   dsync.Resource
	pod string
	el  *Elector

	ctx            context.Context
	cancel         context.CancelFunc
	term           dsync.Term
	mu             sync.RWMutex
	startTermFuncs []func(context.Context)
}

func NewElection(
	ctx context.Context,
	client LeaseClient,
	r dsync.Resource,
	candidate string,
	callbacks ...LeaderCallbacks,
) (*Election, error) {
	ctx, cancel := context.WithCancel(ctx)
	e := Election{
		ctx:    ctx,
		cancel: cancel,
		r:      r,
		pod:    candidate,
	}
	callbacks = e.leaderCallbacks(callbacks...)
	el, err := NewLeaderElector(ctx, client, r, candidate, callbacks...)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.el = el
	return &e, nil
}
func (e *Election) Resource() dsync.Resource { return e.r }
func (e *Election) Candidate() string        { return e.pod }
func (e *Election) Stop() {
	if err := e.Err(); err != nil {
		return
	}
	e.term.Close()
	if e.cancel != nil {
		e.cancel()
	}
}
func (e *Election) Err() error {
	if e == nil {
		return fmt.Errorf("nil Election")
	}
	if e.el == nil {
		return fmt.Errorf("nil Elector")
	}
	return e.ctx.Err()
}
func (e *Election) GetLeader() string {
	if err := e.Err(); err != nil {
		return ""
	}
	return e.el.GetLeader()
}
func (e *Election) WhenElected(fn func(context.Context)) {
	if err := e.Err(); err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.term.Active() {
		go fn(e.term.Context())
	}
	e.startTermFuncs = append(e.startTermFuncs, fn)
}
func (e *Election) IsLeader() bool {
	if err := e.Err(); err != nil {
		return false
	}
	return e.el.IsLeader()
}
func (e *Election) leaderCallbacks(callbacks ...LeaderCallbacks) []LeaderCallbacks {
	cb := LeaderCallbacks{
		OnStartedLeading: e.onStartedLeading,
		OnStoppedLeading: e.onStoppedLeading,
		OnNewLeader:      e.onNewLeader,
	}
	return append([]LeaderCallbacks{cb}, callbacks...)
}
func (e *Election) onStartedLeading(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.term.Close()
	e.term = dsync.NewTerm(ctx)
	e.logEvent("StartedLeading")
	for _, fn := range e.startTermFuncs {
		if fn != nil {
			go fn(e.term.Context())
		}
	}
}
func (e *Election) onStoppedLeading() {
	e.term.Close()
	e.logEvent("StoppedLeading")
}
func (e *Election) onNewLeader(identity string) {
	e.logEvent("NewLeader", "identity", identity)
}
func (e *Election) logEvent(event string, args ...any) {
	go func() {
		e.mu.RLock()
		defer e.mu.RUnlock()
		args = append(args, "fields", map[string]string{
			"resource": e.r.String(),
			"podID":    e.pod,
			"leaderID": e.el.GetLeader(),
		})
		slog.Info("k8sClient "+event, args...)
	}()
}
