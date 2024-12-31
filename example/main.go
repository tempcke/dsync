package main

import (
	"context"
	"os"
	"time"

	"github.com/tempcke/dsync"
	"github.com/tempcke/dsync/configs"
	"github.com/tempcke/dsync/drivers"
)

const (
	TaskManageAccessToken = "manage-access-token"
)

func main() {
	var (
		ctx   = context.Background()
		envFn = os.Getenv // could use viper.GetString
	)

	// construct driver
	conf := configs.WithDefaults(envFn, configs.Defaults)
	driver, err := drivers.NewKubeDriver(conf)
	if err != nil {
		panic(err)
	}

	// construct dsync with driver
	d := dsync.New(driver, conf(configs.KeyPodName))

	// leader election example
	if err := keepTokenFresh(ctx, d); err != nil {
		panic(err)
	}

	// lock example, useful for job queue processing
	if err := checkAccount(ctx, d, envFn("ACCOUNT_ID")); err != nil {
		panic(err)
	}

	// do with lock example
	err = d.NewLock(ctx, "lockName").DoWithLock(ctx, func() error {
		// in here we have the lock, after return it is unlocked for us
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func keepTokenFresh(ctx context.Context, d dsync.Interface) error {
	e, err := d.NewElection(ctx, TaskManageAccessToken)
	if err != nil {
		return err
	}
	e.WhenElected(func(ctx context.Context) {
		var (
			wait     time.Duration // no wait first time
			interval = time.Minute
		)

		for {
			select {
			case <-ctx.Done():
				// exit when no longer elected, this func will re-run if elected again
				return
			case <-time.After(wait):
				wait = interval
				// refresh the token
			}
		}
	})
	return nil
}
func checkAccount(ctx context.Context, d dsync.Interface, aID string) error {
	lock := d.NewLock(ctx, "check-account-"+aID)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	// check the account
	return nil
}
