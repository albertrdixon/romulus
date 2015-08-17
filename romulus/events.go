package romulus

import (
	"fmt"

	"github.com/cenkalti/backoff"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

type Event struct {
	watch.Event
}

func (e Event) fields() map[string]interface{} {
	return map[string]interface{}{"event": e.Type}
}

type WatchFunc func() (watch.Interface, error)

func initEvents(r *Registrar, c context.Context) (<-chan Event, error) {
	out := make(chan Event, 100)
	s, er := setWatch(func() (watch.Interface, error) {
		return r.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	})
	if er != nil {
		return nil, er
	}
	e, er := setWatch(func() (watch.Interface, error) {
		return r.k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	})
	if er != nil {
		return nil, er
	}

	go ingester("Services").ingest(s, out, c)
	go ingester("Endpoints").ingest(e, out, c)
	return out, nil
}

func setWatch(wf WatchFunc) (watch.Interface, error) {
	var w watch.Interface
	fn := func() error {
		wa, e := wf()
		if e != nil {
			return e
		}
		w = wa
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = KubeRetryLimit
	if e := backoff.Retry(fn, b); e != nil {
		return nil, NewErr(e, "kubernetes error")
	}
	return w, nil
}

func event(r *Registrar, e Event) error {
	logf(e).Debug("Got a kubernetes API event")
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

type ingester string

func (i ingester) fields() map[string]interface{} {
	return map[string]interface{}{"channel": (string)(i)}
}

func (i ingester) ingest(in watch.Interface, out chan<- Event, c context.Context) {
	for {
		select {
		case <-c.Done():
			logf(i).Debug("Closing ingest channel")
			return
		case e := <-in.ResultChan():
			out <- Event{e}
		}
	}
}
