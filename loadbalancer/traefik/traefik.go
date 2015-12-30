package traefik

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

var (
	defaultTraefikRoute = map[string]types.Route{
		"default": types.Route{Rule: "Path", Value: "/"},
	}
)

func (t *traefik) Kind() string {
	return "traefik"
}

func (t *traefik) Status() error {
	_, er := t.client.Keys("/")
	return er
}

func (t *traefik) NewFrontend(svc *kubernetes.Service) (loadbalancer.Frontend, error) { return nil, nil }
func (t *traefik) GetFrontend(id string) (loadbalancer.Frontend, error)               { return nil, nil }
func (t *traefik) UpsertFrontend(fr loadbalancer.Frontend) error                      { return nil }
func (t *traefik) DeleteFrontend(fr loadbalancer.Frontend) error                      { return nil }
func (t *traefik) NewBackend(svc *kubernetes.Service) (loadbalancer.Backend, error)   { return nil, nil }
func (t *traefik) GetBackend(id string) (loadbalancer.Backend, error)                 { return nil, nil }
func (t *traefik) UpsertBackend(ba loadbalancer.Backend) error                        { return nil }
func (t *traefik) DeleteBackend(ba loadbalancer.Backend) error                        { return nil }
func (t *traefik) NewServers(svc *kubernetes.Service) ([]loadbalancer.Server, error) {
	return []loadbalancer.Server{}, nil
}
func (t *traefik) GetServers(id string) ([]loadbalancer.Server, error) {
	return []loadbalancer.Server{}, nil
}
func (t *traefik) UpsertServer(ba loadbalancer.Backend, srv loadbalancer.Server) error { return nil }
func (t *traefik) DeleteServer(ba loadbalancer.Backend, srv loadbalancer.Server) error { return nil }
func (t *traefik) NewMiddlewares(svc *kubernetes.Service) ([]loadbalancer.Middleware, error) {
	return []loadbalancer.Middleware{}, nil
}

func (m *middleware) GetID() string {
	return m.id
}

func (f *frontend) GetID() string {
	return f.id
}

func (b *backend) GetID() string {
	return b.id
}

func (s *server) GetID() string {
	return s.id
}

func (f *frontend) AddMiddleware(mid loadbalancer.Middleware) {
	if f.middlewares == nil {
		f.middlewares = make([]*middleware, 0, 1)
	}
	f.middlewares = append(f.middlewares, mid.(*middleware))
}

func (b *backend) AddServer(srv loadbalancer.Server) {
	if b.servers == nil {
		b.servers = make([]*server, 0, 1)
	}
	b.servers = append(b.servers, srv.(*server))
}
