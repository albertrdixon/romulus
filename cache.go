package romulus

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

func (m *cMap) get(key cKey) (runtime.Object, bool) {
	m.RLock()
	defer m.RUnlock()
	return m.m[key]
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
		kc := kubeClient()
		if s, er := kc.Services(en.Namespace).Get(en.Name); er != nil {
			return
		}
		b = true
		cache.put(cKey{s.Name, s.Namespace, s.Kind}, s)
		return
	}
	s = o.(*api.Service)
	return
}

func getEndpoints(name, ns string) (en *api.Endpoints, b bool) {
	if cache == nil {
		return
	}

	o, b := cache.get(cKey{s.Name, s.Namespace, "Endpoints"})
	if !b {
		kc := kubeClient()
		if en, er := kc.Endpoints(s.Namespace).Get(s.Name); e != nil {
			return
		}
		b = true
		cache.put(cKey{en.Name, en.Namespace, en.Kind}, en)
		return
	}
	en = o.(*api.Endpoints)
	return
}
