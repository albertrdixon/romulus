package main

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/runtime"
)

type cKey struct {
	name, ns, kind string
}

type cValue struct {
	t   time.Time
	obj runtime.Object
}

type cMap struct {
	sync.RWMutex
	m map[cKey]cValue
}

func newCache() *cMap {
	return &cMap{m: make(map[cKey]cValue)}
}

func (k cKey) String() string {
	return fmt.Sprintf("{Name: %q, Namespace: %q, Kind: %q}", k.kind, k.name, k.kind)
}

func (v cValue) moreRecent(t time.Time) bool {
	return v.t.After(t)
}

func (m *cMap) get(key cKey) (v cValue, b bool) {
	m.RLock()
	defer m.RUnlock()
	v, b = m.m[key]
	return
}

func (m *cMap) put(key cKey, obj runtime.Object, t time.Time) cValue {
	m.Lock()
	defer m.Unlock()
	val := cValue{t, obj}
	m.m[key] = val
	return val
}

func (m *cMap) del(key cKey) {
	m.Lock()
	defer m.Unlock()
	delete(m.m, key)
}

func getService(name, ns string, t time.Time) (cValue, bool, error) {
	if cache == nil {
		cache = newCache()
	}

	key := cKey{name, ns, serviceType}
	val, b := cache.get(key)
	if b {
		debugf("Cache hit key=%v", key)
		return val, true, nil
	}

	debugf("Cache miss key=%v", key)
	kc, er := kubeClient()
	if er != nil {
		errorf("kubernetes client failure: %v", er)
		return val, false, NewErr(er, "kubernetes client failure")
	}
	s, er := kc.Services(ns).Get(name)
	if er != nil {
		if kubeIsNotFound(er) {
			return val, false, nil
		}
		errorf("kubernetes api failure: %v", er)
		return val, false, NewErr(er, "kubernetes api failure")
	}

	debugf("Caching object, key=%v", key)
	val = cache.put(key, s, t)
	return val, true, nil
}

func getEndpoints(name, ns string, t time.Time) (cValue, bool, error) {
	if cache == nil {
		cache = newCache()
	}

	key := cKey{name, ns, endpointsType}
	val, b := cache.get(key)
	if b {
		debugf("Cache hit key=%v", key)
		return val, true, nil
	}

	debugf("Cache miss key=%v", key)
	kc, er := kubeClient()
	if er != nil {
		errorf("kubernetes client failure: %v", er)
		return val, false, NewErr(er, "kubernetes client failure")
	}
	en, er := kc.Endpoints(ns).Get(name)
	if er != nil {
		if kubeIsNotFound(er) {
			return val, false, nil
		}
		errorf("kubernetes api failure: %v", er)
		return val, false, NewErr(er, "kubernetes api failure")
	}

	debugf("Caching object, key=%v", key)
	val = cache.put(key, en, t)
	return val, true, nil
}
