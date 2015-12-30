package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/cenkalti/backoff"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/util/intstr"

	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
)

func newEngine(kubeapi, user, pass string, insecure bool, lb loadbalancer.LoadBalancer, timeout time.Duration, ctx context.Context) (*Engine, error) {
	kubernetes.Keyspace = "romulus/"
	k1, er := kubernetes.NewClient(kubeapi, user, pass, insecure)
	if er != nil {
		return nil, er
	}
	k2, er := kubernetes.NewExtensionsClient(kubeapi, user, pass, insecure)
	if er != nil {
		return nil, er
	}
	back := backoff.NewExponentialBackOff()
	back.MaxElapsedTime = timeout
	return &Engine{
		Context:      ctx,
		BackOff:      back,
		LoadBalancer: lb,
		cache:        &kubernetes.KubeCache{},
		client:       &kubernetes.Client{Client: k1, ExtensionsClient: k2},
	}, nil
}

func (e *Engine) Start(selector kubernetes.Selector, resync time.Duration) error {
	var (
		er               error
		service, ingress *framework.Controller
	)
	if er = kubernetes.Status(e.client); er != nil {
		return fmt.Errorf("Failed to connect to kubernetes: %v", er)
	}
	if er = e.LoadBalancer.Status(); er != nil {
		return fmt.Errorf("Failed to connect to loadbalancer: %v", er)
	}

	e.Lock()
	defer e.Unlock()

	logger.Debugf("Setting up Service cache")
	if e.cache.Service, er = kubernetes.CreateStore(kubernetes.ServicesKind, e.client.Client, kubernetes.EverythingSelector, resync, e.Context); er != nil {
		return fmt.Errorf("Unable to create Service cache: %v", er)
	}

	logger.Debugf("Setting up Service and Ingress callbacks")
	_, service = kubernetes.CreateUpdateController(kubernetes.ServicesKind, e, e.client.Client, kubernetes.EverythingSelector, resync)
	e.cache.Ingress, ingress = kubernetes.CreateFullController(kubernetes.IngressKind, e, e.client.ExtensionsClient, selector, resync)

	go service.Run(e.Done())
	time.Sleep(3 * time.Millisecond)
	go ingress.Run(e.Done())
	return nil
}

func (e *Engine) Add(obj interface{}) {
	switch o := obj.(type) {
	default:
		logger.Errorf("Got unknown type in Add callback: %+v", o)
	case *extensions.Ingress:
		logger.Debugf("[Callback] Add %v", kubernetes.KubeIngress(*o))
		if er := e.addFrontend(o); er != nil {
			logger.Errorf("Add %v failed: %v", kubernetes.KubeIngress(*o), er)
		}
	case *api.Service:
		logger.Debugf("[Callback] Add %v", kubernetes.KubeService(*o))
		if er := e.addBackends(o); er != nil {
			logger.Errorf("Add %v failed: %v", kubernetes.KubeService(*o), er)
		}
	}
}

func (e *Engine) Delete(obj interface{}) {
	switch o := obj.(type) {
	default:
		logger.Errorf("Got unknown type in Add callback: %+v", o)
	case *extensions.Ingress:
		logger.Debugf("[Callback] Delete %v", kubernetes.KubeIngress(*o))
		if er := e.deleteFrontend(o); er != nil {
			logger.Errorf("Delete %v failed: %v", kubernetes.KubeIngress(*o), er)
		}
	case *api.Service:
		logger.Debugf("[Callback] Delete %v", kubernetes.KubeService(*o))
		if er := e.deleteBackends(o); er != nil {
			logger.Errorf("Delete %v failed: %v", kubernetes.KubeService(*o), er)
		}
	}
}

func (e *Engine) Update(old, next interface{}) {
	switch o := next.(type) {
	default:
		logger.Errorf("Got unknown type in Update callback: %+v", o)
	case *extensions.Ingress:
		prev, ok := old.(*extensions.Ingress)
		if !ok {
			logger.Errorf("Got unknown type in Update callback: %+v", old)
			return
		}
		logger.Debugf("[Callback] Update %v", kubernetes.KubeIngress(*o))
		if er := e.updateFrontend(prev, o); er != nil {
			logger.Errorf("Update %v failed: %v", kubernetes.KubeIngress(*o), er)
		}
	case *api.Service:
		prev, ok := old.(*api.Service)
		if !ok {
			logger.Errorf("Got unknown type in Update callback: %+v", old)
			return
		}
		logger.Debugf("[Callback] Update %v", kubernetes.KubeService(*o))
		if er := e.updateBackends(prev, o); er != nil {
			logger.Errorf("Update %v failed: %v", kubernetes.KubeService(*o), er)
		}
	}
}

func (e *Engine) deleteBackends(s *api.Service) error {
	e.Lock()
	defer e.Unlock()

	backends, er := gatherBackendsFromService(e, s)
	if er != nil {
		return er
	}

	return e.commit(func() error {
		for _, backend := range backends {
			logger.Infof("Deleting %v", backend)
			if er := e.DeleteBackend(backend); er != nil {
				return er
			}
		}
		return nil
	})
}

func (e *Engine) updateBackends(prev, next *api.Service) error {
	e.Lock()
	defer e.Unlock()

	if !api.IsServiceIPSet(next) {
		return errors.New("Service IP not set")
	}

	logger.Debugf("Parse prev version: %v", kubernetes.KubeService(*prev))
	prevBackends, _ := gatherBackendsFromService(e, prev)
	logger.Debugf("%v :: Previous Backends: %v", kubernetes.KubeService(*prev), prevBackends)
	logger.Debugf("Parse new version: %v", kubernetes.KubeService(*next))
	nextBackends, er := gatherBackendsFromService(e, next)
	logger.Debugf("%v :: New Backends: %v", kubernetes.KubeService(*next), nextBackends)
	if er != nil {
		return er
	}
	prevBackendsMap := map[string]loadbalancer.Backend{}
	for _, b := range prevBackends {
		prevBackendsMap[b.GetID()] = b
	}
	logger.Debugf("Backend map: %v", prevBackendsMap)

	return e.commit(func() error {
		for _, backend := range nextBackends {
			logger.Infof("Upserting %v", backend)
			if er := e.UpsertBackend(backend); er != nil {
				return er
			}
			logger.Debugf("Removing %v from map", backend)
			delete(prevBackendsMap, backend.GetID())
		}
		logger.Debugf("Backend map: %v", prevBackendsMap)
		for _, backend := range prevBackendsMap {
			logger.Infof("Removing %v", backend)
			if er := e.DeleteBackend(backend); er != nil {
				logger.Warnf("Failed to remove %v: %v", backend, er)
			}
		}
		return nil
	})
}

func (e *Engine) addBackends(s *api.Service) error {
	e.Lock()
	defer e.Unlock()

	if !api.IsServiceIPSet(s) {
		return errors.New("Service IP not set")
	}

	backends, er := gatherBackendsFromService(e, s)
	logger.Debugf("Backends: %v", backends)
	if er != nil {
		return er
	}

	return e.commit(func() error {
		for _, backend := range backends {
			logger.Infof("Upserting %v", backend)
			if er := e.UpsertBackend(backend); er != nil {
				return er
			}
		}
		return nil
	})
}

func gatherBackendsFromService(e *Engine, s *api.Service) ([]loadbalancer.Backend, error) {
	var backends = []loadbalancer.Backend{}

	logger.Debugf("Gathering Backends from %v", kubernetes.KubeService(*s))
	for _, svcPort := range s.Spec.Ports {
		logger.Debugf(`%v :: Port(name="%s", port=%d)`, kubernetes.KubeService(*s), svcPort.Name, svcPort.Port)
		service, ok := e.FindService(svcPort, s.ObjectMeta)
		if !ok {
			continue
		}

		id := kubernetes.ServerID(s.Spec.ClusterIP, svcPort.Port, s.ObjectMeta)
		service.AddBackend(id, "http", s.Spec.ClusterIP, svcPort.Port)
		logger.Debugf("Found Service: %v", service)

		backend, er := e.NewBackend(service)
		if er != nil {
			return backends, er
		}
		logger.Debugf("[%v] Created Backend: %v", service.ID, backend)

		srvs, er := e.NewServers(service)
		if er != nil {
			return backends, er
		}
		for i := range srvs {
			logger.Debugf("[%v] Adding Server: %v", service.ID, srvs[i])
			backend.AddServer(srvs[i])
		}
		backends = append(backends, backend)
	}
	return backends, nil
}

func (e *Engine) FindService(port api.ServicePort, meta api.ObjectMeta) (*kubernetes.Service, bool) {
	var id string

	if port.Name != "" {
		id = kubernetes.ServiceID(meta, intstr.FromString(port.Name))
		if _, er := e.GetBackend(id); er == nil {
			return kubernetes.NewService(id, meta), true
		}
	}

	id = kubernetes.ServiceID(meta)
	if _, er := e.GetBackend(id); er == nil {
		return kubernetes.NewService(id, meta), true
	}
	return nil, false
}

func (e *Engine) updateFrontend(old, next *extensions.Ingress) error {
	e.Lock()
	defer e.Unlock()

	oldSVCs := kubernetes.ServicesFromIngress(e.cache, old)
	add := kubernetes.ServicesFromIngress(e.cache, next)
	if len(add) < 1 {
		return deleteServices(e, oldSVCs)
	}

	oldSVCmap := make(map[string]*kubernetes.Service, len(oldSVCs))
	for i := range oldSVCs {
		oldSVCmap[oldSVCs[i].ID] = oldSVCs[i]
	}
	logger.Debugf("Old Services: %v", oldSVCmap)

	del := []*kubernetes.Service{}
	for _, s2 := range add {
		if s1, ok := oldSVCmap[s2.ID]; !ok {
			del = append(del, s1)
		}
	}
	logger.Debugf("Services to add: %v", add)
	logger.Debugf("Services to remove: %v", del)

	if er := addServices(e, add); er != nil {
		return er
	}
	return deleteServices(e, del)
}

func (e *Engine) addFrontend(in *extensions.Ingress) error {
	e.Lock()
	defer e.Unlock()

	services := kubernetes.ServicesFromIngress(e.cache, in)
	if len(services) < 1 {
		return fmt.Errorf("No services to add from %v", kubernetes.KubeIngress(*in))
	}

	return addServices(e, services)
}

func (e *Engine) deleteFrontend(in *extensions.Ingress) error {
	e.Lock()
	defer e.Unlock()

	services := kubernetes.ServicesFromIngress(e.cache, in)
	if len(services) < 1 {
		return fmt.Errorf("No services to add from %v", kubernetes.KubeIngress(*in))
	}

	return deleteServices(e, services)
}

func (e *Engine) commit(fn upsertFunc) error {
	e.Reset()
	for {
		select {
		case <-e.Done():
			return nil
		default:
			duration := e.NextBackOff()
			if duration == backoff.Stop {
				return errors.New("Timed out trying to commit changes to loadbalancer")
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

func addServices(e *Engine, services []*kubernetes.Service) error {
	backends := make([]loadbalancer.Backend, 0, len(services))
	frontends := make([]loadbalancer.Frontend, 0, len(services))
	for _, svc := range services {
		logger.Debugf("[%v] Build Frontends and Backends", svc.ID)
		backend, er := e.NewBackend(svc)
		if er != nil {
			return er
		}
		srvs, er := e.NewServers(svc)
		if er != nil {
			return er
		}
		for i := range srvs {
			backend.AddServer(srvs[i])
		}
		logger.Debugf("[%v] Created new object: %v", svc.ID, backend)
		backends = append(backends, backend)

		frontend, er := e.NewFrontend(svc)
		if er != nil {
			return er
		}
		mids, er := e.NewMiddlewares(svc)
		if er != nil {
			return er
		}
		for i := range mids {
			frontend.AddMiddleware(mids[i])
		}
		frontends = append(frontends, frontend)
		logger.Debugf("[%v] Created new object: %v", svc.ID, frontend)
	}

	return e.commit(func() error {
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

func deleteServices(e *Engine, services []*kubernetes.Service) error {
	for _, svc := range services {
		backend, er := e.NewBackend(svc)
		if er != nil {
			return er
		}
		frontend, er := e.NewFrontend(svc)
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
		if er := e.commit(fn); er != nil {
			return er
		}
	}
	return nil
}

// Engine is the main driver and handles kubernetes callbacks
type Engine struct {
	sync.Mutex
	backoff.BackOff
	context.Context
	loadbalancer.LoadBalancer
	cache *kubernetes.KubeCache
	// controller *framework.Controller
	client *kubernetes.Client
}

type upsertFunc func() error

const (
	interval          = 50 * time.Millisecond
	serviceResource   = "services"
	endpointsResource = "endpoints"
	ingressResource   = "ingresses"
)
