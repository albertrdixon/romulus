package traefik

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timelinelabs/romulus/loadbalancer"
)

func TestInterface(t *testing.T) {
	assert.Implements(t, (*loadbalancer.LoadBalancer)(nil), new(traefik))
	assert.Implements(t, (*loadbalancer.Frontend)(nil), new(frontend))
	assert.Implements(t, (*loadbalancer.Backend)(nil), new(backend))
	assert.Implements(t, (*loadbalancer.Server)(nil), new(server))
	assert.Implements(t, (*loadbalancer.Middleware)(nil), new(middleware))
}
