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

	o, b := cache.get(cKey{name, ns, serviceType})
	if b {
		debugf("Cache hit %v", cKey{name, ns, serviceType})
		s := o.(*api.Service)
		return s, true, nil
	}

	debugf("Cache miss %v", cKey{name, ns, serviceType})
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

	debugf("Caching {Kind: %q, Name: %q, Namespace: %q}", serviceType, s.Name, s.Namespace)
	cache.put(cKey{s.Name, s.Namespace, serviceType}, s)
	return s, true, nil
}

func getEndpoints(name, ns string) (*api.Endpoints, bool, error) {
	if cache == nil {
		cache = newCache()
	}

	o, b := cache.get(cKey{name, ns, endpointsType})
	if b {
		debugf("Cache hit %v", cKey{name, ns, endpointsType})
		en := o.(*api.Endpoints)
		return en, true, nil
	}

	debugf("Cache miss %v", cKey{name, ns, endpointsType})
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

	debugf("Caching {Kind: %q, Name: %q, Namespace: %q}", endpointsType, en.Name, en.Namespace)
	cache.put(cKey{en.Name, en.Namespace, endpointsType}, en)
	return en, true, nil
}

func cacheIfNewer(key cKey, o runtime.Object) (runtime.Object, bool) {
	if cache == nil {
		cache = newCache()
	}

	oo, ok := cache.get(key)
	if !ok {
		debugf("Object is new, Caching %v", key)
		cache.put(key, o)
		return o, true
	}

	om, er := getMeta(oo)
	if er != nil {
		debugf("Could not get metadata for original object, Caching %v", key)
		cache.put(key, o)
		return o, true
	}
	nm, er := getMeta(o)
	if er != nil {
		debugf("Could not get metadata, will not cache: %v", o)
		return oo, false
	}

	if nm.version != 0 && nm.version > om.version {
		debugf("Object is newer (%d > %d), Caching %v", nm.version, om.version, key)
		cache.put(key, o)
		return o, true
	}

	debugf("Object is older (%d < %d), using cached version", nm.version, om.version)
	return oo, false
}
