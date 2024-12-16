package k8s

import (
	"context"
	"strings"
	"time"

	"github.com/tempcke/dsync"
	"k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type (
	Kube struct {
		kcp         string // KubeConfigPath - $HOME/.kube/config for local testing
		cs          kubernetes.Interface
		leaseClient LeaseClient
		lockTTL     time.Duration
	}
	KubeOpt func(k *Kube)
)

const (
	DefaultLockTTL       = time.Second * 0 // unlimited
	DefaultRetryInterval = time.Second
)

func New(opts ...KubeOpt) (*Kube, error) {
	var k Kube
	for _, opt := range opts {
		opt(&k)
	}
	if k.cs == nil {
		cs, err := kubeCS(k.kcp)
		if err != nil {
			return nil, err
		}
		k.cs = cs
	}
	k.leaseClient = k.cs.CoordinationV1()
	return &k, nil
}
func KubeWithLockTTL(d time.Duration) KubeOpt    { return func(k *Kube) { k.lockTTL = d } }
func KubeWithCS(cs kubernetes.Interface) KubeOpt { return func(k *Kube) { k.cs = cs } }
func KubeWithKCP(kcp string) KubeOpt             { return func(k *Kube) { k.kcp = kcp } }

func (k Kube) NewElection(
	ctx context.Context,
	r dsync.Resource,
	candidate string,
	callbacks ...LeaderCallbacks,
) (*Election, error) {
	e, err := NewElection(ctx, k.leaseClient, r, candidate, callbacks...)
	if err != nil {
		return nil, err
	}
	return e, nil
}
func (k Kube) GetLeader(ctx context.Context, r dsync.Resource) (string, error) {
	lease, err := k.GetLease(ctx, r.Namespace, r.ElectionName())
	if err != nil {
		if strings.HasSuffix(err.Error(), "not found") {
			return "", ErrNotFound
		}
		return "", err
	}
	leader := lease.Spec.HolderIdentity
	if leader == nil {
		return "", ErrNoLeader
	}
	return *leader, nil
}
func (k Kube) GetLease(ctx context.Context, namespace, name string) (*v1.Lease, error) {
	lease, err := k.leaseClient.Leases(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return lease, err
}
func (k Kube) GetLock(ctx context.Context, r dsync.Resource, requester string) (*Lock, error) {
	opts := []LockOption{
		LockWithTTL(k.lockTTL),
	}
	return NewLock(ctx, k.leaseClient, r, requester, opts...)
}
