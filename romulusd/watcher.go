package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	uapi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

type watchFunc func() (watch.Interface, error)

type event struct {
	watch.Event
	t     time.Time
	retry bool
}

func (e event) String() string {
	m, er := getMeta(e.Object)
	if er != nil {
		return fmt.Sprintf("Event: type=%v object=Unknown", e.Type)
	}
	if s, ok := e.Object.(*uapi.Status); ok {
		return fmt.Sprintf("Status: [%s] code=%d %q", s.Status, s.Code, s.Message)
	}
	return fmt.Sprintf(
		"Event: [%v] object={Kind: %q, Name: %q, Namespace: %q} registerable=%v",
		e.Type, m.kind, m.name, m.ns, registerable(e.Object),
	)
}

func startWatches(c context.Context) (chan *event, error) {
	resourceVersion = ""
	out := make(chan *event, 100)
	kc, er := kubeClient()
	if er != nil {
		return out, er
	}
	sv := func() (watch.Interface, error) {
		debugf("Attempting to set watch on Services")
		return kc.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}
	en := func() (watch.Interface, error) {
		debugf("Attempting to set watch on Endpoints")
		return kc.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
	}

	go watcher("Services", sv, out, c)
	go watcher(endpointsType, en, out, c)
	return out, nil
}

func acquireWatch(fn watchFunc, out chan<- watch.Interface, c context.Context) {
	retry := 2 * time.Second
	t := time.NewTicker(retry)
	defer t.Stop()

	w, e := fn()
	if e == nil && c.Err() == nil {
		out <- w
		return
	}

	for {
		debugf("Setting watch failed, retry in (%v): %v", retry, e)
		select {
		case <-c.Done():
			return
		case <-t.C:
			w, e := fn()
			if e == nil && c.Err() == nil {
				out <- w
				return
			}
		}
	}
}

func watcher(name string, fn watchFunc, out chan<- *event, c context.Context) {
	var w watch.Interface
	var wc = make(chan watch.Interface, 1)
	defer close(wc)

Acquire:
	go acquireWatch(fn, wc, c)
	select {
	case <-c.Done():
		infof("Closing %s watch channel", name)
		return
	case w = <-wc:
		debugf("%s watch set", name)
	}

EventLoop:
	for {
		select {
		case <-c.Done():
			infof("Closing %s watch channel", name)
			return
		case ev := <-w.ResultChan():
			e := &event{ev, time.Now(), true}
			switch {
			case isClosed(e):
				warnf("%s watch closed: %v", name, e)
				goto Acquire
			case isError(e):
				errorf("%s watch error: %v", name, e)
				goto EventLoop
			case c.Err() == nil:
				out <- e
			}
		}
	}
}

func isClosed(e *event) bool {
	return e.Event == watch.Event{}
}

func isError(e *event) bool {
	return e.Type == watch.Error
}
