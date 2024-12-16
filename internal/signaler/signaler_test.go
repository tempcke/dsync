package signaler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync/internal/signaler"
)

var ctx = context.Background()

func TestSignaler(t *testing.T) {
	var (
		ctx, cancel = context.WithCancel(ctx)
	)
	t.Cleanup(cancel)
	t.Run("nil safe", func(t *testing.T) {
		// nil sig
		var sig *signaler.Signaler
		sig.Listen(ctx, func(signaler.Message) {})
		sig.Listen(ctx, nil)
		sig.Send("")
		require.NoError(t, sig.Close())

		// nil sig context
		sig = new(signaler.Signaler)
		sig.Listen(ctx, func(signaler.Message) {})
		sig.Listen(ctx, nil)
		sig.Send("")
		require.NoError(t, sig.Close())
	})
	t.Run("send and receive one message", func(t *testing.T) {
		var (
			sig      = signaler.New(ctx)
			mu       sync.Mutex
			messages []signaler.Message
			body     = uuid.NewString()
			k1, v1   = "k1", "v1"
			k2, v2   = "k2", "v2"
			args     = []string{k1, v1, k2, v2}
		)
		require.NotNil(t, sig)

		sig.Listen(ctx, func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})

		sig.Send(body, args...)
		require.Equal(t, 1, len(messages))
		assert.Equal(t, body, messages[0].Body)
		assert.Equal(t, v1, messages[0].Meta[k1])
		assert.Equal(t, v2, messages[0].Meta[k2])
	})
	t.Run("multiple listeners", func(t *testing.T) {
		var (
			sig      = signaler.New(ctx)
			body     = uuid.NewString()
			mu       sync.Mutex
			messages []signaler.Message
		)

		sig.Listen(ctx, func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})
		sig.Listen(ctx, func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})
		sig.Send(body)
		require.Equal(t, 2, len(messages))
	})
	t.Run("closed signal", func(t *testing.T) {
		var (
			ctx, cancel = context.WithCancel(ctx)
			sig         = signaler.New(ctx)
			body        = uuid.NewString()
			mu          sync.Mutex
			messages    []signaler.Message
		)
		t.Cleanup(cancel)
		sig.Listen(ctx, func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})
		sig.Send(body)
		require.NoError(t, sig.Close())
		sig.Send(body)
		assert.Equal(t, 1, len(messages))
	})
	t.Run("cancel sig ctx", func(t *testing.T) {
		var (
			ctx, cancel = context.WithCancel(ctx)
			sig         = signaler.New(ctx)
			body        = uuid.NewString()
			mu          sync.Mutex
			messages    []signaler.Message
		)
		t.Cleanup(cancel)
		sig.Listen(context.Background(), func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})
		sig.Send(body)
		cancel()
		sig.Send(body)
		assert.Equal(t, 1, len(messages))
	})
	t.Run("cancel listener ctx", func(t *testing.T) {
		var (
			ctx, cancel   = context.WithCancel(signaler.Context(ctx))
			ctx2, cancel2 = context.WithCancel(ctx)
			sig           = signaler.Get(ctx)
			body          = uuid.NewString()
			mu            sync.Mutex
			messages      []signaler.Message
		)
		t.Cleanup(cancel)
		sig.Listen(ctx2, func(m signaler.Message) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
		})
		sig.Send(body)
		cancel2()
		// cancel happens inside a go routine, so to prevent a sleep we just check it a few times
		assert.Eventually(t, func() bool {
			var n = len(messages)
			sig.Send(body)
			return n == len(messages)
		}, time.Second*5, time.Second/100)
	})
}
