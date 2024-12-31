package dsync_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tempcke/dsync"
)

func TestDsync(t *testing.T) {
	// please note that most of the methods depend on a driver
	// so those are all tested in drivers/spec_test.go

	t.Run("SanitizeName", func(t *testing.T) {
		ds := new(dsync.Dsync)
		tests := map[string]string{
			"Example-Name":       "example-name",  // Mixed case
			"--Invalid@Name!!--": "invalid-name",  // Special chars and leading/trailing dashes
			"valid-name":         "valid-name",    // Already valid
			"123--456":           "123--456",      // Numbers and dashes
			"UPPERCASE":          "uppercase",     // All uppercase
			"--dash-at-start":    "dash-at-start", // Dash at start
			"dash-at-end--":      "dash-at-end",   // Dash at end
		}

		for in, want := range tests {
			name := ds.SanitizeName(in)
			assert.Equal(t, want, name)
			r := dsync.NewResource(name)
			assert.NoError(t, r.Validate())
		}
	})
}
