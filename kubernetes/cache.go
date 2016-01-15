package kubernetes

import (
	"errors"
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

func NewCache() Cache {
	return make(map[string]cache.Store, 3)
}

func (k Cache) AddStore(key string, store cache.Store) bool {
	_, ok := k[key]
	if !ok {
		k[key] = store
	}
	return ok
}

func (k Cache) GetEndpoints(namespace, name string) (*api.Endpoints, error) {
	obj, er := getFromCache(k[EndpointsKind], EndpointsKind, namespace, name)
	if er != nil {
		return nil, er
	}
	s, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("Endpoints cache returned non-Endpoints object")
	}
	return s, nil
}

func (k Cache) GetService(namespace, name string) (*api.Service, error) {
	obj, er := getFromCache(k[ServiceKind], ServiceKind, namespace, name)
	if er != nil {
		return nil, er
	}
	s, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("Service cache returned non-Service object")
	}
	return s, nil
}

func getFromCache(store cache.Store, kind, namespace, name string) (interface{}, error) {
	key := cacheLookupKey(namespace, name)
	obj, ok, er := store.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, fmt.Errorf("Could not find %s %q", kind, key)
	}
	return obj, nil
}
