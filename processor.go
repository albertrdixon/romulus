package romulus

import (
	"fmt"
	"time"

	"github.com/prometheus/common/log"
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
}

func getMeta(obj runtime.Object) (m *metadata, e error) {
	var (
		name, ns, kind      string
		labels, annotations map[string]string
		a                   meta.MetadataAccessor
	)
	a = meta.NewAccessor()

	name, e = a.Name(obj)
	if e != nil {
		return
	}
	ns, e = a.Namespace(obj)
	if e != nil {
		return
	}
	kind, e = a.Kind(obj)
	if e != nil {
		return
	}
	labels, e = a.Labels(obj)
	if e != nil {
		return
	}
	annotations, e = a.Annotations(obj)
	if e != nil {
		return
	}

	m = &metadata{name, ns, kind, labels, annotations}
	return
}

func processor(in chan watch.Event, c context.Context) {
	for {
		select {
		case <-c.Done():
			return
		case e := <-in:
			if registerable(e) {
				if er := process(e); er != nil {
					log.Error(er.Error())
					go retry(in, e)
				}
			}
		}
	}
}

func retry(ch chan watch.Event, e watch.Event) {
	time.Sleep(2 * time.Second)
	ch <- e
}

func process(e watch.Event) error {
	switch e.Type {
	default:
		log.Debugf("Unsupported event type %q: %+v", e.Type, e)
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
	m, er := getMeta(r)
	if er != nil {
		return NewErr(e, "Unable to get object metadata")
	}
	cache.put(cKey{m.name, m.ns, m.kind}, r)

	switch o := r.(type) {
	case *api.Service:
		if s == "mod" {
			return registerService(o)
		}
		return nil
	case *api.Endpoints:
		return registerEndpoints(o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}
