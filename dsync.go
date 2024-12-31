package dsync

import (
	"context"
	"regexp"
	"strings"

	"github.com/tempcke/dsync/configs"
)

type (
	PodID        = string
	LockID       = string
	ElectionID   = string
	ResourceName = string
	Dsync        struct {
		driver   Driver
		instance string // recommend you use pod name
	}
	Driver interface {
		Resource(string) Resource
		GetLock(context.Context, ResourceName, PodID) (LockDriver, error)
		GetElection(context.Context, ResourceName, PodID) (ElectionDriver, error)
	}
	Logger interface { // slog.Logger
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}
	Interface interface {
		NewElection(context.Context, string) (Election, error)
		NewLock(context.Context, string) Lock
	}
)

const (
	DefaultNamespace = configs.DefNamespace
	DefaultScope     = configs.DefLeaseScope
	MasterTask       = "master"
)

func New(driver Driver, instance string) Dsync {
	return Dsync{
		driver:   driver,
		instance: instance,
	}
}
func (d Dsync) Resource(name string) Resource {
	return d.driver.Resource(d.SanitizeName(name))
}

// NewElection will return an Election if there is an error or not
// the returned election will simply no-op in case of an error
func (d Dsync) NewElection(ctx context.Context, task string) (Election, error) {
	e := NewElection(ctx, d.driver, task, d.instance)
	return e, e.Err()
}

// NewLock constructs and returns a Lock
// if there is any error with the Lock itself then the methods on Lock will return them
// therefore no need to return the error here if you want to check the error yourself use Lock.Err()
func (d Dsync) NewLock(ctx context.Context, name string) Lock {
	name = d.SanitizeName(name)
	return NewLock(ctx, d.driver, name, d.instance)
}

// SanitizeName returns a valid resource name the best it can
// upper case letters will be made lower case
// special chars will be converted to dashes
// and dashes will be trimmed from start and end
func (d Dsync) SanitizeName(input string) string {
	return SanitizeName(input)
}

var sanitizeNameRE = regexp.MustCompile(`[^a-z0-9-]`)

func SanitizeName(in string) string {
	r := sanitizeNameRE
	d := "-"
	// Replace invalid characters with dashes (anything not a-z, 0-9, or '-')
	// Trim dashes from the start and end
	return strings.Trim(r.ReplaceAllString(strings.ToLower(in), d), d)
}
