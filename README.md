# Distributed Sync (dsync)

`dsync` is for distributed sync.  It aims to solve problems where coordination between different pods running the same service are required.

k8s offers an API to allow you to execute leader election.  However, that API is much more complex than we actually require.  So this library was written as a wrapper around it.

## Upfront warning about testing
DO NOT import the fake k8s client.  It is huge and will make `go vet` take forever and make `staticcheck` OOM kill your pipeline.  This package uses it to confirm things so that you do not have to.  Which is why this package does not use staticcheck in pipeline.

Rather use `dsync.New(drivers.NewMockDriver(), "podName")` to construct a client for your tests.

## Constructors

For a more complete example look at examples/main.go

```go
package main

import (
	"context"
	"os"

	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/drivers"
)

func main() {
	var (
		ctx       = context.Background()
		envFn     = os.Getenv // could use viper.GetString
	)

	// construct driver
	conf := configs.WithDefaults(envFn, configs.Defaults)
	driver, err := drivers.NewKubeDriver(conf)
	if err != nil {
		panic(err)
	}

	// construct dsync with driver
	d := dsync.New(driver, conf(configs.KeyPodName))
	e := d.Election(ctx, "token-manager")
	// if you don't check Err() all methods just no-op
	if err := e.Err(); err != nil {
		panic(err)
    }
	e.WhenElected(func(ctx context.Context){
		// do stuff until ctx is canceled
    })
}
```

### Configuration
`conf configs.Config` allows you to define the configuration.  This is obviously important for tests or if you use different env vars than the ones it expects.

*NOTE:* For an example of how to setup your helm chart please look in `example/pipeline`

* `KUBE_NAMESPACE` - the namespace you would use with `kubectl -n` which can be given to you easily in your helm chart
* `KUBE_LEASE_SCOPE` - it is recommended to use the name of your service.  because all leases are scoped to the namespace, it is possible that lease names from different services could conflict with each-other.  Therefore, we provide a service level prefix to ensure that they don't.  Because it is used to prefix lease names it must follow the same constraints that lease names have which is lower case letters `a-z`, numbers `0-9`, and `-`'s only.
* `KUBE_POD_NAME` - this can be anything that uniquely identifies your pod in the namespace.  You can generate a uuid on start and use that, or you can actually use your real pod_name from your helm chart.  If omitted it will use a global var set to a UUID for this running instance.
* `KUBE_CONFIG_PATH` - you should never need to use this unless you want to use your own local kube config to run a test which will create and use real leases on the remote kubernetes cluster.  These kinds of tests should only ever be created or ran from within this library and obviously could never run in pipeline.

## Useful Methods
```go
package dsync

import "context"

type Dsync struct{}
func (Dsync) Election(context.Context, string) Election
func (Dsync) NewLock(context.Context, string) Lock

type Election struct{}
func (Election) Resource() Resource
func (Election) Err() error
func (Election) GetLeader() string
func (Election) IsLeader() bool
func (Election) WhenElected(func(context.Context))
func (Election) Stop()

type Lock struct{}
func (Lock) Resource() Resource
func (Lock) Err() error
func (Lock) Lock() error
func (Lock) LockContext(context.Context) error
func (Lock) TryLock() error
func (Lock) Unlock() error

type Resource struct {
	Namespace string // optional
	Scope     string // optional
	Name      string
}
func (Resource) String() string
func (Resource) Equal(Resource) bool
func (Resource) Validate() error
func (Resource) ElectionName() string
func (Resource) LockName() string
```

### Election
* `IsLeader() bool` returns true if you are the leader.  It is important to note that it does not wait to ensure that there is a leader before returning.  It simply returns the current state and if the election hasn't finished yet, then you are not the leader yet.
* `WhenElected(ctx, func(ctx))` will result in your function being called after every election where you transition from not being the leader, to being the leader.  The `ctx` will be canceled if at any time you stop being the leader for any reason.  So given two pods `A` and `B` and you are `A`.  If `A` is elected, then `B` is elected, then `A` elected again.  What will happen for you is your func will be called, the context will be canceled, then your func will be called again.  The context will always be canceled before a second execution can happen however it is up to you to ensure you have correctly reacted to the canceled context.

