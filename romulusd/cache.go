package main

import (
	"fmt"
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

func (k cKey) String() string {
	return fmt.Sprintf("{Kind: %q, Name: %q, Namespace: %q}", k.kind, k.name, k.ns)
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

func getService(name, ns string) (*api.Service, bool, error) {
	if cache == nil {
		cache = newCache()
	}

	o, b := cache.get(cKey{name, ns, "Service"})
	if b {
		debugf("Cache hit %v", cKey{name, ns, "Service"})
		s := o.(*api.Service)
		return s, true, nil
	}

	debugf("Cache miss %v", cKey{name, ns, "Service"})
	kc, er := kubeClient()
	if er != nil {
		errorf("kubernetes client failure: %v", er)
		return nil, false, NewErr(er, "kubernetes client failure")
	}
	s, er := kc.Services(ns).Get(name)
	if er != nil {
		if kubeIsNotFound(er) {
			return nil, false, nil
		}
		errorf("kubernetes api failure: %v", er)
		return nil, false, NewErr(er, "kubernetes api failure")
	}

	debugf("Caching {Kind: %q, Name: %q, Namespace: %q}", "Service", s.Kind, s.Name, s.Namespace)
	cache.put(cKey{s.Name, s.Namespace, "Service"}, s)
	return s, true, nil
}

func getEndpoints(name, ns string) (*api.Endpoints, bool, error) {
	if cache == nil {
		cache = newCache()
	}

	o, b := cache.get(cKey{name, ns, "Endpoints"})
	if b {
		debugf("Cache hit %v", cKey{name, ns, "Endpoints"})
		en := o.(*api.Endpoints)
		return en, true, nil
	}

	debugf("Cache miss %v", cKey{name, ns, "Endpoints"})
	kc, er := kubeClient()
	if er != nil {
		errorf("kubernetes client failure: %v", er)
		return nil, false, NewErr(er, "kubernetes client failure")
	}
	en, er := kc.Endpoints(ns).Get(name)
	if er != nil {
		if kubeIsNotFound(er) {
			return nil, false, nil
		}
		errorf("kubernetes api failure: %v", er)
		return nil, false, NewErr(er, "kubernetes api failure")
	}

	debugf("Caching {Kind: %q, Name: %q, Namespace: %q}", "Endpoints", en.Kind, en.Name, en.Namespace)
	cache.put(cKey{en.Name, en.Namespace, "Endpoints"}, en)
	return en, true, nil
}
