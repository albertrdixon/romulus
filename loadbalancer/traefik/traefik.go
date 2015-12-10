package traefik

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/pkg/store"
	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
	"golang.org/x/net/context"
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

func (t *traefik) NewFrontend(svc *kubernetes.Service) (Frontend, error) { return nil, nil }
func (t *traefik) GetFrontend(svc *kubernetes.Service) (Frontend, error) { return nil, nil }
func (t *traefik) UpsertFrontend(fr loadbalancer.Frontend) error         { return nil }
func (t *traefik) DeleteFrontend(fr loadbalancer.Frontend) error         { return nil }
func (t *traefik) NewBackend(svc *kubernetes.Service) (Backend, error)   { return nil, nil }
func (t *traefik) GetBackend(svc *kubernetes.Service) (Backend, error)   { return nil, nil }
func (t *traefik) UpsertBackend(ba loadbalancer.Backend) error           { return nil }
func (t *traefik) DeleteBackend(ba loadbalancer.Backend) error           { return nil }
func (t *traefik) NewServers(addr Addresses, svc *kubernetes.Service) ([]Server, error) {
	return []loadbalancer.Server{}, nil
}
func (t *traefik) GetServers(svc *kubernetes.Service) ([]Server, error) {
	return []loadbalancer.Server{}, nil
}
func (t *traefik) UpsertServer(ba loadbalancer.Backend, srv Server) error { return nil }
func (t *traefik) DeleteServer(ba loadbalancer.Backend, srv Server) error { return nil }
func (t *traefik) NewMiddlewares(svc *kubernetes.Service) ([]Middleware, error) {
	return []loadbalancer.Middleware{}, nil
}

type traefik struct {
	client store.Store
	c      context.Context
}

type frontend struct {
	types.Frontend
}

type backend struct {
	types.Backend
}

type server struct {
	types.Server
}

func buildTraefikRoute(rt kubernetes.Route) map[string]types.Route {
	if rt.Empty() {
		return defaultTraefikRoute
	}

	routes := map[string]types.Route{}
	if rt.Host != "" {
		routes["host"] = types.Route{Rule: "Host", Value: rt.Host}
	}
	if rt.Path != "" {
		routes["path"] = types.Route{Rule: "Path", Value: rt.Path}
	}
	return routes
}
