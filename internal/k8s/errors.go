package k8s

import (
	"errors"

	"github.com/tempcke/dsync"
)

var (
	ErrNoLeader        = dsync.ErrNoLeader
	ErrNotFound        = dsync.ErrNotFound
	ErrAlreadyLocked   = dsync.ErrAlreadyLocked
	ErrLeaseNotFound   = errors.New("resource lease not found")
	ErrUpdateLeaseFail = errors.New("resource lease update failed")
	ErrCreateLeaseFail = errors.New("resource lease create failed")
	ErrNotLockHolder   = dsync.ErrNotLockHolder
	ErrNotLocked       = dsync.ErrNotLocked
)
