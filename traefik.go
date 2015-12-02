package main

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/pkg/store"
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

func (t *traefik) NewFrontend(meta *Metadata) (Frontend, error)                { return nil, nil }
func (t *traefik) GetFrontend(meta *Metadata) (Frontend, error)                { return nil, nil }
func (t *traefik) UpsertFrontend(fr Frontend) error                            { return nil }
func (t *traefik) DeleteFrontend(fr Frontend) error                            { return nil }
func (t *traefik) NewBackend(meta *Metadata) (Backend, error)                  { return nil, nil }
func (t *traefik) GetBackend(meta *Metadata) (Backend, error)                  { return nil, nil }
func (t *traefik) UpsertBackend(ba Backend) error                              { return nil }
func (t *traefik) DeleteBackend(ba Backend) error                              { return nil }
func (t *traefik) NewServers(addr Addresses, meta *Metadata) ([]Server, error) { return []Server{}, nil }
func (t *traefik) GetServers(meta *Metadata) ([]Server, error)                 { return []Server{}, nil }
func (t *traefik) UpsertServer(ba Backend, srv Server) error                   { return nil }
func (t *traefik) DeleteServer(ba Backend, srv Server) error                   { return nil }
func (t *traefik) NewMiddlewares(meta *Metadata) ([]Middleware, error)         { return []Middleware{}, nil }

type traefik struct {
	client store.Store
	c      context.Context
}

type tFrontend struct {
	types.Frontend
}

type tBackend struct {
	types.Backend
}

type tServer struct {
	types.Server
}

func buildTraefikRoute(meta *Metadata) map[string]types.Route {
	if meta.Annotations == nil || len(meta.Annotations) < 1 {
		return defaultTraefikRoute
	}

	routes := map[string]types.Route{}
	return routes
}
