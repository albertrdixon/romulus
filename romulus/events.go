package romulus

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

var WatchRetryInterval = 2 * time.Second

type Event struct {
	watch.Event
}

func (e Event) fields() map[string]interface{} {
	return map[string]interface{}{"event": e.Type}
}

type WatchFunc func() (watch.Interface, error)

func initEvents(r *Registrar, c context.Context) (<-chan Event, error) {
	out := make(chan Event, 100)
	s := func() (watch.Interface, error) {
		log().Debug("Attempting to set watch on Services")
		return r.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}
	e := func() (watch.Interface, error) {
		log().Debug("Attempting to set watch on Endpoints")
		return r.k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
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
		if a, ok := e.Object.(*api.Status); ok {
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

func (i ingester) fields() map[string]interface{} {
	return map[string]interface{}{"channel": i.name}
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

func isClosed(e watch.Event) bool {
	return e.Type == watch.Error || e == watch.Event{}
}
