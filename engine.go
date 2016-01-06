package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/cenkalti/backoff"
	"github.com/davecgh/go-spew/spew"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
)

var (
	upsertBackoff = backoff.NewExponentialBackOff()
	upsertTimeout = 10 * time.Second
)

func newEngine(kubeapi, kubever string, insecure bool, sel map[string]string, lb LoadBalancer, ctx context.Context) (*Engine, error) {
	k, er := newKubeClient(kubeapi, kubever, insecure)
	if er != nil {
		return nil, er
	}
	return &Engine{
		cache:      &kubeCache{},
		controller: &kubeController{},
		lb:         lb,
		kube:       k,
		selector:   sel,
		ctx:        ctx,
	}, nil
}

func (e *Engine) Start(resync time.Duration) error {
	if er := Status(e.kube); er != nil {
		return fmt.Errorf("Failed to connect to kubernetes: %v", er)
	}
	if er := e.lb.Status(); er != nil {
		return fmt.Errorf("Failed to connect to loadbalancer: %v", er)
	}

	e.cache.service, e.controller.service = SetWatch(e, e.kube, serviceResource, e.selector, resync)
	e.cache.endpoints, e.controller.endpoints = SetWatch(e, e.kube, endpointsResource, e.selector, resync)
	// e.cache.service = scache
	// e.cache.endpoints = ecache

	go e.controller.run(serviceResource, e.ctx)
	go e.controller.run(endpointsResource, e.ctx)
	return nil
}

func (e *Engine) Add(obj interface{}) {
	var (
		service   *Service
		endpoints *Endpoints
	)

	switch o := obj.(type) {
	default:
		logger.Debugf(spew.Sprintf("Other: %#v", o))
		return
	case *api.Service:
		service = &Service{*o}
		logger.Debugf("Callback: Add %v", service)
		en, er := e.cache.getEndpoints(o)
		if er != nil || en == nil {
			logger.Errorf("No Endpoints for %v", service)
			return
		}
		endpoints = &Endpoints{*en}
	case *api.Endpoints:
		endpoints = &Endpoints{*o}
		logger.Debugf("Callback: Add %v", endpoints)
		s, er := e.cache.getService(o)
		if er != nil || s == nil {
			logger.Errorf("No Service for %v", endpoints)
			return
		}
		service = &Service{*s}
	}
	if er := e.add(service, endpoints); er != nil {
		logger.Errorf("Add failed: %v", er)
	}
}

func (e *Engine) Delete(obj interface{}) {
	switch o := obj.(type) {
	case *api.Service:
		logger.Debugf("Callback: Delete %v", Service{*o})
		if er := e.deleteService(o); er != nil {
			logger.Warnf("Delete %v failed: %v", Service{*o}, er)
		}
	case *api.Endpoints:
		logger.Debugf("Callback: Delete %v", Endpoints{*o})
		if er := e.deleteBackend(o); er != nil {
			logger.Warnf("Delete %v failed: %v", Endpoints{*o}, er)
		}
	}
}

func (e *Engine) Update(old, next interface{}) {
	var (
		service   *Service
		endpoints *Endpoints
	)

	switch o := next.(type) {
	default:
		logger.Debugf(spew.Sprintf("Other: %#v", o))
		return
	case *api.Service:
		service = &Service{*o}
		logger.Debugf("Callback: Update %v", service)
		en, er := e.cache.getEndpoints(o)
		if er != nil || en == nil {
			logger.Errorf("No Endpoints for %v", service)
			return
		}
		endpoints = &Endpoints{*en}
	case *api.Endpoints:
		endpoints = &Endpoints{*o}
		logger.Debugf("Callback: Update %v", endpoints)
		s, er := e.cache.getService(o)
		if er != nil || s == nil {
			logger.Errorf("No Service for %v", endpoints)
			return
		}
		service = &Service{*s}
	}
	if er := e.add(service, endpoints); er != nil {
		logger.Errorf("Add failed: %v", er)
	}
}

func (e *Engine) add(svc *Service, en *Endpoints) error {
	e.Lock()
	defer e.Unlock()

	m, er := GetMetadata(&(svc.Service))
	if er != nil {
		return er
	}

	backend, er := e.lb.NewBackend(m)
	if er != nil {
		return er
	}
	addr := AddressesFromSubsets(en.Subsets)
	logger.Debugf("[%v] Addresses found in %v: %v", backend.GetID(), Endpoints{*en}, addr)
	srvs, er := e.lb.NewServers(addr, m)
	logger.Debugf("[%v] Servers created: %v", backend.GetID(), srvs)
	if er != nil {
		return er
	}
	for i := range srvs {
		logger.Debugf("[%v] Adding %v", backend.GetID(), srvs[i])
		backend.AddServer(srvs[i])
	}

	frontend, er := e.lb.NewFrontend(m)
	if er != nil {
		return er
	}
	mids, er := e.lb.NewMiddlewares(m)
	if er != nil {
		return er
	}
	for i := range mids {
		logger.Debugf("[%v] Adding %v", frontend.GetID(), mids[i])
		frontend.AddMiddleware(mids[i])
	}

	e.commit(func() error {
		logger.Infof("[%v] Upserting %v", backend.GetID(), backend)
		if er := e.lb.UpsertBackend(backend); er != nil {
			return er
		}
		logger.Infof("[%v] Upserting %v", frontend.GetID(), frontend)
		return e.lb.UpsertFrontend(frontend)
	})
	return nil
}

func (e *Engine) deleteService(s *api.Service) error {
	e.Lock()
	defer e.Unlock()

	m, er := GetMetadata(s)
	if er != nil {
		return er
	}

	frontend, er := e.lb.NewFrontend(m)
	if er != nil {
		return er
	}
	e.commit(func() error {
		logger.Infof("Removing %v", frontend)
		return e.lb.DeleteFrontend(frontend)
	})
	return nil
}

func (e *Engine) deleteBackend(en *api.Endpoints) error {
	e.Lock()
	defer e.Unlock()

	m, er := GetMetadata(en)
	if er != nil {
		return er
	}
	backend, er := e.lb.NewBackend(m)
	if er != nil {
		return er
	}
	e.commit(func() error {
		logger.Infof("[%v] Removing %v", backend.GetID(), backend)
		return e.lb.DeleteBackend(backend)
	})
	return nil
}

func (e *Engine) commit(fn upsertFunc) {
	upsertBackoff.MaxElapsedTime = upsertTimeout
	upsertBackoff.Reset()

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			duration := upsertBackoff.NextBackOff()
			if duration == backoff.Stop {
				logger.Errorf("Timed out trying to commit changes to loadbalancer")
				return
			}
			er := fn()
			if er == nil {
				return
			}
			logger.Warnf("Commit failed, retry in %v: %v", duration, er)
			time.Sleep(duration)
		}
	}
}

func (c *kubeCache) getEndpoints(s *api.Service) (*api.Endpoints, error) {
	key, er := cache.MetaNamespaceKeyFunc(s)
	if er != nil {
		return nil, er
	}
	obj, ok, er := c.endpoints.GetByKey(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, nil
	}
	e, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("Endpoints cache returned non-Endpoints object")
	}
	return e, nil
}

func (c *kubeCache) getService(e *api.Endpoints) (*api.Service, error) {
	key, er := cache.MetaNamespaceKeyFunc(e)
	if er != nil {
		return nil, er
	}
	obj, ok, er := c.service.GetByKey(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, nil
	}
	s, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("Service cache returned non-Service object")
	}
	return s, nil
}

func (c *kubeController) run(resource string, ctx context.Context) {
	logger.Debugf("Starting %q watch", resource)
	switch resource {
	case serviceResource:
		c.service.Run(ctx.Done())
	case endpointsResource:
		c.endpoints.Run(ctx.Done())
	}
}

func (c *kubeController) requeue(resource string, obj interface{}) {
	logger.Debugf("Requeue %v", obj)
	switch resource {
	case serviceResource:
		c.service.Requeue(obj)
	case endpointsResource:
		c.endpoints.Requeue(obj)
	}
}

// Engine is the main driver and handles kubernetes callbacks
type Engine struct {
	sync.Mutex
	cache      *kubeCache
	controller *kubeController
	lb         LoadBalancer
	kube       *unversioned.Client
	selector   map[string]string
	timeout    time.Duration
	ctx        context.Context
}

type kubeCache struct {
	service   cache.Store
	endpoints cache.Store
}

type kubeController struct {
	service   *framework.Controller
	endpoints *framework.Controller
}

type upsertFunc func() error

const (
	interval          = 50 * time.Millisecond
	serviceResource   = "services"
	endpointsResource = "endpoints"
)

func doAdd(s *api.Service) bool {
	logger.Debugf("%v type=%v ip-set=%v", Service{*s}, s.Spec.Type, api.IsServiceIPSet(s))
	return s.Spec.Type == api.ServiceTypeClusterIP && api.IsServiceIPSet(s)
}
