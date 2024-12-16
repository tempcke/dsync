package drivers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tempcke/dsync/configs"
)

func TestKubeDriver(t *testing.T) {
	var (
		ns    = randString(8)
		scope = randString(8)
		task  = randString(8)
		conf  = configs.FromMap(map[string]string{
			configs.KeyNamespace:  ns,
			configs.KeyLeaseScope: scope,
		})
	)
	k := kubeDriver(t, conf)
	r := k.Resource(task)
	assert.Equal(t, ns, r.Namespace)
	assert.Equal(t, scope, r.Scope)
	assert.Equal(t, task, r.Name)
}
