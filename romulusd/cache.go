package main

import (
	"sync"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type cKey struct {
	name, ns, kind string
}

type cMap struct {
	sync.RWMutex
	m map[cKey]runtime.Object
}

func newCache() *cMap {
	return &cMap{m: make(map[cKey]runtime.Object)}
}

func (m *cMap) get(key cKey) (o runtime.Object, b bool) {
	m.RLock()
	defer m.RUnlock()
	o, b = m.m[key]
	return
}

func (m *cMap) put(key cKey, o runtime.Object) {
	m.Lock()
	defer m.Unlock()
	m.m[key] = o
}

func (m *cMap) del(key cKey) {
	m.Lock()
	defer m.Unlock()
	delete(m.m, key)
}

func getService(name, ns string) (s *api.Service, b bool) {
	if cache == nil {
		return
	}

	o, b := cache.get(cKey{name, ns, "Service"})
	if !b {
		kc, er := kubeClient()
		if er != nil {
			warnf("kubernetes client failure: %v", er)
		}
		s, er := kc.Services(ns).Get(name)
		if er != nil {
			debugf("Failed to get Service: %v", er)
			return nil, false
		}
		b = true
		cache.put(cKey{s.Name, s.Namespace, s.Kind}, s)
		return s, b
	}
	s = o.(*api.Service)
	return
}

func getEndpoints(name, ns string) (en *api.Endpoints, b bool) {
	if cache == nil {
		return
	}

	o, b := cache.get(cKey{name, ns, "Endpoints"})
	if !b {
		kc, er := kubeClient()
		if er != nil {
			warnf("kubernetes client failure: %v", er)
		}
		en, er := kc.Endpoints(ns).Get(name)
		if er != nil {
			debugf("failed to get Endpoints", er)
			return nil, false
		}
		b = true
		cache.put(cKey{en.Name, en.Namespace, en.Kind}, en)
		return en, b
	}
	en = o.(*api.Endpoints)
	return
}
