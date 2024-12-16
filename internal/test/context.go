package test

import (
	"context"
	"testing"
	"time"

	"github.com/tempcke/dsync/internal/signaler"
)

func Context(t testing.TB, d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return signaler.Context(ctx)
}
