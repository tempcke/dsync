package drivers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/drivers"
	"github.com/tempcke/dsync/internal/signaler"
	"github.com/tempcke/dsync/internal/test"
)

func TestMockDriver(t *testing.T) {
	var (
		podA                = "A"
		podB                = "B"
		conf                = configs.New()
		driver              = drivers.NewMockDriver(conf)
		sigWhenElectedStart = "sigWhenElectedStart"
		sigWhenElectedEnd   = "sigWhenElectedEnd"
	)
	t.Run("force leader", func(t *testing.T) {
		var (
			ctx    = test.Context(t, time.Minute/4)
			sigSpy = test.NewSigSpy(ctx)
			task   = randString(8)

			dA = dsync.New(driver, podA)
			dB = dsync.New(driver, podB)
			r  = dA.Resource(task)
		)
		sigSpy.ListenAndPrint(ctx, t)

		// A elected first
		eA, err := dA.NewElection(ctx, task)
		require.NoError(t, err)
		eA.WhenElected(func(termCtx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "pod", podA)
			<-termCtx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "pod", podA)
		})
		sigSpy.SeenEventually(t, sigWhenElectedStart, "pod", podA)

		eB, err := dB.NewElection(ctx, task)
		require.NoError(t, err)
		eB.WhenElected(func(termCtx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "pod", podB)
			<-termCtx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "pod", podB)
		})

		require.True(t, eA.IsLeader())
		require.False(t, eB.IsLeader())

		sigSpy.Clear()
		driver.ForceLeader(r, podB)
		require.False(t, eA.IsLeader())
		require.True(t, eB.IsLeader())
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "pod", podA)
		sigSpy.SeenEventually(t, sigWhenElectedStart, "pod", podB)

		sigSpy.Clear()
		driver.ForceLeader(r, podA)
		require.True(t, eA.IsLeader())
		require.False(t, eB.IsLeader())
		sigSpy.SeenEventually(t, sigWhenElectedStart, "pod", podA)
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "pod", podB)
	})
}
