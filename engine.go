package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/cenkalti/backoff"

	"golang.org/x/net/context"

	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

func NewEngine(kubeapi, user, pass string, insecure bool, lb loadbalancer.LoadBalancer, timeout time.Duration, ctx context.Context) (*Engine, error) {
	kc, er := kubernetes.NewClient(kubeapi, user, pass, insecure)
	if er != nil {
		return nil, er
	}
	back := backoff.NewExponentialBackOff()
	back.MaxElapsedTime = timeout
	return &Engine{
		Context:      ctx,
		BackOff:      back,
		LoadBalancer: lb,
		Cache:        kubernetes.NewCache(),
		Client:       kc,
	}, nil
}

func (e *Engine) Start(selector kubernetes.Selector, resync time.Duration) error {
	var (
		er error
	)
	if er = kubernetes.Status(e.Client); er != nil {
		return fmt.Errorf("Failed to connect to kubernetes: %v", er)
	}
	if er = e.LoadBalancer.Status(); er != nil {
		return fmt.Errorf("Failed to connect to loadbalancer: %v", er)
	}

	e.Lock()
	defer e.Unlock()

	createObjectCache(e, selector, resync)
	time.Sleep(200 * time.Millisecond)
	return createKubernetesCallbacks(e, selector, resync)
}

func (e *Engine) Add(obj interface{}) {
	e.Lock()
	defer e.Unlock()

	resources, er := kubernetes.GenResources(e.Cache, obj)
	if er != nil {
		logger.Errorf(er.Error())
	}

	if er := addResources(e, resources); er != nil {
		logger.Errorf(er.Error())
	}
}

func (e *Engine) Delete(obj interface{}) {
	e.Lock()
	defer e.Unlock()

	resources, er := kubernetes.GenResources(e.Cache, obj)
	if er != nil {
		logger.Errorf(er.Error())
	}

	if er := deleteResources(e, resources); er != nil {
		logger.Errorf(er.Error())
	}

}

func (e *Engine) Update(old, next interface{}) {
	e.Lock()
	defer e.Unlock()

	logger.Debugf("Gather resources from previous object")
	oldResources, er := kubernetes.GenResources(e.Cache, old)
	if er != nil {
		logger.Errorf(er.Error())
	}

	logger.Debugf("Gather resources from new object")
	newResources, er := kubernetes.GenResources(e.Cache, next)
	if er != nil {
		logger.Errorf(er.Error())
	}

	if er := updateResources(e, newResources, oldResources); er != nil {
		logger.Errorf(er.Error())
	}
}

func (e *Engine) Commit(fn UpsertFunc) error {
	e.Reset()
	for {
		select {
		case <-e.Done():
			return nil
		default:
			duration := e.NextBackOff()
			if duration == backoff.Stop {
				return errors.New("Timed out trying to Commit changes to loadbalancer")
			}
			er := fn()
			if er == nil {
				return er
			}
			logger.Warnf("Commit failed, retry in %v: %v", duration, er)
			time.Sleep(duration)
		}
	}
}

func updateResources(e *Engine, resources, previous kubernetes.ResourceList) error {
	removals := kubernetes.ResourceList{}
	m := resources.Map()
	for _, rsc := range previous {
		if _, ok := m[rsc.ID()]; !ok {
			removals = append(removals, rsc)
		}
	}

	if er := addResources(e, resources); er != nil {
		return er
	}
	return deleteResources(e, removals)
}

func deleteResources(e *Engine, resources kubernetes.ResourceList) error {
	for _, rsc := range resources {
		backend, er := e.NewBackend(rsc)
		if er != nil {
			return er
		}
		frontend, er := e.NewFrontend(rsc)
		if er != nil {
			return er
		}

		fn := func() error {
			logger.Infof("Removing %v", frontend)
			if er := e.DeleteFrontend(frontend); er != nil {
				return er
			}
			logger.Infof("Removing %v", backend)
			return e.DeleteBackend(backend)
		}
		if er := e.Commit(fn); er != nil {
			return er
		}
	}
	return nil
}

func addResources(e *Engine, resources kubernetes.ResourceList) error {
	backends := make([]loadbalancer.Backend, 0, len(resources))
	frontends := make([]loadbalancer.Frontend, 0, len(resources))
	for _, rsc := range resources {
		logger.Debugf("[%v] Build Frontends and Backends", rsc.ID())
		backend, er := e.NewBackend(rsc)
		if er != nil {
			return er
		}
		srvs, er := e.NewServers(rsc)
		if er != nil {
			return er
		}
		for i := range srvs {
			backend.AddServer(srvs[i])
		}
		logger.Debugf("[%v] Created new object: %v", rsc.ID(), backend)
		backends = append(backends, backend)

		frontend, er := e.NewFrontend(rsc)
		if er != nil {
			return er
		}
		mids, er := e.NewMiddlewares(rsc)
		if er != nil {
			return er
		}
		for i := range mids {
			frontend.AddMiddleware(mids[i])
		}
		frontends = append(frontends, frontend)
		logger.Debugf("[%v] Created new object: %v", rsc.ID(), frontend)
	}

	return e.Commit(func() error {
		for _, backend := range backends {
			logger.Infof("Upserting %v", backend)
			if er := e.UpsertBackend(backend); er != nil {
				return er
			}
		}
		for _, frontend := range frontends {
			logger.Infof("Upserting %v", frontend)
			if er := e.UpsertFrontend(frontend); er != nil {
				return er
			}
		}
		return nil
	})
}

func createObjectCache(e *Engine, selector kubernetes.Selector, resync time.Duration) {
	var (
		uc = e.GetUnversionedClient()
		ec = e.GetExtensionsClient()
	)
	logger.Infof("Creating kubernetes object cache")

	service, er := kubernetes.CreateStore(kubernetes.ServicesKind, uc, selector, resync, e.Context)
	if er != nil {
		logger.Warnf("Failed to create Service cache")
	}
	endpoints, er := kubernetes.CreateStore(kubernetes.EndpointsKind, uc, selector, resync, e.Context)
	if er != nil {
		logger.Warnf("Failed to create Endpoints cache")
	}
	ingress, er := kubernetes.CreateStore(kubernetes.IngressesKind, ec, selector, resync, e.Context)
	if er != nil {
		logger.Warnf("Failed to create Ingress cache")
	}

	e.SetIngressStore(ingress)
	e.SetServiceStore(service)
	e.SetEndpointsStore(endpoints)
}

func createKubernetesCallbacks(e *Engine, selector kubernetes.Selector, resync time.Duration) error {
	var (
		uc = e.GetUnversionedClient()
		ec = e.GetExtensionsClient()
	)

	logger.Infof("Starting kubernetes watchers")

	_, endpoint := kubernetes.CreateFullController(kubernetes.EndpointsKind, e, uc, selector, resync)
	_, service := kubernetes.CreateFullController(kubernetes.ServicesKind, e, uc, selector, resync)
	_, ingress := kubernetes.CreateFullController(kubernetes.IngressesKind, e, ec, selector, resync)

	go endpoint.Run(e.Done())
	go service.Run(e.Done())
	go ingress.Run(e.Done())
	return nil
}

type Engine struct {
	sync.Mutex
	backoff.BackOff
	context.Context
	loadbalancer.LoadBalancer
	*kubernetes.Cache
	*kubernetes.Client
}

type UpsertFunc func() error
type ResourceFunc func(*Engine, kubernetes.ResourceList) error

const (
	interval          = 50 * time.Millisecond
	serviceResource   = "services"
	endpointsResource = "endpoints"
	ingressResource   = "ingresses"
)
