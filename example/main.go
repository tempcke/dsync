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
	keepTokenFresh(ctx, d)

	// lock example, useful for job queue processing
	accountID := envFn("ACCOUNT_ID")
	err = checkAccount(ctx, d, accountID)
	if err != nil {
		panic(err)
	}
}

func keepTokenFresh(ctx context.Context, d dsync.Interface) {
	d.Election(ctx, TaskManageAccessToken).WhenElected(func(ctx context.Context) {
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
}
func checkAccount(ctx context.Context, d dsync.Interface, aID string) error {
	var (
		lock = d.NewLock(ctx, "check-account-"+aID)
	)

	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	// check the account
	return nil
}
