package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	uApi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

const retryInterval = 2 * time.Second

func processor(in chan *event, c context.Context) {
	for {
		select {
		case <-c.Done():
			close(in)
			return
		case e := <-in:
			if registerable(e.Object) {
				debugf("Recieved: %v", e)
				if er := process(e); er != nil && c.Err() != nil {
					errorf(er.Error())
					if e.retry {
						go retry(in, e)
					}
				}
			} else {
				debugf("Object not registerable: %v", e.Object)
			}
		}
	}
}

func retry(ch chan *event, e *event) {
	debugf("(Retry in %v) %v", retryInterval, e)
	time.Sleep(retryInterval)
	ch <- e
}

func process(e *event) error {
	if isError(e) {
		e.retry = false
		if a, ok := e.Object.(*uApi.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return NewErr(e, "Kubernetes API failure: %s", a.Message)
		}
		return UnknownKubeErr
	}

	etcd.SetPrefix(getVulcanKey(e.Object))
	defer etcd.SetPrefix(*vulcanKey)

	switch e.Type {
	default:
		e.retry = false
		return NewErr(nil, "Unsupported event type: %v", e)
	case watch.Deleted:
		return remove(e.Object)
	case watch.Added, watch.Modified:
		return update(e)
	}
}

func remove(r runtime.Object) error {
	kc, e := kubeClient()
	if e != nil {
		return NewErr(e, "kubernetes API error")
	}

	switch o := r.(type) {
	case *api.Endpoints:
		key := cKey{o.Name, o.Namespace, endpointsType}
		_, e = kc.Endpoints(o.Namespace).Get(o.Name)
		if e == nil {
			cache.del(key)
			return nil
		}
		if kubeIsNotFound(e) {
			if e = deregisterEndpoints(o); e == nil {
				cache.del(key)
				return nil
			}
		}
		return e
	case *api.Service:
		key := cKey{o.Name, o.Namespace, serviceType}
		_, e = kc.Services(o.Namespace).Get(o.Name)
		if e == nil {
			cache.del(key)
			return nil
		}
		if kubeIsNotFound(e) {
			if e = deregisterService(o); e == nil {
				cache.del(key)
				return nil
			}
		}
		return e
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}

func update(e *event) error {
	switch o := e.Object.(type) {
	case *api.Service:
		s, ok, er := getService(o.Name, o.Namespace, e.t)
		if !ok {
			return er
		}

		en, ok, er := getEndpoints(o.Name, o.Namespace, e.t)
		if !ok {
			if er == nil {
				warnf("Could not find Endpoints for Service '%s-%s'", o.Name, o.Namespace)
			}
			return er
		}

		if s.moreRecent(e.t) || en.moreRecent(e.t) {
			debugf("Event is old, rejecting (%v)", e)
			return nil
		}

		resourceVersion = o.ResourceVersion
		ep, _ := en.obj.(*api.Endpoints)
		return register(o, ep)
	case *api.Endpoints:
		en, ok, er := getEndpoints(o.Name, o.Namespace, e.t)
		if !ok {
			return er
		}

		s, ok, er := getService(o.Name, o.Namespace, e.t)
		if !ok {
			if er == nil {
				warnf("Could not find Service for Endpoints '%s-%s'", o.Name, o.Namespace)
			}
			return er
		}

		if s.moreRecent(e.t) || en.moreRecent(e.t) {
			debugf("Event is old, rejecting (%v)", e)
			return nil
		}

		resourceVersion = o.ResourceVersion
		sv, _ := s.obj.(*api.Service)
		return register(sv, o)
	default:
		return NewErr(nil, "Unsupported api object: %v", o)
	}
}
