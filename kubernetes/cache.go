package kubernetes

import (
	"errors"
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

func NewCache() *Cache {
	return &Cache{
		ingress:   cache.NewStore(cache.MetaNamespaceKeyFunc),
		service:   cache.NewStore(cache.MetaNamespaceKeyFunc),
		endpoints: cache.NewStore(cache.MetaNamespaceKeyFunc),
	}
}

func (k *Cache) SetIngressStore(store cache.Store) {
	// _, ok := k[key]
	// if !ok {
	// 	k[key] = store
	// }
	// return ok
	k.ingress = store
}

func (k *Cache) SetServiceStore(store cache.Store) {
	k.service = store
}

func (k *Cache) SetEndpointsStore(store cache.Store) {
	k.endpoints = store
}

func (k *Cache) GetEndpoints(namespace, name string) (*api.Endpoints, error) {
	key := cacheLookupKey(namespace, name)
	obj, ok, er := k.endpoints.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, fmt.Errorf("Could not find Endpoints %q", key)
	}

	s, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("Endpoints cache returned non-Endpoints object")
	}
	return s, nil
}

func (k *Cache) GetService(namespace, name string) (*api.Service, error) {
	key := cacheLookupKey(namespace, name)
	obj, ok, er := k.service.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, fmt.Errorf("Could not find Service %q", key)
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
