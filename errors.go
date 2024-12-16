package dsync

import (
	"errors"
)

var (
	ErrBadResourceName = errors.New("bad resource scope or name")
	ErrInvalidState    = errors.New("invalid state")
	ErrLockNotFound    = errors.New("lock not found")

	ErrNoLeader      = errors.New("no leader right now")
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyLocked = errors.New("resource is already locked")
	ErrNotLockHolder = errors.New("resource locked by someone else")
	ErrNotLocked     = errors.New("resource not locked")
)
