package test

import (
	"context"

	"github.com/tempcke/dsync/internal/k8s"
	"github.com/tempcke/dsync/internal/signaler"
)

const (
	SigStartedLeading = "sigStartedLeading"
	SigStoppedLeading = "sigStoppedLeading"
	SigNewLeader      = "sigNewLeader"
)

func ElectorCallbacks(ctx context.Context, pod string) k8s.LeaderCallbacks {
	sig := signaler.Get(ctx)
	return k8s.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			sig.Send(SigStartedLeading, "pod", pod)
		},
		OnStoppedLeading: func() {
			sig.Send(SigStoppedLeading, "pod", pod)
		},
		OnNewLeader: func(identity string) {
			sig.Send(SigNewLeader, "pod", pod, "id", identity)
		},
	}
}
