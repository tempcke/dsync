package k8s_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/internal/k8s"
	"github.com/tempcke/dsync/internal/test"
	"k8s.io/client-go/kubernetes/fake"
)

func TestElector(t *testing.T) {
	var (
		ns    = "local"
		podA  = "A"
		podB  = "B"
		scope = "testing"
		cs    = fake.NewClientset()
	)

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
}

func newTaskName() string {
	return "task-" + uuid.NewString()[24:]
}
