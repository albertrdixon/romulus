package traefik

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/pkg/store"
	"golang.org/x/net/context"
)

type traefik struct {
	client store.Store
	c      context.Context
}

type frontend struct {
	types.Frontend
	id          string
	middlewares []*middleware
}

type backend struct {
	types.Backend
	id      string
	servers []*server
}

type server struct {
	types.Server
	id string
}

type middleware struct {
	id string
}
