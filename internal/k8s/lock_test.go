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

func TestLock(t *testing.T) {
	var (
		ns      = "local"
		scope   = "testing"
		cs      = fake.NewClientset()
		kWithCS = k8s.KubeWithCS(cs)
		podA    = "A"
		podB    = "B"
	)
	t.Run("try", func(t *testing.T) {
		var (
			ctx  = test.Context(t, time.Minute/4)
			task = uuid.NewString()[24:]
			r    = dsync.NewScopedResource(ns, scope, task)
		)

		k, err := k8s.New(kWithCS)
		require.NoError(t, err)

		lockA, err := k.GetLock(ctx, r, podA)
		require.NoError(t, err)

		lockB, err := k.GetLock(ctx, r, podB)
		require.NoError(t, err)

		require.NoError(t, lockA.TryLock())
		require.Equal(t, podA, lockA.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())
		require.ErrorIs(t, lockB.TryLock(), k8s.ErrAlreadyLocked)
		require.ErrorIs(t, lockB.Unlock(), k8s.ErrNotLockHolder)

		require.NoError(t, lockA.Unlock())
		require.Equal(t, "", lockA.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())

		require.NoError(t, lockB.TryLock())
		require.Equal(t, podB, lockB.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())
		require.ErrorIs(t, lockA.TryLock(), k8s.ErrAlreadyLocked)

		require.NoError(t, lockB.Unlock())
		require.ErrorIs(t, lockB.Unlock(), k8s.ErrNotLocked)
		require.ErrorIs(t, lockA.Unlock(), k8s.ErrNotLocked)
	})
	t.Run("lock", func(t *testing.T) {
		var (
			ctx           = test.Context(t, time.Minute/4)
			task          = uuid.NewString()[24:]
			r             = dsync.NewScopedResource(ns, scope, task)
			ttl           = time.Second * 1
			retryInterval = time.Second / 10
			kWithLockTTL  = k8s.KubeWithLockTTL(ttl)
		)

		k, err := k8s.New(kWithCS, kWithLockTTL)
		require.NoError(t, err)

		lockA, err := k.GetLock(ctx, r, podA)
		require.NoError(t, err)
		lockB, err := k.GetLock(ctx, r, podB)
		require.NoError(t, err)
		k8s.LockWithRetryInterval(retryInterval)(lockA)
		k8s.LockWithRetryInterval(retryInterval)(lockB)

		// lock and unlock
		require.NoError(t, lockA.Lock())
		require.Equal(t, podA, lockA.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())
		require.ErrorIs(t, lockB.TryLock(), k8s.ErrAlreadyLocked)
		require.ErrorIs(t, lockB.Unlock(), k8s.ErrNotLockHolder)

		require.NoError(t, lockA.Unlock())
		require.Equal(t, "", lockA.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())

		require.NoError(t, lockB.Lock())
		require.Equal(t, podB, lockB.Holder())
		require.Equal(t, lockA.Holder(), lockB.Holder())
		require.ErrorIs(t, lockA.TryLock(), k8s.ErrAlreadyLocked)

		require.NoError(t, lockB.Unlock())
		require.ErrorIs(t, lockB.Unlock(), k8s.ErrNotLocked)
		require.ErrorIs(t, lockA.Unlock(), k8s.ErrNotLocked)

		// take lock after ttl
		require.NoError(t, lockA.Lock())
		acquiredA := time.Now()
		require.NoError(t, lockB.Lock())
		acquiredB := time.Now()
		if d := acquiredB.Sub(acquiredA).Round(time.Millisecond); d < ttl {
			t.Fatalf("lockB took the lock from A in %v when ttl was %v", d.String(), ttl.String())
		}
		assert.ErrorIs(t, lockA.Unlock(), k8s.ErrNotLockHolder)
		require.NoError(t, lockB.Unlock())
	})
}
