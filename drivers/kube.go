package drivers

import (
	"context"

	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/internal/k8s"
)

type (
	KubeDriver struct {
		k     k8s.Kube
		ns    string
		scope string
	}
)

func NewKubeDriver(conf configs.Config) (*KubeDriver, error) {
	k, err := k8s.New()
	if err != nil {
		return nil, err
	}
	return NewKubeWithK(conf, *k), nil
}
func NewKubeWithK(conf configs.Config, k k8s.Kube) *KubeDriver {
	conf = configs.WithDefaults(conf, configs.Defaults)
	return &KubeDriver{
		k:     k,
		ns:    conf(configs.KeyNamespace),
		scope: conf(configs.KeyLeaseScope),
	}
}
func (d *KubeDriver) GetLeader(task string) string {
	out, err := d.k.GetLeader(context.TODO(), d.Resource(task))
	if err != nil {
		return ""
	}
	return out
}
func (d *KubeDriver) Resource(name string) dsync.Resource {
	return dsync.NewScopedResource(d.ns, d.scope, dsync.SanitizeName(name))
}

func (d *KubeDriver) GetElection(ctx context.Context, task, candidate string) (dsync.ElectionDriver, error) {
	r := d.Resource(task)
	e, err := d.k.NewElection(ctx, r, candidate)
	if err != nil {
		return nil, err
	}
	return e, nil
}
func (d *KubeDriver) GetLock(ctx context.Context, name, pod string) (dsync.LockDriver, error) {
	r := d.Resource(name)
	return d.k.GetLock(ctx, r, pod)
}
