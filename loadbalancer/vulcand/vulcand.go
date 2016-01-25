package vulcand

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/url"
	"github.com/timelinelabs/vulcand/api"
	"github.com/timelinelabs/vulcand/engine"
	"github.com/timelinelabs/vulcand/plugin"
	"github.com/timelinelabs/vulcand/plugin/registry"
	vroute "github.com/vulcand/route"
	"golang.org/x/net/context"

	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

const (
	DefaultRoute = "Path(`/`)"

	FrontendSettingsKey        = "frontend_settings"
	BackendSettingsKey         = "backend_settings"
	BackendTypeKey             = "backend_type"
	PassHostHeaderKey          = "pass_host_header"
	TrustForwardHeadersKey     = "trust_forward_headers"
	FailoverExpressionKey      = "failover_expression"
	DailTimeoutKey             = "dial_timeout"
	ReadTimeoutKey             = "read_timeout"
	MaxIdleConnsKey            = "max_idle_conns_per_host"
	CustomMiddlewareKeyPattern = `^middleware\.([^\.]+)`

	websocket = "websocket"
	ws        = "ws"
	HTTP      = "http"
	Enabled   = "enabled"

	RedirectSSLID = "redirect_ssl"
	TraceID       = "trace"
	AuthID        = "auth"
	MaintenanceID = "maintenance"
)

func New(vulcanURL string, reg *plugin.Registry, ctx context.Context) (*vulcan, error) {
	if _, er := url.Parse(vulcanURL); er != nil {
		return nil, er
	}

	if reg == nil {
		reg = registry.GetRegistry()
	}
	client := api.NewClient(vulcanURL, reg)
	if client == nil {
		return nil, errors.New("Failed to create vulcand client")
	}
	return &vulcan{Client: *client, c: ctx}, nil
}

func (v *vulcan) Kind() string {
	return "vulcand"
}

func (v *vulcan) Status() error {
	return v.GetStatus()
}

func (v *vulcan) NewFrontend(rsc *kubernetes.Resource) (loadbalancer.Frontend, error) {
	s := engine.HTTPFrontendSettings{}
	if val, ok := rsc.GetAnnotation(PassHostHeaderKey); ok {
		b, _ := strconv.ParseBool(val)
		s.PassHostHeader = b
	}
	if val, ok := rsc.GetAnnotation(TrustForwardHeadersKey); ok {
		b, _ := strconv.ParseBool(val)
		s.TrustForwardHeader = b
	}
	if val, ok := rsc.GetAnnotation(FailoverExpressionKey); ok {
		s.FailoverPredicate = val
	}
	if val, ok := rsc.GetAnnotation(FrontendSettingsKey); ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for frontend %q: %v", rsc.ID, er)
		}
	}

	f, er := engine.NewHTTPFrontend(vroute.NewMux(), rsc.ID(), rsc.ID(), NewRoute(rsc.Route).String(), s)
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *vulcan) NewBackend(rsc *kubernetes.Resource) (loadbalancer.Backend, error) {
	s := engine.HTTPBackendSettings{
		Timeouts:  engine.HTTPBackendTimeouts{},
		KeepAlive: engine.HTTPBackendKeepAlive{},
	}
	if val, ok := rsc.GetAnnotation(DailTimeoutKey); ok {
		s.Timeouts.Dial = val
	}
	if val, ok := rsc.GetAnnotation(ReadTimeoutKey); ok {
		s.Timeouts.Read = val
	}
	if val, ok := rsc.GetAnnotation(MaxIdleConnsKey); ok {
		if i, er := strconv.Atoi(val); er == nil {
			s.KeepAlive.MaxIdleConnsPerHost = i
		}
	}
	if val, ok := rsc.GetAnnotation(BackendSettingsKey); ok {
		if er := json.Unmarshal([]byte(val), &s); er != nil {
			logger.Warnf("Failed to parse settings for frontend %q: %v", rsc.ID, er)
		}
	}

	b, er := engine.NewHTTPBackend(rsc.ID(), s)
	if er != nil {
		return nil, er
	}
	if rsc.IsWebsocket() {
		b.Type = ws
	}
	return newBackend(b), nil
}

func (v *vulcan) NewServers(rsc *kubernetes.Resource) ([]loadbalancer.Server, error) {
	list := make([]loadbalancer.Server, 0, 1)
	for _, server := range rsc.Servers() {
		s, er := engine.NewServer(server.ID(), server.URL().String())
		if er != nil {
			return list, er
		}
		list = append(list, newServer(s))
	}
	return list, nil
}

func (v *vulcan) NewMiddlewares(rsc *kubernetes.Resource) ([]loadbalancer.Middleware, error) {
	mids := make([]loadbalancer.Middleware, 0, 1)
	for key, def := range DefaultMiddleware {
		if val, ok := rsc.GetAnnotation(key); ok && len(val) > 0 {
			switch key {
			case RedirectSSLID:
				if b, er := strconv.ParseBool(val); er != nil || !b {
					continue
				}
			case TraceID:
				re := regexp.MustCompile(`\s+`)
				list, er := json.Marshal(strings.Split(re.ReplaceAllString(val, ""), ","))
				if er != nil || string(list) == "" {
					logger.Warnf("Unable to json-ify trace headers: %v", er)
					list = []byte("[]")
				}
				def = fmt.Sprintf(def, string(list), string(list))
			case AuthID:
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
			case MaintenanceID:
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
	matches, _ := rsc.GetAnnotations(CustomMiddlewareKeyPattern)
	for key, val := range matches {
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

func (v *vulcan) UpsertFrontend(fr loadbalancer.Frontend) error {
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

func (v *vulcan) UpsertBackend(ba loadbalancer.Backend) error {
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

func (v *vulcan) UpsertServer(backend loadbalancer.Backend, srv loadbalancer.Server) error {
	return v.Client.UpsertServer(engine.BackendKey{Id: backend.GetID()}, srv.(*server).Server, 0)
}

func (v *vulcan) GetFrontend(frontendID string) (loadbalancer.Frontend, error) {
	f, er := v.Client.GetFrontend(engine.FrontendKey{Id: frontendID})
	if er != nil {
		return nil, er
	}
	return newFrontend(f), nil
}

func (v *vulcan) GetBackend(backendID string) (loadbalancer.Backend, error) {
	logger.Debugf("Lookup Backend: %q", backendID)
	b, er := v.Client.GetBackend(engine.BackendKey{Id: backendID})
	if er != nil {
		logger.Debugf("Lookup failed: %v", er)
		return nil, er
	}
	return newBackend(b), nil
}

func (v *vulcan) GetServers(backendID string) ([]loadbalancer.Server, error) {
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

func (v *vulcan) DeleteFrontend(fr loadbalancer.Frontend) error {
	return v.Client.DeleteFrontend(engine.FrontendKey{Id: fr.GetID()})
}

func (v *vulcan) DeleteBackend(ba loadbalancer.Backend) error {
	return v.Client.DeleteBackend(engine.BackendKey{Id: ba.GetID()})
}

func (v *vulcan) DeleteServer(ba loadbalancer.Backend, srv loadbalancer.Server) error {
	return v.Client.DeleteServer(
		engine.ServerKey{BackendKey: engine.BackendKey{Id: ba.GetID()}, Id: srv.GetID()},
	)
}

func (b *backend) GetID() string             { return b.GetId() }
func (b *backend) GetKey() engine.BackendKey { return engine.BackendKey{Id: b.GetID()} }

func (b *backend) AddServer(srv loadbalancer.Server) {
	if b.servers == nil {
		b.servers = make([]*server, 0, 1)
	}
	b.servers = append(b.servers, srv.(*server))
}

func (f *frontend) GetID() string { return f.GetId() }

func (f *frontend) AddMiddleware(mid loadbalancer.Middleware) {
	if f.middlewares == nil {
		f.middlewares = make([]*middleware, 0, 1)
	}
	f.middlewares = append(f.middlewares, mid.(*middleware))
}

func (m *middleware) GetID() string { return m.Id }

func (s *server) GetID() string { return s.GetId() }
