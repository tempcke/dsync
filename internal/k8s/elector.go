package k8s

import (
	"context"
	"time"

	"github.com/tempcke/dsync"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type (
	// LeaderCallbacks callbacks that are triggered during certain lifecycle events
	// This is copied directly out of k8s.io/client-go/tools/leaderelection
	LeaderCallbacks struct {
		// OnStartedLeading is called when a LeaderElector client starts leading
		OnStartedLeading func(context.Context)
		// OnStoppedLeading is called when a LeaderElector client stops leading
		OnStoppedLeading func()
		// OnNewLeader is called when the client observes a leader that is
		// not the previously observed leader. This includes the first observed
		// leader when the client starts.
		OnNewLeader func(identity string)
	}

	LeaseClient = coordinationv1client.LeasesGetter

	Elector struct {
		r         dsync.Resource
		candidate string
		client    LeaseClient
		callbacks []LeaderCallbacks
		le        *leaderelection.LeaderElector
	}
)

func NewLeaderElector(
	ctx context.Context,
	client LeaseClient,
	r dsync.Resource,
	candidate string,
	callbacks ...LeaderCallbacks,
) (*Elector, error) {
	e := Elector{
		r:         r,
		candidate: candidate,
		client:    client,
		callbacks: callbacks,
	}

	lec := e.conf()
	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return nil, err
	}
	if lec.WatchDog != nil {
		lec.WatchDog.SetLeaderElection(le)
	}
	e.le = le
	go e.le.Run(ctx)
	return &e, nil
}

func (e Elector) Task() string {
	return e.r.Name
}

// GetLeader will return the current leader
// if the election has not completed it returns empty string
func (e Elector) GetLeader() string {
	if e.le != nil {
		return e.le.GetLeader()
	}
	return ""
}

// IsLeader will tell you if you are the leader right now
// however you could become the new leader at any moment
func (e Elector) IsLeader() bool {
	if e.le != nil {
		return e.le.IsLeader()
	}
	return false
}

func (e Elector) conf() leaderelection.LeaderElectionConfig {
	return leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      e.r.ElectionName(),
				Namespace: e.r.Namespace,
			},
			Client:     e.client,
			LockConfig: resourcelock.ResourceLockConfig{Identity: e.candidate},
		},
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: e.onStartedLeading,
			OnStoppedLeading: e.onStoppedLeading,
			OnNewLeader:      e.onNewLeader,
		},
	}
}
func (e Elector) onStartedLeading(ctx context.Context) {
	for _, cb := range e.callbacks {
		if f := cb.OnStartedLeading; f != nil {
			f(ctx)
		}
	}
}
func (e Elector) onStoppedLeading() {
	for _, cb := range e.callbacks {
		if f := cb.OnStoppedLeading; f != nil {
			f()
		}
	}
}
func (e Elector) onNewLeader(identity string) {
	for _, cb := range e.callbacks {
		if f := cb.OnNewLeader; f != nil {
			f(identity)
		}
	}

}
