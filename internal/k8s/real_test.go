//go:build realtest

package k8s_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/internal/k8s"
	"github.com/tempcke/dsync/internal/test"
)

func TestReal_Election(t *testing.T) {
	// To run this test you must set KUBE_CONFIG_PATH and KUBE_NAMESPACE in .env
	// KUBE_CONFIG_PATH=$home/.kube/config
	// KUBE_NAMESPACE=development # assuming your gcloud env is development
	if os.Getenv(configs.KeyConfigPath) == "" {
		t.Setenv(configs.KeyConfigPath, fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))
	}
	if os.Getenv(configs.KeyNamespace) == "" {
		t.Setenv(configs.KeyNamespace, "sandbox")
	}
	var (
		conf   = os.Getenv
		ns     = conf(configs.KeyNamespace)
		scope  = conf(configs.KeyLeaseScope)
		kcp    = conf(configs.KeyConfigPath)
		idBase = uuid.NewString()
		podA   = "A-" + idBase
		podB   = "B-" + idBase
	)
	k, err := k8s.New(k8s.KubeWithKCP(kcp))
	require.NoError(t, err)
	t.Run("is leader multi pod", func(t *testing.T) {
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

		// no leader yet
		elected, err := k.GetLeader(ctx, r)
		assert.ErrorIs(t, err, k8s.ErrNotFound)
		assert.Empty(t, elected)

		// podA
		eA, err := k.NewElection(ctx, r, podA, cbA)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podA, "id", podA)
		assert.True(t, eA.IsLeader())
		assert.Equal(t, podA, eA.GetLeader())
		elected, err = k.GetLeader(ctx, r)
		require.NoError(t, err)
		assert.Equal(t, podA, elected)

		// podB
		eB, err := k.NewElection(ctx, r, podB, cbB)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podB)
		assert.True(t, eA.IsLeader() || eB.IsLeader())
		assert.False(t, eA.IsLeader() && eB.IsLeader())
	})
	t.Run("get leader", func(t *testing.T) {
		// here we have 2 pods volunteering to be leader
		// in the end one and only one must be leader, makes no difference which one
		var (
			ctx    = test.Context(t, time.Minute/4)
			task   = newTaskName()
			r      = dsync.NewScopedResource(ns, scope, task)
			sigSpy = test.NewSigSpy(ctx)
			cbA    = test.ElectorCallbacks(ctx, podA)

			leaseCtx, leaseCancel = context.WithCancel(ctx)
		)
		t.Cleanup(leaseCancel)
		sigSpy.ListenAndPrint(ctx, t)

		// no leader yet
		elected, err := k.GetLeader(ctx, r)
		assert.ErrorIs(t, err, k8s.ErrNotFound)
		assert.Empty(t, elected)

		// elect A
		eA, err := k.NewElection(leaseCtx, r, podA, cbA)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podA, "id", podA)
		assert.True(t, eA.IsLeader())
		assert.Equal(t, podA, eA.GetLeader())
		elected, err = k.GetLeader(ctx, r)
		require.NoError(t, err)
		assert.Equal(t, podA, elected)

		// // cancel leaseCtx and check
		// // turns out testing this isn't practical unless we hack the lease duration
		// // which I'm not going to do yet, so leaving this here for the future...
		// leaseCancel()
		// assert.EventuallyWithT(t, func(t *assert.CollectT) {
		// 	elected, err := k.GetLeader(ctx, r)
		// 	assert.ErrorIs(t, err, k8s.ErrNoLeader)
		// 	assert.Empty(t, elected)
		// }, time.Minute/6, time.Second/4)
	})
	t.Run("multiple electors for same thing", func(t *testing.T) {
		// what happens if we construct the same elector multiple times?
		// turns out the k8s client just handles this for us and both work the same
		var (
			ctx    = test.Context(t, time.Minute/4)
			task   = newTaskName()
			r      = dsync.NewScopedResource(ns, scope, task)
			sigSpy = test.NewSigSpy(ctx)
			cbA    = test.ElectorCallbacks(ctx, podA)
		)
		sigSpy.ListenAndPrint(ctx, t)

		// no leader yet
		elected, err := k.GetLeader(ctx, r)
		assert.ErrorIs(t, err, k8s.ErrNotFound)
		assert.Empty(t, elected)

		// podA
		eA, err := k.NewElection(ctx, r, podA, cbA)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", podA, "id", podA)
		assert.True(t, eA.IsLeader())
		assert.Equal(t, podA, eA.GetLeader())
		elected, err = k.GetLeader(ctx, r)
		require.NoError(t, err)
		assert.Equal(t, podA, elected)

		// let's try to make the elector again...
		a2 := "A2" // pod name isn't really any different, but we will signal on this, so we can tell
		cbA2 := test.ElectorCallbacks(ctx, a2)
		eA2, err := k.NewElection(ctx, r, podA, cbA2)
		require.NoError(t, err)
		sigSpy.SeenEventually(t, test.SigNewLeader, "pod", a2, "id", podA)
		assert.True(t, eA2.IsLeader())
		assert.Equal(t, podA, eA2.GetLeader())
		assert.True(t, eA.IsLeader() && eA2.IsLeader())
	})
}
