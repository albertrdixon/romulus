package vulcand

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/url"
	"github.com/bradfitz/slice"
	"github.com/timelinelabs/vulcand/api"
	"github.com/timelinelabs/vulcand/engine"
	"github.com/timelinelabs/vulcand/plugin"
	"github.com/timelinelabs/vulcand/plugin/registry"
	"github.com/vulcand/route"
	"golang.org/x/net/context"

	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

func New(vulcanURL string, reg *plugin.Registry, ctx context.Context) (*Vulcan, error) {
	if ur, er := url.Parse(vulcanURL); er != nil || !validVulcanURL(ur) {
		return nil, er
	}

	if reg == nil {
		reg = registry.GetRegistry()
	}
	client := api.NewClient(vulcanURL, reg)
	if client == nil {
		return nil, errors.New("Failed to create Vulcand client")
	}
	return &Vulcan{Client: *client, c: ctx}, nil
}

func (v *Vulcan) Kind() string {
	return "vulcand"
}

func (v *Vulcan) Status() error {
	return v.GetStatus()
}

func (v *Vulcan) NewFrontend(svc *kubernetes.Service) (loadbalancer.Frontend, error) {
	s := engine.HTTPFrontendSettings{}
	if val, ok := svc.GetAnnotation(frontendSettingsKey); ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for frontend %q: %v", svc.ID, er)
		}
	}

	f, er := engine.NewHTTPFrontend(route.NewMux(), svc.ID, svc.ID, buildRoute(svc.Route), s)
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *Vulcan) NewBackend(svc *kubernetes.Service) (loadbalancer.Backend, error) {
	s := engine.HTTPBackendSettings{}
	if val, ok := svc.GetAnnotation(backendSettingsKey); ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for frontend %q: %v", svc.ID, er)
		}
	}

	b, er := engine.NewHTTPBackend(svc.ID, s)
	if er != nil {
		return nil, er
	}
	if kind, ok := svc.GetAnnotation(backendTypeKey); ok {
		if kind == websocket || kind == ws {
			b.Type = ws
		}
	}
	return newBackend(b), nil
}

func (v *Vulcan) NewServers(svc *kubernetes.Service) ([]loadbalancer.Server, error) {
	list := make([]loadbalancer.Server, 0, 1)
	for _, server := range svc.Backends {
		s, er := engine.NewServer(server.ID, server.URL().String())
		if er != nil {
			return list, er
		}
		list = append(list, newServer(s))
	}
	return list, nil
}

func (v *Vulcan) NewMiddlewares(svc *kubernetes.Service) ([]loadbalancer.Middleware, error) {
	mids := make([]loadbalancer.Middleware, 0, 1)
	for key, def := range DefaultMiddleware {
		if val, ok := svc.GetAnnotation(key); ok && len(val) > 0 {
			switch key {
			case "trace":
				re := regexp.MustCompile(`\s+`)
				list, er := json.Marshal(strings.Split(re.ReplaceAllString(val, ""), ","))
				if er != nil {
					logger.Warnf("Unable to json-ify trace headers: %v", er)
					list = []byte("[]")
				}
				def = fmt.Sprintf(def, string(list), string(list))
			case "auth":
				bits := strings.SplitN(val, ":", 2)
				switch len(bits) {
				case 1:
					def = fmt.Sprintf(def, bits[0], "")
				case 2:
					def = fmt.Sprintf(def, bits[0], bits[1])
				default:
					logger.Errorf("Failed to parse provided basic auth, using default (admin:admin)")
					def = fmt.Sprintf(def, "admin", "admin")
				}
			case "maintenance":
				def = fmt.Sprintf(def, val)
			}

			m, er := engine.MiddlewareFromJSON([]byte(def), v.Registry.GetSpec, key)
			if er != nil {
				logger.Warnf("Failed to parse Middleware %s: %v", key, er)
				logger.Debugf("%q", def)
				continue
			}
			mids = append(mids, newMiddleware(m))
		}
	}

	rg := regexp.MustCompile(CustomMiddlewareKeyPattern)
	for key, val := range svc.Annotations {
		if match := rg.FindStringSubmatch(key); match != nil {
			id := match[1]
			m, er := engine.MiddlewareFromJSON([]byte(val), v.Registry.GetSpec, id)
			if er != nil {
				logger.Warnf("Failed to parse Middleware %s: %v", id, er)
				continue
			}
			mids = append(mids, newMiddleware(m))
		}
	}

	return mids, nil
}

func (v *Vulcan) UpsertFrontend(fr loadbalancer.Frontend) error {
	f, ok := fr.(*frontend)
	if !ok {
		return loadbalancer.ErrUnexpectedFrontendType
	}
	if er := v.Client.UpsertFrontend(f.Frontend, 0); er != nil {
		return er
	}
	for _, mid := range f.middlewares {
		if er := v.UpsertMiddleware(f.GetKey(), mid.Middleware, 0); er != nil {
			logger.Warnf("Failed to upsert Middleware %s for frontend %s: %v", mid.GetID(), f.GetID(), er)
		}
	}
	return nil
}

func (v *Vulcan) UpsertBackend(ba loadbalancer.Backend) error {
	b, ok := ba.(*backend)
	if !ok {
		return loadbalancer.ErrUnexpectedBackendType
	}
	if er := v.Client.UpsertBackend(b.Backend); er != nil {
		return er
	}

	extra := make(map[string]loadbalancer.Server)
	ss, _ := v.Client.GetServers(engine.BackendKey{Id: b.GetID()})
	for i := range ss {
		extra[ss[i].GetId()] = &server{ss[i]}
	}
	for _, srv := range b.servers {
		logger.Infof("Upserting %v", srv)
		if er := v.UpsertServer(b, srv); er != nil {
			return er
		}
		delete(extra, srv.GetID())
	}
	for _, srv := range extra {
		logger.Infof("Removing %v", srv)
		v.DeleteServer(b, srv)
	}
	return nil
}

func (v *Vulcan) UpsertServer(backend loadbalancer.Backend, srv loadbalancer.Server) error {
	return v.Client.UpsertServer(engine.BackendKey{Id: backend.GetID()}, srv.(*server).Server, 0)
}

func (v *Vulcan) GetFrontend(frontendID string) (loadbalancer.Frontend, error) {
	f, er := v.Client.GetFrontend(engine.FrontendKey{Id: frontendID})
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *Vulcan) GetBackend(backendID string) (loadbalancer.Backend, error) {
	logger.Debugf("Lookup Backend: %q", backendID)
	b, er := v.Client.GetBackend(engine.BackendKey{Id: backendID})
	if er != nil {
		logger.Debugf("Lookup failed: %v", er)
		return nil, er
	}
	return newBackend(b), nil
}

func (v *Vulcan) GetServers(backendID string) ([]loadbalancer.Server, error) {
	srvs, er := v.Client.GetServers(engine.BackendKey{Id: backendID})
	if er != nil {
		return []loadbalancer.Server{}, er
	}

	servers := make([]loadbalancer.Server, 0, len(srvs))
	for _, s := range srvs {
		servers = append(servers, newServer(&s))
	}
	return servers, nil
}

func (v *Vulcan) DeleteFrontend(fr loadbalancer.Frontend) error {
	return v.Client.DeleteFrontend(engine.FrontendKey{Id: fr.GetID()})
}

func (v *Vulcan) DeleteBackend(ba loadbalancer.Backend) error {
	return v.Client.DeleteBackend(engine.BackendKey{Id: ba.GetID()})
}

func (v *Vulcan) DeleteServer(ba loadbalancer.Backend, srv loadbalancer.Server) error {
	return v.Client.DeleteServer(
		engine.ServerKey{BackendKey: engine.BackendKey{Id: ba.GetID()}, Id: srv.GetID()},
	)
}

func newBackend(b *engine.Backend) *backend {
	return &backend{
		Backend: *b,
		servers: make([]*server, 0, 1),
	}
}

func (b *backend) GetID() string             { return b.GetId() }
func (b *backend) GetKey() engine.BackendKey { return engine.BackendKey{Id: b.GetID()} }

func (b *backend) AddServer(srv loadbalancer.Server) {
	b.servers = append(b.servers, srv.(*server))
}

func newFrontend(f *engine.Frontend) *frontend {
	return &frontend{
		Frontend:    *f,
		middlewares: make([]*middleware, 0, 1),
	}
}

func (f *frontend) GetID() string { return f.GetId() }

func (f *frontend) AddMiddleware(mid loadbalancer.Middleware) {
	f.middlewares = append(f.middlewares, mid.(*middleware))
}

func newMiddleware(m *engine.Middleware) *middleware {
	return &middleware{*m}
}

func (m *middleware) GetID() string { return m.Id }

func newServer(s *engine.Server) *server {
	return &server{*s}
}

func (s *server) GetID() string { return s.GetId() }

type Vulcan struct {
	api.Client
	c context.Context
}

type frontend struct {
	engine.Frontend
	middlewares []*middleware
}

type backend struct {
	engine.Backend
	servers []*server
}

type server struct {
	engine.Server
}

type middleware struct {
	engine.Middleware
}

const (
	DefaultRoute = "Path(`/`)"

	frontendSettingsKey        = "frontend_settings"
	backendSettingsKey         = "backend_settings"
	backendTypeKey             = "backend_type"
	CustomMiddlewareKeyPattern = `^romulus/middleware\.([^\.]+)`

	websocket = "websocket"
	ws        = "ws"
	HTTP      = "http"
	Enabled   = "enabled"
)

func buildRoute(rt *kubernetes.Route) string {
	if rt.Empty() {
		return DefaultRoute
	}

	bits := []string{}
	for k, v := range rt.GetParts() {
		bits = append(bits, fmt.Sprintf("%s(`%s`)", strings.Title(k), v))
	}
	for k, v := range rt.GetHeader() {
		bits = append(bits, fmt.Sprintf("Header(`%s`, `%s`)", k, v))
	}
	slice.Sort(bits, func(i, j int) bool {
		return bits[i] < bits[j]
	})
	expr := strings.Join(bits, " && ")
	if len(expr) < 1 || !route.IsValid(expr) {
		logger.Debugf("Provided route not valid: %s", expr)
		return DefaultRoute
	}
	return expr
}

func isRegexp(r string) bool {
	if !strings.HasPrefix(r, "/") || !strings.HasSuffix(r, "/") {
		return false
	}
	if _, er := regexp.Compile(r); er != nil {
		logger.Debugf("Regexp compile failure: %v", er)
		return false
	}
	return true
}

func validVulcanURL(u *url.URL) bool {
	return true
	// return len(u.GetHost()) > 0 && len(u.Scheme) > 0 && len(u.Path) > 0
}
