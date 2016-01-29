package traefik

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/albertrdixon/gearbox/ezd"
	"github.com/albertrdixon/gearbox/logger"
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

var (
	defaultTraefikRoute = map[string]types.Route{
		"default": types.Route{Rule: "Path", Value: "/"},
	}

	defaultCircuitBreaker     = &types.CircuitBreaker{Expression: `NetworkErrorRatio() > 0.6`}
	defaultLoadBalancerMethod = &types.LoadBalancer{Method: wrr}
)

const (
	DefaultPrefix          = "/traefik"
	LoadbalancingMethodKey = "loadbalancer_method"

	cb  = "circuitbreaker"
	lb  = "loadbalancer"
	phh = "passHostHeader"

	wrr, drr = "wrr", "drr"
)

func New(prefix string, peers []string, timeout time.Duration, ctx context.Context) (*traefik, error) {
	ec, er := ezd.New(peers, timeout)
	if er != nil {
		return nil, er
	}
	t := &traefik{
		prefix:  prefix,
		Context: ctx,
		Client:  ec,
	}
	t.Mkdir(path.Join(prefix, "frontends"))
	t.Mkdir(path.Join(prefix, "backends"))
	return t, nil
}

func (t *traefik) Kind() string {
	return "traefik"
}

func (t *traefik) Status() error {
	return t.Exists("/")
}

func (t *traefik) NewFrontend(rsc *kubernetes.Resource) (loadbalancer.Frontend, error) {
	f := types.Frontend{Backend: rsc.ID(), PassHostHeader: false}
	f.Routes = NewRoute(rsc.Route)
	if phh, ok := rsc.GetAnnotation(loadbalancer.PassHostHeaderKey); ok {
		if val, er := strconv.ParseBool(phh); er == nil {
			f.PassHostHeader = val
		}
	}

	return &frontend{Frontend: f, id: rsc.ID(), middlewares: make([]*middleware, 0, 1)}, nil
}

func (t *traefik) GetFrontend(id string) (loadbalancer.Frontend, error) {
	return getFrontend(t.Client, t.prefix, id)
}

func (t *traefik) UpsertFrontend(fr loadbalancer.Frontend) error {
	f, ok := fr.(*frontend)
	if !ok {
		return fmt.Errorf("Not of expected type: %v", fr)
	}

	pre := path.Join(t.prefix, "frontends", fr.GetID())
	if er := t.Set(path.Join(pre, "backend"), f.Backend); er != nil {
		return fmt.Errorf("Upsert %v failed: %v", fr, er)
	}
	if f.PassHostHeader {
		if er := t.Set(path.Join(pre, phh), "true"); er != nil {
			logger.Warnf("[%v] Upsert %s error: %v", fr.GetID(), phh, er)
		}
	}

	for id, rt := range f.Routes {
		logger.Debugf("[%v] Adding Route(%s=%q)", fr.GetID(), rt.Rule, rt.Value)
		ruleK := path.Join(pre, "routes", id, "rule")
		valk := path.Join(pre, "routes", id, "value")
		if er := t.Set(ruleK, rt.Rule); er != nil {
			logger.Warnf("[%v] Upsert rule error: %v", fr.GetID(), er)
		}
		if er := t.Set(valk, rt.Value); er != nil {
			logger.Warnf("[%v] Upsert value error: %v", fr.GetID(), er)
		}
	}
	return nil
}

func (t *traefik) DeleteFrontend(fr loadbalancer.Frontend) error {
	logger.Debugf("[%v] Attempting to delete: %v", fr.GetID(), fr)
	key := path.Join(t.prefix, "frontends", fr.GetID())
	return t.Delete(key)
}

func (t *traefik) NewBackend(rsc *kubernetes.Resource) (loadbalancer.Backend, error) {
	// b := new(types.Backend)
	b := &types.Backend{
		LoadBalancer:   defaultLoadBalancerMethod,
		CircuitBreaker: defaultCircuitBreaker,
	}
	b.Servers = make(map[string]types.Server)
	if lbm, ok := rsc.GetAnnotation(LoadbalancingMethodKey); ok && validLBM(lbm) {
		b.LoadBalancer = &types.LoadBalancer{Method: lbm}
	}
	if exp, ok := rsc.GetAnnotation(loadbalancer.FailoverExpressionKey); ok {
		b.CircuitBreaker = &types.CircuitBreaker{Expression: exp}
	}

	return &backend{Backend: *b, id: rsc.ID()}, nil
}

func (t *traefik) GetBackend(id string) (loadbalancer.Backend, error) {
	return getBackend(t.Client, t.prefix, id)
}

func (t *traefik) UpsertBackend(ba loadbalancer.Backend) error {
	b, ok := ba.(*backend)
	if !ok {
		return fmt.Errorf("Not of expected type: %v", ba)
	}

	pre := path.Join(t.prefix, "backends", ba.GetID())
	if b.CircuitBreaker != nil && b.CircuitBreaker.Expression != "" {
		if er := t.Set(path.Join(pre, cb), b.CircuitBreaker.Expression); er != nil {
			logger.Warnf("[%v] Upsert %s error: %v", ba.GetID(), cb, er)
		}
	}
	if b.LoadBalancer != nil && b.LoadBalancer.Method != "" {
		if er := t.Set(path.Join(pre, lb), b.LoadBalancer.Method); er != nil {
			logger.Warnf("[%v] Upsert %s error: %v", ba.GetID(), lb, er)
		}
	}

	for id, srv := range b.Servers {
		logger.Debugf("[%v] Upserting Server(%v)", ba.GetID(), srv.URL)
		urlK := path.Join(pre, "servers", id, "url")
		weightK := path.Join(pre, "servers", id, "weight")
		if er := t.Set(urlK, srv.URL); er != nil {
			logger.Warnf("[%v] Upsert error: %v", ba.GetID(), er)
		}
		weight := strconv.Itoa(srv.Weight)
		if er := t.Set(weightK, weight); er != nil {
			logger.Warnf("[%v] Upsert error: %v", ba.GetID(), er)
		}
	}
	return nil
}

func (t *traefik) DeleteBackend(ba loadbalancer.Backend) error {
	logger.Debugf("[%v] Attempting delete: %v", ba.GetID(), ba)
	key := path.Join(t.prefix, "backends", ba.GetID())
	return t.Delete(key)
}

func (t *traefik) NewServers(rsc *kubernetes.Resource) ([]loadbalancer.Server, error) {
	list := make([]loadbalancer.Server, 0, 1)
	for _, srv := range rsc.Servers() {
		s := types.Server{URL: srv.URL().String(), Weight: 1}
		list = append(list, &server{Server: s, id: srv.ID()})
	}
	return list, nil
}

func (t *traefik) GetServers(id string) ([]loadbalancer.Server, error) {
	return getServers(t.Client, t.prefix, id), nil
}

func (t *traefik) UpsertServer(ba loadbalancer.Backend, srv loadbalancer.Server) error {
	if er := t.Exists(path.Join(t.prefix, "backends", ba.GetID())); er != nil {
		return fmt.Errorf("Lookup %v failed: %v", ba, er)
	}

	urlKey := path.Join(t.prefix, "backends", ba.GetID(), "servers", srv.GetID(), "url")
	weightKey := path.Join(t.prefix, "backends", ba.GetID(), "servers", srv.GetID(), "weight")
	if er := t.Set(urlKey, srv.(*server).URL); er != nil {
		return er
	}
	weight := strconv.Itoa(srv.(*server).Weight)
	if er := t.Set(weightKey, weight); er != nil {
		return er
	}
	return nil
}

func (t *traefik) DeleteServer(ba loadbalancer.Backend, srv loadbalancer.Server) error {
	logger.Debugf("[%v] Attempting delete: %v", ba.GetID(), srv)
	key := path.Join(t.prefix, "backends", ba.GetID(), "servers", srv.GetID())
	if er := t.Exists(key); er != nil {
		return fmt.Errorf("Lookup %v failed: %v", srv, er)
	}

	return t.Delete(key)
}

func (t *traefik) NewMiddlewares(rsc *kubernetes.Resource) ([]loadbalancer.Middleware, error) {
	return []loadbalancer.Middleware{}, nil
}
