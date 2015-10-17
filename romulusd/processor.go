package main

import (
	"fmt"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	uApi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

type metadata struct {
	name, ns, kind      string
	labels, annotations map[string]string
	version             int
}

func getMeta(obj runtime.Object) (m *metadata, e error) {
	m = new(metadata)
	a := meta.NewAccessor()

	switch obj.(type) {
	default:
		m.kind, e = a.Kind(obj)
	case *api.Service:
		m.kind = "Service"
	case *api.Endpoints:
		m.kind = "Endpoints"
	}

	if m.name, e = a.Name(obj); e != nil {
		return
	}
	if m.ns, e = a.Namespace(obj); e != nil {
		return
	}
	if m.labels, e = a.Labels(obj); e != nil {
		return
	}
	if m.annotations, e = a.Annotations(obj); e != nil {
		return
	}
	if ver, e := a.ResourceVersion(obj); e == nil {
		m.version, _ = strconv.Atoi(ver)
	}

	return
}

func processor(in chan event, c context.Context) {
	for {
		select {
		case <-c.Done():
			close(in)
			return
		case e := <-in:
			if registerable(e.Object) {
				debugf("Recieved: %v", e)
				if er := process(e); er != nil {
					errorf(er.Error())
					go retry(in, e)
				}
			} else {
				debugf("Object not registerable: %v", e.Object)
			}
		}
	}
}

func retry(ch chan event, e event) {
	time.Sleep(2 * time.Second)
	ch <- e
}

func process(e event) error {
	if m, _ := getMeta(e.Object); m.version > 0 {
		resourceVersion = fmt.Sprintf("%d", m.version)
	}
	switch e.Type {
	default:
		debugf("Unsupported event type %q: %+v", e.Type, e)
		return nil
	case watch.Error:
		if a, ok := e.Object.(*uApi.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return NewErr(e, "Kubernetes API failure: %s", a.Message)
		}
		return UnknownKubeErr
	case watch.Deleted:
		return remove(e.Object)
	case watch.Added:
		return update(e.Object, "add")
	case watch.Modified:
		return update(e.Object, "mod")
	}
}

func remove(r runtime.Object) error {
	etcd.SetPrefix(getVulcanKey(r))
	switch o := r.(type) {
	case *api.Endpoints:
		return deregisterEndpoints(o)
	case *api.Service:
		return deregisterService(o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}

func update(r runtime.Object, s string) error {
	etcd.SetPrefix(getVulcanKey(r))
	// m, er := getMeta(r)
	// if er != nil {
	// 	return NewErr(er, "Unable to get object metadata")
	// }

	// debugf("Caching {Kind: %q, Name: %q, Namespace: %q}", m.kind, m.name, m.ns)
	// cache.put(cKey{m.name, m.ns, m.kind}, r)
	// cacheIfNewer(cKey{m.name, m.ns, m.kind}, r)

	switch o := r.(type) {
	case *api.Service:
		if oo, ok := cacheIfNewer(cKey{o.Name, o.Namespace, o.Kind}, o); !ok {
			o = oo.(*api.Service)
		}
		// if s == "mod" {
		// 	return registerService(o)
		// }
		// return nil
		return registerService(o)
	case *api.Endpoints:
		if oo, ok := cacheIfNewer(cKey{o.Name, o.Namespace, o.Kind}, o); !ok {
			o = oo.(*api.Endpoints)
		}
		return registerEndpoints(o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}
