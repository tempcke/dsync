package test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync/internal/signaler"
)

// SigSpy will spy on a signaler, keeping track of all messages
// this means that if you leave it on, it will eat up a lot of memory
// therefore this should ONLY really be used from tests
type SigSpy struct {
	ctx      context.Context
	sig      *signaler.Signaler
	mu       sync.RWMutex
	messages []signaler.Message
}

func NewSigSpy(ctx context.Context) *SigSpy {
	s := &SigSpy{
		ctx: ctx,
		sig: signaler.Get(ctx),
	}
	s.listen(ctx)
	return s
}
func (s *SigSpy) listen(ctx context.Context) {
	s.sig.Listen(ctx, func(m signaler.Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.messages = append(s.messages, m)
	})
}

func (s *SigSpy) Seen(msg string, kvArgs ...string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.messages {
		if m.Body == msg {
			var argsMatch = true
			for k, v := range signaler.Args2Meta(kvArgs...) {
				if v2, ok := m.Meta[k]; !ok || v2 != v {
					argsMatch = false
					break
				}
			}
			if argsMatch {
				return true
			}
		}
	}
	return false
}
func (s *SigSpy) Print() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.messages {
		fmt.Println(m)
	}
}
func (s *SigSpy) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.messages))
	for i, m := range s.messages {
		out[i] = m.Body
	}
	return out
}
func (s *SigSpy) SeenEventually(t testing.TB, msg string, kvArgs ...string) {
	t.Helper()
	var (
		wait = time.Second * 20
		tick = time.Second / 10
	)
	require.Eventually(t, func() bool {
		return s.Seen(msg, kvArgs...)
	}, wait, tick, "signal not seen: %s %v", msg, kvArgs)
}
func (s *SigSpy) NotSeen(t testing.TB, msg string, kvArgs ...string) {
	t.Helper()
	require.False(t, s.Seen(msg, kvArgs...), "signal seen: %s", msg)
}
func (s *SigSpy) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}
func (s *SigSpy) ListenAndPrint(ctx context.Context, t testing.TB) {
	var (
		mu      sync.RWMutex
		entries []string
	)
	s.sig.Listen(ctx, func(m signaler.Message) {
		c := clock(ctx).Now().UTC().Format(time.TimeOnly)
		entry := fmt.Sprintf("[%s] SIG: %s\t%v", c, m.Body, m.Meta)
		// t.Log(entry)
		mu.Lock()
		defer mu.Unlock()
		entries = append(entries, entry)
	})
	t.Cleanup(func() {
		if t.Failed() {
			mu.RLock()
			defer mu.RUnlock()
			t.Log("signals\n" + strings.Join(entries, "\n"))
		}
	})
}
func clock(ctx context.Context) clockwork.Clock {
	return clockwork.FromContext(ctx)
}
