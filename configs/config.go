package configs

import (
	"os"

	"github.com/google/uuid"
)

const (
	// KeyNamespace is the same that you would use with kubectl --namespace
	// just define it in your namespace specific helm chart
	KeyNamespace = "KUBE_NAMESPACE"
	DefNamespace = "default"

	// KeyLeaseScope is used as a prefix for leases.
	// leases are scoped by default only to namespace, so this scopes them
	// to your service to ensure leases from multiple services do not collide
	// for that reason, just use your service name such as tm-sync
	KeyLeaseScope = "KUBE_LEASE_SCOPE"
	DefLeaseScope = "unscoped"

	// KeyConfigPath is only needed by tests ran on your local machine
	// which must talk to the real external k8s system using your own account
	// 99.9% of your tests should use the mock client rather than do this.
	// if you do decide to use it your own local config file is used
	// which is likely /home/YOUR_USER_NAME/.kube/config
	KeyConfigPath = "KUBE_CONFIG_PATH"

	// KeyPodName is only needed to allow tests to simulate different pod's
	// each running instance of the application will generate its own
	// global unique ID, this will not match what you see
	KeyPodName = "KUBE_POD_NAME"
)

type (
	Config = func(string) string // os.Getenv | viper.GetString etc
)

var FromEnv = os.Getenv

var Defaults = map[string]string{
	KeyNamespace:  DefNamespace,
	KeyLeaseScope: DefLeaseScope,
	KeyPodName:    uuid.NewString()[24:],
}

func New() Config {
	return WithDefaults(FromEnv, Defaults)
}
func FromMap(m map[string]string) Config {
	return new(ConfBuilder).WithMap(m).Build()
}

func WithDefaults(conf Config, m map[string]string) Config {
	if conf == nil {
		conf = FromEnv
	}
	return func(k string) string {
		if v := conf(k); v != "" {
			return v
		}
		if m, ok := m[k]; ok {
			return m
		}
		return ""
	}
}
