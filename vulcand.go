package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/url"
	"github.com/timelinelabs/vulcand/api"
	"github.com/timelinelabs/vulcand/engine"
	"github.com/timelinelabs/vulcand/plugin"
	"github.com/timelinelabs/vulcand/plugin/registry"
	"github.com/vulcand/oxy/utils"
	"github.com/vulcand/route"
	"golang.org/x/net/context"
)

var (
	RouteMatchers     = []string{"host", "path", "method", "header"}
	DefaultMiddleware = map[string]string{
		"forceSSL": `{
      "Priority": 1,
      "Type": "rewrite",
      "Middleware": {
        "Regexp": "^http://(.*)",
        "Replacement": "https://$1",
        "Rewritebody": false,
        "Redirect": true
      }
    }`,
		"trace": `{
      "Priority": 1,
      "Type": "trace",
      "Middleware": {
        "ReqHeaders": %s,
        "RespHeaders": %s,
        "Addr": "syslog://127.0.0.1:514",
        "Prefix": "@app"
      }
    }`,
		"auth": `{
    	"Priority": 1,
    	"Type": "auth",
    	"Middleware": {
    		"User": "%s",
    		"Pass": "%s"
    	}
    }`,
	}
	defaultBasicAuth = &utils.BasicAuth{Username: "admin", Password: "admin"}
)

func newVulcanLB(vulcanURL string, reg *plugin.Registry, ctx context.Context) (*vulcan, error) {
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
	return &vulcan{Client: *client, c: ctx}, nil
}

func (v *vulcan) Kind() string {
	return "vulcand"
}

func (v *vulcan) Status() error {
	return v.GetStatus()
}

func (v *vulcan) NewFrontend(meta *Metadata) (Frontend, error) {
	id := GetID(meta)

	s := engine.HTTPFrontendSettings{}
	if val, ok := meta.Annotations[AnnotationsKeyf(FrontendSettingsKey)]; ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for Frontend %q: %v", id, er)
		}
	}

	f, er := engine.NewHTTPFrontend(route.NewMux(), id, id, buildVulcanRoute(meta), s)
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *vulcan) NewBackend(meta *Metadata) (Backend, error) {
	id := GetID(meta)

	s := engine.HTTPBackendSettings{}
	if val, ok := meta.Annotations[AnnotationsKeyf(BackendSettingsKey)]; ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for Frontend %q: %v", id, er)
		}
	}

	b, er := engine.NewHTTPBackend(id, s)
	if er != nil {
		return nil, er
	}
	if kind, ok := meta.Annotations[AnnotationsKeyf(BackendTypeKey)]; ok {
		if kind == websocket || kind == WS {
			b.Type = WS
		}
	}
	return newBackend(b), nil
}

func (v *vulcan) NewServers(addr Addresses, meta *Metadata) ([]Server, error) {
	sr := make([]Server, 0, 1)
	for _, urls := range addr {
		for _, url := range urls {
			s, er := engine.NewServer(GetSrvID(url, meta), url.String())
			if er != nil {
				return sr, er
			}
			sr = append(sr, newServer(s))
		}
	}
	return sr, nil
}

func (v *vulcan) NewMiddlewares(meta *Metadata) ([]Middleware, error) {
	mids := make([]Middleware, 0, 1)
	for key, def := range DefaultMiddleware {
		if val, ok := meta.Annotations[AnnotationsKeyf(key)]; ok && len(val) > 0 {
			switch key {
			case "trace":
				h := meta.Annotations[AnnotationsKeyf("traceHeaders")]
				if len(h) < 1 {
					h = "[]"
				}
				def = fmt.Sprintf(def, h, h)
			case "auth":
				bits := strings.SplitN(val, ":", 2)
				switch len(bits) {
				case 1:
					def = fmt.Sprintf(def, bits[0], "")
				case 2:
					def = fmt.Sprintf(def, bits[0], bits[1])
				default:
					logger.Errorf("Failed to parse provided basic auth, using default (admin:admin)")
					def = fmt.Sprintf(def, defaultBasicAuth.Username, defaultBasicAuth.Password)
				}
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
	for key, val := range meta.Annotations {
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

func (v *vulcan) UpsertFrontend(fr Frontend) error {
	f, ok := fr.(*vFrontend)
	if !ok {
		return ErrUnexpectedFrontendType
	}
	if er := v.Client.UpsertFrontend(f.Frontend, 0); er != nil {
		return er
	}
	for _, mid := range f.middlewares {
		logger.Debugf("[%v] Upserting %v", fr, mid)
		if er := v.UpsertMiddleware(f.GetKey(), mid.Middleware, 0); er != nil {
			logger.Warnf("Failed to upsert Middleware %s for Frontend %s: %v", mid.GetID(), f.GetID(), er)
		}
	}
	return nil
}

func (v *vulcan) UpsertBackend(ba Backend) error {
	b, ok := ba.(*vBackend)
	if !ok {
		return ErrUnexpectedBackendType
	}
	if er := v.Client.UpsertBackend(b.Backend); er != nil {
		return er
	}

	extra := make(map[string]Server)
	ss, _ := v.Client.GetServers(engine.BackendKey{Id: b.GetID()})
	for i := range ss {
		extra[ss[i].GetId()] = &vServer{ss[i]}
	}
	for _, srv := range b.servers {
		logger.Debugf("[%v] Upserting %v", ba, srv)
		if er := v.UpsertServer(b, srv); er != nil {
			return er
		}
		delete(extra, srv.GetID())
	}
	for _, srv := range extra {
		logger.Debugf("Removing %v", srv)
		v.DeleteServer(b, srv)
	}
	return nil
}

func (v *vulcan) UpsertServer(backend Backend, srv Server) error {
	return v.Client.UpsertServer(engine.BackendKey{Id: backend.GetID()}, srv.(*vServer).Server, 0)
}

func (v *vulcan) GetFrontend(meta *Metadata) (Frontend, error) {
	id := GetID(meta)
	f, er := v.Client.GetFrontend(engine.FrontendKey{Id: id})
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *vulcan) GetBackend(meta *Metadata) (Backend, error) {
	id := GetID(meta)
	b, er := v.Client.GetBackend(engine.BackendKey{Id: id})
	if er != nil {
		return nil, er
	}
	return newBackend(b), nil
}

func (v *vulcan) GetServers(meta *Metadata) ([]Server, error) {
	id := GetID(meta)
	srvs, er := v.Client.GetServers(engine.BackendKey{Id: id})
	if er != nil {
		return []Server{}, er
	}

	servers := make([]Server, 0, len(srvs))
	for _, s := range srvs {
		servers = append(servers, newServer(&s))
	}
	return servers, nil
}

func (v *vulcan) DeleteFrontend(fr Frontend) error {
	return v.Client.DeleteFrontend(engine.FrontendKey{Id: fr.GetID()})
}

func (v *vulcan) DeleteBackend(ba Backend) error {
	return v.Client.DeleteBackend(engine.BackendKey{Id: ba.GetID()})
}

func (v *vulcan) DeleteServer(ba Backend, srv Server) error {
	return v.Client.DeleteServer(
		engine.ServerKey{BackendKey: engine.BackendKey{Id: ba.GetID()}, Id: srv.GetID()},
	)
}

func newBackend(b *engine.Backend) *vBackend {
	return &vBackend{
		Backend: *b,
		servers: make([]*vServer, 0, 1),
	}
}

func (b *vBackend) GetID() string             { return b.GetId() }
func (b *vBackend) GetKey() engine.BackendKey { return engine.BackendKey{Id: b.GetID()} }

func (b *vBackend) AddServer(srv Server) {
	b.servers = append(b.servers, srv.(*vServer))
}

func newFrontend(f *engine.Frontend) *vFrontend {
	return &vFrontend{
		Frontend:    *f,
		middlewares: make([]*vMiddleware, 0, 1),
	}
}

func (f *vFrontend) GetID() string { return f.GetId() }

func (f *vFrontend) AddMiddleware(mid Middleware) {
	f.middlewares = append(f.middlewares, mid.(*vMiddleware))
}

func newMiddleware(m *engine.Middleware) *vMiddleware {
	return &vMiddleware{*m}
}

func (m *vMiddleware) GetID() string { return m.Id }

func newServer(s *engine.Server) *vServer {
	return &vServer{*s}
}

func (s *vServer) GetID() string { return s.GetId() }

type vulcan struct {
	api.Client
	c context.Context
}

type vFrontend struct {
	engine.Frontend
	middlewares []*vMiddleware
}

type vBackend struct {
	engine.Backend
	servers []*vServer
}

type vServer struct {
	engine.Server
}

type vMiddleware struct {
	engine.Middleware
}

const (
	DefaultVulcanRoute = "Path(`/`)"

	FrontendSettingsKey        = "frontendSettings"
	BackendSettingsKey         = "backendSettings"
	BackendTypeKey             = "backendType"
	CustomMiddlewareKeyPattern = `^romulus/middleware\.([^\.]+)`

	websocket = "websocket"
	WS        = "ws"
	HTTP      = "http"
	Enabled   = "enabled"
)

func buildVulcanRoute(meta *Metadata) string {
	if meta.Annotations == nil || len(meta.Annotations) < 1 {
		return DefaultVulcanRoute
	}

	bits := []string{}
	for _, matcher := range RouteMatchers {
		key := LabelKeyf(matcher)
		if val, ok := meta.Annotations[key]; ok {
			bit := fmt.Sprintf("%s(`%s`)", strings.Title(matcher), val)
			bits = append(bits, bit)
		}
		regexMatcher := matcher + "Regexp"
		key = LabelKeyf(regexMatcher)
		if val, ok := meta.Annotations[key]; ok {
			bit := fmt.Sprintf("%s(`%s`)", strings.Title(matcher), val)
			bits = append(bits, bit)
		}
	}
	expr := strings.Join(bits, " && ")
	if len(expr) < 1 || !route.IsValid(expr) {
		return DefaultVulcanRoute
	}
	return expr
}

func validVulcanURL(u *url.URL) bool {
	return true
	// return len(u.GetHost()) > 0 && len(u.Scheme) > 0 && len(u.Path) > 0
}
