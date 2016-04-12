package traefik

import (
	"fmt"
	"strings"

	"github.com/albertrdixon/gearbox/ezd"
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/loadbalancer"
	"golang.org/x/net/context"
)

type traefik struct {
	ezd.Client
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

// type route struct {
// 	host, path, prefix, method types.Route
// 	header, headerRegex        types.Route
// }

type route map[string]types.Route

func (f *frontend) GetID() string   { return strings.Replace(f.id, ".", "-", -1) }
func (b *backend) GetID() string    { return strings.Replace(b.id, ".", "-", -1) }
func (m *middleware) GetID() string { return strings.Replace(m.id, ".", "-", -1) }
func (s *server) GetID() string     { return strings.Replace(s.id, ".", "-", -1) }

func (f *frontend) String() string {
	return fmt.Sprintf("Frontend(id=%q, backend=%q)", f.id, f.Backend)
}

func (b *backend) String() string {
	srvs := make([]string, 0, len(b.Servers))
	for _, s := range b.Servers {
		srvs = append(srvs, s.URL)
	}
	return fmt.Sprintf("Backend(id=%q, servers=%v)", b.id, srvs)
}

func (s *server) String() string {
	return fmt.Sprintf("Server(id=%q, url=%s)", s.id, s.URL)
}

func (f *frontend) AddMiddleware(m loadbalancer.Middleware) {
	if f.middlewares == nil {
		f.middlewares = make([]*middleware, 0, 1)
	}
	f.middlewares = append(f.middlewares, m.(*middleware))
}

func (b *backend) AddServer(s loadbalancer.Server) {
	b.Servers[s.GetID()] = s.(*server).Server
}
