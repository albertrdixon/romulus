package romulus

import "sync"

type cMap struct {
	sync.RWMutex
	m map[string]*Frontend
}

func newCache() *cMap {
	return &cMap{m: make(map[string]*Frontend)}
}

func (m *cMap) get(s string) (f *Frontend, b bool) {
	m.RLock()
	defer m.RUnlock()
	f, b = m.m[s]
	return
}

func (m *cMap) put(s string, f *Frontend) {
	m.Lock()
	defer m.Unlock()
	m.m[s] = f
}
