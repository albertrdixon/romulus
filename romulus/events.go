package romulus

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	unvApi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

var WatchRetryInterval = 2 * time.Second

type Event watch.Event

type watchFunc func() (watch.Interface, error)

func initEvents(c context.Context) (chan watch.Event, error) {
	out := make(chan watch.Event, 100)
	k := unversioned.New(conf.KubeConfig)
	s := func() (watch.Interface, error) {
		log().Debug("Attempting to set watch on Services")
		return k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}
	e := func() (watch.Interface, error) {
		log().Debug("Attempting to set watch on Endpoints")
		return k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}

	go ingester{"Services", s}.ingest(out, c)
	go ingester{"Endpoints", e}.ingest(out, c)
	return out, nil
}

func event(r *Registrar, e Event) error {
	switch e.Type {
	default:
		logf(e).Debugf("Unsupported event type")
		return nil
	case watch.Error:
		if a, ok := e.Object.(*unvApi.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return NewErr(e, "Kubernetes API failure: %s", a.Message)
		}
		return UnknownKubeErr
	case watch.Deleted:
		return r.delete(e.Object)
	case watch.Added:
		return r.update(e.Object, "add")
	case watch.Modified:
		return r.update(e.Object, "mod")
	}
}

type ingester struct {
	name string
	fn   WatchFunc
}

type writer string

func (i ingester) fields() map[string]interface{} {
	return map[string]interface{}{"channel": i.name}
}

func (w writer) fields() map[string]interface{} {
	return map[string]interface{}{"channel": "etcd writer"}
}

func (i ingester) watch(out chan<- watch.Interface, c context.Context) {
	t := time.NewTicker(WatchRetryInterval)
	defer t.Stop()

	w, e := i.fn()
	if e == nil {
		out <- w
		return
	}

	for {
		logf(i).Debugf("Setting watch failed, retry in (%v): %v", WatchRetryInterval, e)
		select {
		case <-c.Done():
			return
		case <-t.C:
			w, e := i.fn()
			if e == nil {
				out <- w
				return
			}
		}
	}
}

func (i ingester) ingest(out chan<- Event, c context.Context) {
	var w watch.Interface
	var wc = make(chan watch.Interface, 1)
	defer close(wc)

	for {
		go i.watch(wc, c)
		select {
		case <-c.Done():
			logf(i).Info("Closing ingest channel")
			return
		case w = <-wc:
			logf(i).Debug("Watch set")
		}

	EventLoop:
		for {
			select {
			case <-c.Done():
				logf(i).Info("Closing ingest channel")
				return
			case e := <-w.ResultChan():
				if isClosed(e) {
					logf(i).Warnf("Watch closed: %+v", e)
					break EventLoop
				}
				out <- Event{e}
			}
		}
	}
}

func writer(c context.Context) chan writeEvent {
	go func() {
		
	}
}

func isClosed(e watch.Event) bool {
	return e.Type == watch.Error || e == watch.Event{}
}
