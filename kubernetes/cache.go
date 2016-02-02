package kubernetes

import (
	"errors"
	"fmt"

	"github.com/albertrdixon/gearbox/logger"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

func NewCache() *Cache {
	return &Cache{
		ingress:   cache.NewStore(cache.MetaNamespaceKeyFunc),
		service:   cache.NewStore(cache.MetaNamespaceKeyFunc),
		endpoints: cache.NewStore(cache.MetaNamespaceKeyFunc),
		ingMap:    make(map[cache.ExplicitKey]cache.ExplicitKey),
	}
}

func (k *Cache) SetIngressStore(store cache.Store) {
	k.ingress = store
}

func (k *Cache) SetServiceStore(store cache.Store) {
	k.service = store
}

func (k *Cache) SetEndpointsStore(store cache.Store) {
	k.endpoints = store
}

func (k *Cache) MapServiceToIngress(namespace, serviceName, ingressName string) {
	var (
		svcKey = cacheLookupKey(namespace, serviceName)
		ingKey = cacheLookupKey(namespace, ingressName)
	)

	logger.Debugf("Mapping Service(%q) -> Ingress(%q)", svcKey, ingKey)
	k.ingMap[svcKey] = ingKey
}

func (k *Cache) ServiceDeleted(namespace, name string) {
	var key = cacheLookupKey(namespace, name)
	delete(k.ingMap, key)
}

func (k *Cache) GetEndpoints(client unversioned.Interface, namespace, name string) (*api.Endpoints, error) {
	var (
		key = cacheLookupKey(namespace, name)
	)

	logger.Debugf("Looking up Endpoints(%q) in cache", key)
	obj, ok, er := k.endpoints.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		logger.Debugf("Looking up Endpoints(%q) on server", key)
		if en, er := client.Endpoints(namespace).Get(name); er == nil {
			k.endpoints.Add(en)
			return en, nil
		}
		return nil, fmt.Errorf("Could not find Endpoints %q", key)
	}

	s, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("Endpoints cache returned non-Endpoints object")
	}

	logger.Debugf("Found %v", Endpoints(*s))
	return s, nil
}

func (k *Cache) GetService(client unversioned.Interface, namespace, name string) (*api.Service, error) {
	var (
		key = cacheLookupKey(namespace, name)
	)

	logger.Debugf("Looking up Service(%q) in cache", key)
	obj, ok, er := k.service.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		logger.Debugf("Looking up Service(%q) on server", key)
		if sv, er := client.Services(namespace).Get(name); er == nil {
			k.service.Add(sv)
			return sv, nil
		}
		return nil, fmt.Errorf("Could not find Service %q", key)
	}

	s, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("Service cache returned non-Service object")
	}
	logger.Debugf("Found %v", Service(*s))
	return s, nil
}

func (k *Cache) GetIngress(client unversioned.ExtensionsInterface, namespace, name string) (*extensions.Ingress, error) {
	var (
		sk      = cacheLookupKey(namespace, name)
		key, ok = k.ingMap[sk]
	)

	if !ok {
		return nil, fmt.Errorf("No Ingress associated with Service(%q)", sk)
	}

	logger.Debugf("Looking up Ingress(%q) in cache", key)
	obj, ok, er := k.ingress.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		logger.Debugf("Looking up Ingress(%q) on server", key)
		if in, er := client.Ingress(namespace).Get(name); er == nil {
			k.ingress.Add(in)
			return in, nil
		}
		return nil, fmt.Errorf("Could not find Ingress %q", key)
	}

	s, ok := obj.(*extensions.Ingress)
	if !ok {
		return nil, errors.New("Ingress cache returned non-Ingress object")
	}

	logger.Debugf("Found %v", Ingress(*s))
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
