package vulcand

import (
	"github.com/timelinelabs/vulcand/api"
	"github.com/timelinelabs/vulcand/engine"
	"golang.org/x/net/context"
)

type Vulcan struct {
	api.Client
	c context.Context
}

type frontend struct {
	engine.Frontend
	middlewares []*middleware
}

func newFrontend(f *engine.Frontend) *frontend {
	return &frontend{
		Frontend:    *f,
		middlewares: make([]*middleware, 0, 1),
	}
}

type backend struct {
	engine.Backend
	servers []*server
}

func newBackend(b *engine.Backend) *backend {
	return &backend{
		Backend: *b,
		servers: make([]*server, 0, 1),
	}
}

type server struct {
	engine.Server
}

func newServer(s *engine.Server) *server {
	return &server{*s}
}

type middleware struct {
	engine.Middleware
}

func newMiddleware(m *engine.Middleware) *middleware {
	return &middleware{*m}
}
