package romulus

import (
	"time"

	"github.com/prometheus/common/log"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

type watchFunc func() (watch.Interface, error)

func acquireWatch(fn watchFunc, out chan<- watch.Interface, c context.Context) {
	retry := 2 * time.Second
	t := time.NewTicker(retry)
	defer t.Stop()

	w, e := fn()
	if e == nil {
		out <- w
		return
	}

	for {
		log.Debugf("Setting watch failed, retry in (%v): %v", retry, e)
		select {
		case <-c.Done():
			return
		case <-t.C:
			w, e := fn()
			if e == nil {
				out <- w
				return
			}
		}
	}
}

func startWatches(c context.Context) <-chan watch.Event {
	out := make(chan watch.Event, 100)
	kc := kubeClient()
	sv := func() (watch.Interface, error) {
		log.Debug("Attempting to set watch on Services")
		return kc.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}
	en := func() (watch.Interface, error) {
		log.Debug("Attempting to set watch on Endpoints")
		return kc.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}

	go watch("Services", sv, out, c)
	go watch("Endpoints", en, out, c)
	return out
}

func watch(name string, fn watchFunc, out chan<- watch.Event, c context.Context) {
	var w watch.Interface
	var wc = make(chan watch.Interface, 1)
	defer close(wc)

Acquire:
	go acquireWatch(fn, wc, c)
	select {
	case <-c.Done():
		log.Infof("Closing %s watch channel", name)
		return
	case w = <-wc:
		log.Debug("%s watch set", name)
	}

	for {
		select {
		case <-c.Done():
			log.Infof("Closing %s watch channel", name)
			return
		case e := <-w.ResultChan():
			if isClosed(e) {
				log.Warnf("%s watch closed: %+v", name, e)
				goto Acquire
			}
			out <- e
		}
	}

}
