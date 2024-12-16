package k8s_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/internal/k8s"
	"github.com/tempcke/dsync/internal/signaler"
	"github.com/tempcke/dsync/internal/test"
	"k8s.io/client-go/kubernetes/fake"
)

func TestElection_WhenElected(t *testing.T) {
	var (
		ns    = "local"
		scope = "testing"
		task  = uuid.NewString()[24:]
		r     = dsync.NewScopedResource(ns, scope, task)
		cs    = fake.NewClientset()
		podA  = "A"
		podB  = "B"

		sigWhenElectedStart = "sigWhenElectedStart"
		sigWhenElectedEnd   = "sigWhenElectedEnd"
	)

	t.Run("empty election should noop", func(t *testing.T) {
		var e k8s.Election
		assert.False(t, e.IsLeader())
		assert.Equal(t, "", e.GetLeader())
		e.Stop()
		e.WhenElected(nil)
	})
	t.Run("is leader", func(t *testing.T) {
		// here we have 2 pods volunteering to be leader
		// in the end one and only one must be leader, makes no difference which one
		var (
			ctx    = test.Context(t, time.Minute/4)
			task   = newTaskName()
			r      = dsync.NewScopedResource(ns, scope, task)
			sigSpy = test.NewSigSpy(ctx)
			cbA    = test.ElectorCallbacks(ctx, podA)
			cbB    = test.ElectorCallbacks(ctx, podB)
		)
		sigSpy.ListenAndPrint(ctx, t)

		// podA
		k, err := k8s.New(k8s.KubeWithCS(cs))
		require.NoError(t, err)
		eA, err := k.NewElection(ctx, r, podA, cbA)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podA, "id", podA)
		assert.True(t, eA.IsLeader())
		assert.Equal(t, podA, eA.GetLeader())
		elected, err := k.GetLeader(ctx, r)
		require.NoError(t, err)
		assert.Equal(t, podA, elected)

		// podB
		eB, err := k.NewElection(ctx, r, podB, cbB)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podB)
		assert.True(t, eA.IsLeader() || eB.IsLeader())
		assert.False(t, eA.IsLeader() && eB.IsLeader())
	})
	t.Run("when elected", func(t *testing.T) {
		var (
			ctx    = test.Context(t, time.Minute/4)
			sigSpy = test.NewSigSpy(ctx)
		)
		sigSpy.ListenAndPrint(ctx, t)

		k, err := k8s.New(k8s.KubeWithCS(cs))
		require.NoError(t, err)

		cbA := test.ElectorCallbacks(ctx, podA)
		e, err := k.NewElection(ctx, r, podA, cbA)
		require.NoError(t, err)

		// can it handle multiple WhenElected 's ?
		e.WhenElected(func(termCtx context.Context) {
			signaler.Send(termCtx, sigWhenElectedStart, "whenElected", "1")
			<-termCtx.Done()
			signaler.Send(termCtx, sigWhenElectedEnd, "whenElected", "1")
		})
		e.WhenElected(func(termCtx context.Context) {
			signaler.Send(termCtx, sigWhenElectedStart, "whenElected", "2")
			<-termCtx.Done()
			signaler.Send(termCtx, sigWhenElectedEnd, "whenElected", "2")
		})
		sigSpy.SeenEventually(t, sigWhenElectedStart, "whenElected", "1")
		sigSpy.SeenEventually(t, sigWhenElectedStart, "whenElected", "2")
		require.True(t, e.IsLeader())

		// check our own callbacks
		sigSpy.SeenEventually(t, test.SigStartedLeading, "pod", podA)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podA, "id", podA)

		e.Stop()
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "whenElected", "1")
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "whenElected", "2")
		sigSpy.SeenEventually(t, test.SigStoppedLeading, "pod", podA)
		assert.False(t, e.IsLeader())
	})
	t.Run("two elections same candidate", func(t *testing.T) {
		// in case someone creates 2 election objects for the same resource and same pod
		// we want to make sure their callbacks both fire when elected
		// and that they use the correct parent context
		var (
			ctx1    = test.Context(t, time.Minute/4)
			sigSpy1 = test.NewSigSpy(ctx1)

			ctx2    = test.Context(t, time.Minute/4)
			sigSpy2 = test.NewSigSpy(ctx2)
		)
		sigSpy1.ListenAndPrint(ctx1, t)
		sigSpy2.ListenAndPrint(ctx2, t)

		k, err := k8s.New(k8s.KubeWithCS(cs))
		require.NoError(t, err)

		e1, err := k.NewElection(ctx1, r, podA, test.ElectorCallbacks(ctx1, podA))
		require.NoError(t, err)
		e2, err := k.NewElection(ctx2, r, podA, test.ElectorCallbacks(ctx2, podA))
		require.NoError(t, err)

		// can it handle multiple WhenElected 's ?
		e1.WhenElected(func(ctx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "e", "1")
			<-ctx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "e", "1")
		})
		e2.WhenElected(func(ctx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "e", "2")
			<-ctx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "e", "2")
		})
		sigSpy1.SeenEventually(t, sigWhenElectedStart, "e", "1")
		sigSpy2.SeenEventually(t, sigWhenElectedStart, "e", "2")

		assert.True(t, e1.IsLeader())
		assert.True(t, e2.IsLeader())

		e1.Stop()
		sigSpy1.SeenEventually(t, sigWhenElectedEnd, "e", "1")
		sigSpy2.NotSeen(t, sigWhenElectedEnd, "e", "2")
		e2.Stop()
	})
}
