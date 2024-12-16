package dsync_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tempcke/dsync"
)

func TestResource_String(t *testing.T) {
	var (
		a, b, c = "a", "b", "c"
		e       = "" // for empty
	)
	var tests = map[string]struct {
		in  string
		r   dsync.Resource
		out string
	}{
		"a":    {"a", dsync.NewScopedResource(e, e, a), "a"},
		"ab":   {"a/b", dsync.NewScopedResource(e, a, b), "a/b"},
		"abc":  {"a/b/c", dsync.NewScopedResource(a, b, c), "a/b/c"},
		"abcd": {"a/b/c/d", dsync.NewScopedResource(a, b, "c/d"), "a/b/c/d"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := tc.r
			r2 := dsync.ToResource(tc.in)
			assert.Equal(t, tc.out, r.String())
			assert.Equal(t, tc.out, r2.String())
			assert.True(t, r.Equal(r2))
		})
	}
}
func TestResource_Validation(t *testing.T) {
	// see comments on dsync.NamePattern
	var (
		ns     = "local"
		scope  = "testing"
		name   = randName()
		validR = dsync.NewScopedResource(ns, scope, name)
	)
	require.NoError(t, validR.Validate())
}
func randName() string {
	return uuid.NewString()[24:]
}
