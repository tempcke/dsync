package drivers_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/drivers"
	"github.com/tempcke/dsync/internal/k8s"
	"github.com/tempcke/dsync/internal/signaler"
	"github.com/tempcke/dsync/internal/test"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDrivers(t *testing.T) {
	var (
		_ dsync.Driver = (*drivers.KubeDriver)(nil)
		_ dsync.Driver = (*drivers.MockDriver)(nil)

		conf = configs.New()
	)

	var tests = map[string]struct{ driver dsync.Driver }{
		"mock": {drivers.NewMockDriver(conf)},
		"kube": {kubeDriver(t, conf)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testElection(t, tc.driver)
			testLocks(t, tc.driver)
		})
	}
}
func testElection(t *testing.T, driver dsync.Driver) {
	var (
		podA = "A"
		podB = "B"

		sigWhenElectedStart = "sigWhenElectedStart"
		sigWhenElectedEnd   = "sigWhenElectedEnd"
		sigElected          = "sigElected"
	)
	t.Run("resource", func(t *testing.T) {
		var (
			d        = dsync.New(driver, podA)
			rName    = randString(5)
			resource = d.Resource(rName)
		)
		assert.Equal(t, 5, len(rName))
		assert.Equal(t, dsync.DefaultNamespace, resource.Namespace)
		require.Equal(t, dsync.DefaultScope, resource.Scope)
		assert.Equal(t, rName, resource.Name)
	})
	t.Run("election with invalid resource name", func(t *testing.T) {
		var (
			ctx  = test.Context(t, time.Minute/4)
			dA   = dsync.New(driver, podA)
			task = "Bad_Name"
			r    = dA.Resource("Bad_Name")
			eA   = dA.Election(ctx, task)
		)
		assert.ErrorIs(t, r.Validate(), dsync.ErrBadResourceName)
		assert.ErrorIs(t, eA.Err(), dsync.ErrBadResourceName)
	})
	t.Run("election IsLeader", func(t *testing.T) {
		var (
			ctx  = test.Context(t, time.Minute/4)
			dA   = dsync.New(driver, podA)
			dB   = dsync.New(driver, podB)
			task = randString(5)
			r    = dA.Resource(task)
			eA   = dA.Election(ctx, task)
			eB   = dB.Election(ctx, task)
		)

		// both dA and dB should construct a resource the same way given the same name
		assert.True(t, dA.Resource(task).Equal(dB.Resource(task)))
		assert.NoError(t, r.Validate())
		assert.NoError(t, eA.Err())
		assert.NoError(t, eB.Err())

		// eventually either A or B should be elected which is not instant
		assert.Eventually(t, func() bool {
			return eA.GetLeader() != "" && eB.GetLeader() != ""
		}, time.Minute/4, time.Second/2)

		// now one and only one must be leader, we do not care which
		assert.True(t, eA.IsLeader() || eB.IsLeader())
		assert.False(t, eA.IsLeader() && eB.IsLeader())
	})
	t.Run("election WhenElected", func(t *testing.T) {
		var (
			ctx    = test.Context(t, time.Minute/4)
			dA     = dsync.New(driver, podA)
			task   = randString(5)
			eA     = dA.Election(ctx, task)
			sigSpy = test.NewSigSpy(ctx)
		)
		sigSpy.ListenAndPrint(ctx, t)
		eA.WhenElected(func(termCtx context.Context) {
			require.NotNil(t, termCtx)
			signaler.Send(ctx, sigElected)
		})
		sigSpy.SeenEventually(t, sigElected)
	})
	t.Run("when elected", func(t *testing.T) {
		var (
			d = dsync.New(driver, podA)

			ctx    = test.Context(t, time.Minute/4)
			sigSpy = test.NewSigSpy(ctx)
			task   = randString(8)
		)
		sigSpy.ListenAndPrint(ctx, t)

		e := d.Election(ctx, task)
		// can it handle multiple WhenElected 's ?
		e.WhenElected(func(termCtx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "whenElected", "1")
			<-termCtx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "whenElected", "1")
		})
		e.WhenElected(func(termCtx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "whenElected", "2")
			<-termCtx.Done()
			signaler.Send(ctx, sigWhenElectedEnd, "whenElected", "2")
		})
		sigSpy.SeenEventually(t, sigWhenElectedStart, "whenElected", "1")
		sigSpy.SeenEventually(t, sigWhenElectedStart, "whenElected", "2")
		require.True(t, e.IsLeader())

		e.Stop()
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "whenElected", "1")
		sigSpy.SeenEventually(t, sigWhenElectedEnd, "whenElected", "2")
		assert.False(t, e.IsLeader())
	})
	t.Run("two elections for same pod", func(t *testing.T) {
		var (
			ctx1    = test.Context(t, time.Minute/4)
			ctx2    = test.Context(t, time.Minute/4)
			sigSpy1 = test.NewSigSpy(ctx1)
			sigSpy2 = test.NewSigSpy(ctx2)
			task    = randString(8)

			dA1 = dsync.New(driver, podA)
			dA2 = dsync.New(driver, podA)
			e1  = dA1.Election(ctx1, task)
			e2  = dA2.Election(ctx2, task)
		)
		sigSpy1.ListenAndPrint(ctx1, t)
		sigSpy2.ListenAndPrint(ctx2, t)

		e1.WhenElected(func(ctx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "e", "1")
		})
		e2.WhenElected(func(ctx context.Context) {
			signaler.Send(ctx, sigWhenElectedStart, "e", "2")
		})

		sigSpy1.SeenEventually(t, sigWhenElectedStart, "e", "1")

		require.NoError(t, e1.Err())
		require.NoError(t, e2.Err())
		require.True(t, e1.IsLeader())
		require.True(t, e2.IsLeader())

		sigSpy2.SeenEventually(t, sigWhenElectedStart, "e", "2")
		sigSpy1.NotSeen(t, sigWhenElectedStart, "e", "2")
		sigSpy2.NotSeen(t, sigWhenElectedStart, "e", "1")
	})
}
func testLocks(t *testing.T, driver dsync.Driver) {
	var (
		podA = "A"
		podB = "B"
	)
	t.Run("lock", func(t *testing.T) {
		var (
			ctx      = test.Context(t, time.Minute/4)
			d        = dsync.New(driver, podA)
			lockName = randString(8)
		)
		dl := d.NewLock(ctx, lockName)

		require.NoError(t, dl.Err())
		require.Error(t, dl.Unlock(), dsync.ErrNotLocked)
		require.NoError(t, dl.TryLock())
		require.ErrorIs(t, dl.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, dl.Unlock())

		require.NoError(t, dl.Lock())
		require.ErrorIs(t, dl.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, dl.Unlock())
		require.Error(t, dl.Unlock(), dsync.ErrNotLocked)
		require.NoError(t, dl.Err())
	})
	t.Run("lock multi pod", func(t *testing.T) {
		var (
			ctx          = test.Context(t, time.Minute/4)
			dA           = dsync.New(driver, podA)
			dB           = dsync.New(driver, podB)
			lockName     = randString(8)
			sigSpy       = test.NewSigSpy(ctx)
			sigStartLock = "sigStartLock"
			sigLocked    = "sigLocked"
		)

		lockA := dA.NewLock(ctx, lockName)
		require.NoError(t, lockA.Err())
		lockB := dB.NewLock(ctx, lockName)
		require.NoError(t, lockB.Err())

		// TryLock
		require.NoError(t, lockA.TryLock())
		require.ErrorIs(t, lockB.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, lockA.Unlock())
		require.NoError(t, lockB.TryLock())
		require.ErrorIs(t, lockA.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, lockB.Unlock())

		// take turns with Lock
		require.NoError(t, lockA.Lock())
		require.ErrorIs(t, lockB.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, lockA.Unlock())
		require.NoError(t, lockB.Lock())
		require.ErrorIs(t, lockA.TryLock(), dsync.ErrAlreadyLocked)
		require.NoError(t, lockB.Unlock())

		// wait for lock
		require.NoError(t, lockA.Lock())
		go func() {
			signaler.Send(ctx, sigStartLock, "lock", "B")
			require.NoError(t, lockB.Lock())
			signaler.Send(ctx, sigLocked, "lock", "B")
		}()
		sigSpy.SeenEventually(t, sigStartLock, "lock", "B")
		sigSpy.NotSeen(t, sigLocked, "lock", "B")
		require.NoError(t, lockA.Unlock())
		sigSpy.SeenEventually(t, sigLocked, "lock", "B")
		require.NoError(t, lockB.Unlock())
	})
	t.Run("lock with context can cancel wait for lock", func(t *testing.T) {
		// canceling the context should cancel the attempt to acquire a lock
		// once a lock locked with a context, canceling the context does
		// not guarantee that the lock will be unlocked.
		var (
			ctx          = test.Context(t, time.Minute/4)
			dA           = dsync.New(driver, podA)
			dB           = dsync.New(driver, podB)
			lockName     = randString(8)
			sigSpy       = test.NewSigSpy(ctx)
			sigStartLock = "sigStartLock"
			// sigLocked    = "sigLocked"
			sigLockCanceled = "sigLockCanceled"
		)

		// create locks with parent context
		lockA := dA.NewLock(ctx, lockName)
		require.NoError(t, lockA.Err())
		lockB := dB.NewLock(ctx, lockName)
		require.NoError(t, lockB.Err())

		// ctxA can be canceled
		ctxA, cancelA := context.WithCancel(ctx)
		t.Cleanup(cancelA)
		ctxB, cancelB := context.WithCancel(ctx)
		t.Cleanup(cancelB)

		require.NoError(t, lockA.LockContext(ctxA))
		go func() {
			signaler.Send(ctx, sigStartLock, "lock", "B")
			require.ErrorIs(t, lockB.LockContext(ctxB), context.Canceled)
			signaler.Send(ctx, sigLockCanceled, "lock", "B")
		}()
		sigSpy.SeenEventually(t, sigStartLock, "lock", "B")
		sigSpy.NotSeen(t, sigLockCanceled, "lock", "B")

		// cancel B causes us to give up trying to get the lock
		cancelB()
		sigSpy.SeenEventually(t, sigLockCanceled, "lock", "B")

		// cancel A shouldn't do anything
		require.ErrorIs(t, lockA.TryLock(), dsync.ErrAlreadyLocked)
		cancelA()
		require.ErrorIs(t, lockA.TryLock(), dsync.ErrAlreadyLocked)
		require.ErrorIs(t, lockB.TryLock(), dsync.ErrAlreadyLocked)

		// only A should be able to unlock it
		require.Error(t, lockB.Unlock(), dsync.ErrNotLockHolder)
		require.NoError(t, lockA.Unlock())
	})
	t.Run("DoWithLock", func(t *testing.T) {
		var (
			ctx       = test.Context(t, time.Minute/4)
			dA        = dsync.New(driver, podA)
			dB        = dsync.New(driver, podB)
			lockName  = randString(8)
			sigSpy    = test.NewSigSpy(ctx)
			sigLocked = "sigLocked"
		)

		lockA := dA.NewLock(ctx, lockName)
		require.NoError(t, lockA.Err())
		lockB := dB.NewLock(ctx, lockName)
		require.NoError(t, lockB.Err())

		err := lockA.DoWithLock(ctx, func() error {
			signaler.Send(ctx, sigLocked, "lock", "A")
			// prove we have the lock while this function is executing.
			require.ErrorIs(t, lockB.TryLock(), dsync.ErrAlreadyLocked)
			return nil
		})
		require.NoError(t, err)
		require.True(t, sigSpy.Seen(sigLocked, "lock", "A"))
		require.NoError(t, lockB.TryLock())
		require.NoError(t, lockB.Unlock())
	})
}

func kubeDriver(t *testing.T, c configs.Config) *drivers.KubeDriver {
	cs := fake.NewClientset()
	k, err := k8s.New(k8s.KubeWithCS(cs))
	require.NoError(t, err)
	return drivers.NewKubeWithK(c, *k)
}
func randString(n int) string {
	return uuid.NewString()[36-n:]
}
