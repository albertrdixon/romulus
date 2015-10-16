package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

func setup(t *testing.T) (*assert.Assertions, *require.Assertions) {
	if testing.Verbose() {
		*logLevel, *debug = "debug", true
		setupLog()
	}
	test = true
	cache = newCache()
	*vulcanKey = "/vulcand-test"
	return assert.New(t), require.New(t)
}

func fakeKubeClient(defs string) testclient.ObjectRetriever {
	c := &testclient.Fake{}
	o := testclient.NewObjects(api.Scheme, api.Scheme)
	c.AddReactor("*", "*", testclient.ObjectReaction(o, testapi.Default.RESTMapper()))
	for _, d := range definitions[defs] {
		e := testclient.AddObjectsFromPath(d, o, api.Scheme)
		if e != nil {
			panic(e)
		}
	}

	tKubeClient = c
	return o
}

func addObject(o testclient.ObjectRetriever, f string) {
	testclient.AddObjectsFromPath(f, o, api.Scheme)
}

func fakeObject(o testclient.ObjectRetriever, kind, name string) runtime.Object {
	obj, _ := o.Kind(kind, name)
	switch obj := obj.(type) {
	case *api.Service:
		obj.Kind = kind
	case *api.Endpoints:
		obj.Kind = kind
	}
	return obj
}

func newEvent(t watch.EventType, o runtime.Object) event {
	e := event{watch.Event{Type: t, Object: o}}
	debugf("%v", e)
	return e
}
