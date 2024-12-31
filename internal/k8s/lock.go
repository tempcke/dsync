package k8s

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tempcke/dsync"
	v1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	Lock struct {
		ctx    context.Context
		client LeaseClient // v1.LeasesGetter
		r      dsync.Resource
		pod    string
		lease  *v1.Lease

		ttl           time.Duration
		retryInterval time.Duration
	}
	LockOption func(*Lock)
)

func (l *Lock) Resource() dsync.Resource { return l.r }
func (l *Lock) Pod() string              { return l.pod }
func (l *Lock) Holder() string {
	if lease := l.getLease(); lease != nil {
		if id := lease.Spec.HolderIdentity; id != nil {
			return *id
		}
	}
	return ""
}
func (l *Lock) Lock() error                           { return l.lock(l.ctx) }
func (l *Lock) LockContext(ctx context.Context) error { return l.lock(ctx) }
func (l *Lock) lock(ctx context.Context) error {
	var (
		wait  time.Duration
		retry = retryInterval(l.retryInterval)
	)
	if ctx == nil {
		ctx = l.ctx
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			if err := l.TryLock(); err != nil {
				if !errors.Is(err, ErrAlreadyLocked) {
					return err // something unexpected happened
				}
				if wait == 0 {
					wait = retry
				}
				continue // lock taken, try again
			}
			return nil // lock is ours!
		}
	}
}
func (l *Lock) TryLock() error {
	lease := l.getLease()
	if lease == nil {
		return ErrLeaseNotFound
	}

	// Check if the lease is already held and active
	if h := lease.Spec.HolderIdentity; h != nil {
		at := lease.Spec.AcquireTime.Time
		ld := ldsToDur(lease.Spec.LeaseDurationSeconds)
		if ld == 0 || at.Add(ld).After(time.Now()) {
			// lock not expired
			return ErrAlreadyLocked
		}
		// else lock is expired, so we can take it
	}

	// Attempt to update the lease to claim the lock
	var (
		now         = metav1.NowMicro()
		transitions = lease.Spec.LeaseTransitions
	)
	if transitions == nil || *transitions < 0 {
		transitions = ptr(int32(0))
	}
	lease.Spec.HolderIdentity = ptr(l.pod)
	lease.Spec.LeaseDurationSeconds = leaseDurSecs(l.ttl)
	lease.Spec.RenewTime = &now
	lease.Spec.LeaseTransitions = transitions
	lease.Spec.AcquireTime = &now
	if err := l.updateLease(lease); err != nil {
		return err
	}

	return nil
}
func (l *Lock) Unlock() error {
	lease := l.getLease()
	if lease == nil {
		return ErrLeaseNotFound
	}

	// Ensure this pod is the current holder
	if h := lease.Spec.HolderIdentity; h == nil {
		return ErrNotLocked
	} else if *h != l.pod {
		return ErrNotLockHolder
	}

	// Clear the HolderIdentity to release the lock
	lease.Spec.HolderIdentity = nil
	lease.Spec.AcquireTime = nil
	lease.Spec.LeaseDurationSeconds = nil
	if err := l.updateLease(lease); err != nil {
		return err
	}
	return nil
}

func (l *Lock) getLease() *v1.Lease {
	if lease, _ := getLease(l.ctx, l.client, l.r); lease != nil {
		l.lease = lease
		return lease
	}
	return nil
}
func (l *Lock) updateLease(in *v1.Lease) error {
	out, err := l.client.Leases(l.r.Namespace).Update(l.ctx, in, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateLeaseFail, err)
	}
	l.lease = out
	return nil
}

func NewLock(
	ctx context.Context, lc LeaseClient,
	r dsync.Resource, pod string, opts ...LockOption,
) (*Lock, error) {
	lease, err := getOrCreateLease(ctx, lc, r)
	if err != nil {
		return nil, err
	}
	lock := Lock{
		ctx:    ctx,
		client: lc,
		r:      r,
		pod:    pod,
		lease:  lease,
	}
	for _, opt := range opts {
		opt(&lock)
	}
	return &lock, nil
}
func LockWithTTL(d time.Duration) LockOption           { return func(l *Lock) { l.ttl = d } }
func LockWithRetryInterval(d time.Duration) LockOption { return func(l *Lock) { l.retryInterval = d } }
func getOrCreateLease(ctx context.Context, lc LeaseClient, r dsync.Resource) (*v1.Lease, error) {
	if lease, err := getLease(ctx, lc, r); err == nil {
		return lease, nil
	}
	lease, err := lc.Leases(r.Namespace).Create(ctx, ptr(newLease(r)), metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateLeaseFail, err)
	}
	return lease, nil
}
func newLease(r dsync.Resource) v1.Lease {
	return v1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: r.LockName()},
		Spec: v1.LeaseSpec{
			HolderIdentity:       nil,
			LeaseDurationSeconds: nil,
			LeaseTransitions:     ptr(int32(0)),
		},
	}
}
func getLease(ctx context.Context, lc LeaseClient, r dsync.Resource) (*v1.Lease, error) {
	return lc.Leases(r.Namespace).Get(ctx, r.LockName(), metav1.GetOptions{})
}
func retryInterval(d time.Duration) time.Duration {
	if d == 0 {
		return DefaultRetryInterval
	}
	return d
}
func leaseDurSecs(dur time.Duration) *int32 {
	if dur.Seconds() < 1 {
		dur = DefaultLockTTL
	}
	return ptr(int32(dur.Seconds()))
}
func ldsToDur(in *int32) time.Duration {
	if in == nil {
		return 0
	}
	return time.Duration(*in) * time.Second
}
func ptr[T any](in T) *T {
	return &in
}
