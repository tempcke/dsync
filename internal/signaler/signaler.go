package signaler

import (
	"context"
	"fmt"
	"sync"
)

type (
	Signaler struct {
		ctx       context.Context
		cancel    context.CancelFunc
		mu        sync.RWMutex
		listeners []CallBack
	}
	Transmitter interface {
		Send(msg string, kvArgs ...string)
	}
	Receiver interface {
		Listen(context.Context, CallBack)
	}
	Message struct {
		Body string
		Meta Metadata
	}
	Metadata = map[string]string
	CallBack = func(Message)
	ctxT     string
)

const ctxKey ctxT = "signaler"

func New(ctx context.Context) *Signaler {
	ctx, cancel := context.WithCancel(ctx)
	sig := Signaler{
		ctx:    ctx,
		cancel: cancel,
	}
	return &sig
}
func (s *Signaler) Send(msg string, kvArgs ...string) {
	if !s.isReady() {
		return
	}

	m := Message{
		Body: msg,
		Meta: Args2Meta(kvArgs...),
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.listeners {
		if f != nil {
			f(m)
		}
	}
}
func (s *Signaler) Listen(ctx context.Context, callback CallBack) {
	if !s.isReady() {
		return
	}
	n := len(s.listeners)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, callback)

	go func() {
		cancel := func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.listeners[n] = nil
		}
		select {
		case <-s.ctx.Done():
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()
}
func (s *Signaler) Close() error {
	if s.isReady() {
		s.cancel()
	}
	return nil
}
func (s *Signaler) isReady() bool {
	return s != nil && s.ctx != nil && s.ctx.Err() == nil
}

func (m Message) String() string {
	return fmt.Sprintf("[SIG]: %s %v", m.Body, m.Meta)
}

func Args2Meta(kvArgs ...string) Metadata {
	meta := make(map[string]string, len(kvArgs)/2)
	for i := 0; i < len(kvArgs); i += 2 {
		meta[kvArgs[i]] = kvArgs[i+1]
	}
	// empty is ok, but never nil please to avoid read panics!
	return meta
}

func Context(ctx context.Context) context.Context {
	if v := ctx.Value(ctxKey); v != nil {
		return ctx // noop, already a sig context
	}
	var sig = New(ctx)
	return context.WithValue(ctx, ctxKey, sig)
}
func Listen(ctx context.Context, callback CallBack)          { Get(ctx).Listen(ctx, callback) }
func Send(ctx context.Context, msg string, kvArgs ...string) { Get(ctx).Send(msg, kvArgs...) }
func Close(ctx context.Context) error {
	return Get(ctx).Close()
}
func Get(ctx context.Context) *Signaler {
	if ctx != nil {
		if v := ctx.Value(ctxKey); v != nil {
			return v.(*Signaler)
		}
	}
	return nil
}
