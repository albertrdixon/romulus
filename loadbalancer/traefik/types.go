package traefik

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/loadbalancer"
	"golang.org/x/net/context"
)

type traefik struct {
	Store
	context.Context
	prefix string
}

type frontend struct {
	types.Frontend
	id          string
	middlewares []*middleware
}

type backend struct {
	types.Backend
	id string
}

type server struct {
	types.Server
	id string
}

type middleware struct {
	id string
}

func (f *frontend) GetID() string   { return f.id }
func (b *backend) GetID() string    { return b.id }
func (m *middleware) GetID() string { return m.id }
func (s *server) GetID() string     { return s.id }

func (f *frontend) AddMiddleware(m loadbalancer.Middleware) {
	if f.middlewares == nil {
		f.middlewares = make([]*middleware, 0, 1)
	}
	f.middlewares = append(f.middlewares, m.(*middleware))
}

func (b *backend) AddServer(s loadbalancer.Server) {
	b.Servers[s.GetID()] = s.(*server).Server
}
