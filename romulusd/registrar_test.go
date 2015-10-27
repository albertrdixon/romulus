package main

import (
	"testing"

	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/davecgh/go-spew/spew"
)

var (
	definitions = map[string][]string{
		"single-port": []string{
			"test/single-port-endpoints.yaml",
			"test/single-port-svc.yaml",
		},
		"multi-port": []string{
			"test/multi-port-endpoints.yaml",
			"test/multi-port-svc.yaml",
		},
		"resource-ver": []string{
			"test/resourceVer-svc.yaml",
			"test/resourceVer-endpoints.yaml",
			"test/resourceVer-svc2.yaml",
		},
		"messy": []string{
			"test/messy-one-svc.yaml",
			"test/messy-two-endpoints.yaml",
			// "test/messy-three-svc.yaml",
			"test/messy-four-endpoints.yaml",
			"test/messy-five-svc.yaml",
			"test/messy-six-svc.yaml",
		},
	}

	apiVersion = "v1"
	selector   = map[string]string{"type": "public"}

	singlePortID   = getVulcanID("singlePort", "test", "web")
	apiMultiPortID = getVulcanID("multiPort", "test", "api")
	webMultiPortID = getVulcanID("multiPort", "test", "web")
	resourceVerID  = getVulcanID("resource", "test", "web")
	singlePort     = []VulcanObject{
		NewBackend(singlePortID),
		NewFrontend(singlePortID, singlePortID, "Host(`www.example.com`)", "Path(`/web`)"),
	}
	multiPort = []VulcanObject{
		&Backend{ID: apiMultiPortID, Type: WS},
		NewBackend(webMultiPortID),
		&Frontend{ID: apiMultiPortID, BackendID: apiMultiPortID, Type: WS, Route: "Host(`www.example.com`) && Path(`/api/v1`)"},
		NewFrontend(webMultiPortID, webMultiPortID, "Host(`www.example.com`)", "Path(`/blog`)"),
	}
	resourceVer = []VulcanObject{
		NewBackend(resourceVerID),
		NewFrontend(resourceVerID, resourceVerID, "Host(`www.example.com`)", "Path(`/web`)"),
	}
)

func TestBasicRegister(te *testing.T) {
	is, _ := setup(te)

	var tests = []struct {
		defs, kind, name string
		event            watch.EventType
		valid            bool
		data             []VulcanObject
	}{
		{"single-port", endpointsType, "singlePort", watch.Added, true, singlePort},
		{"multi-port", serviceType, "multiPort", watch.Modified, true, multiPort},
		{"resource-ver", serviceType, "resource", watch.Modified, true, resourceVer},
	}

	for _, t := range tests {
		newFakeEtcdClient(*vulcanKey)
		o := fakeKubeClient(t.defs)

		obj := fakeObject(o, t.kind, t.name)
		w := newEvent(t.event, obj)
		if t.valid && !is.NoError(process(w)) {
			te.Logf("Fake Etcd: %v", spew.Sdump(etcd))
		}

		for _, d := range t.data {
			expVal, _ := d.Val()
			key := prefix(*vulcanKey, d.Key())
			val, er := etcd.Val(d.Key())
			is.NoError(er, "%q not written to etcd", key)
			is.Equal(expVal, val, "Encoding for '%s-%s' not expected", t.name, t.kind)
		}
	}
}

func TestMessyRegister(te *testing.T) {
	is, _ := setup(te)
	var w *event

	fEtcd := newFakeEtcdClient(*vulcanKey)
	o := fakeKubeClient("")

	addObject(o, definitions["messy"][0])
	obj := fakeObject(o, serviceType, "oneTwoThree")
	w = newEvent(watch.Added, obj)
	is.NoError(process(w))
	v, ok := fEtcd.k[*vulcanKey+"/frontends/web.oneTwoThree.test"]
	if !is.False(ok) || !is.Empty(v) {
		debugf("FRONTEND web.oneTwoThree.test EXISTS AND SHOULD NOT")
		te.Log("FRONTEND web.oneTwoThree.test EXISTS AND SHOULD NOT")
		te.Log(spew.Sdump(etcd))
	}

	addObject(o, definitions["messy"][1])
	obj = fakeObject(o, endpointsType, "oneTwoThree")
	w = newEvent(watch.Added, obj)
	is.NoError(process(w))
	v, _ = fEtcd.k[*vulcanKey+"/frontends/web.oneTwoThree.test/frontend"]
	f, _ := NewFrontend("web.oneTwoThree.test", "web.oneTwoThree.test", "Host(`www.example.com`)", "Path(`/web`)").Val()
	if !is.Equal(f, v) {
		debugf("FRONTEND web.oneTwoThree.test DOES NOT EXIST AND SHOULD")
		te.Log("FRONTEND web.oneTwoThree.test DOES NOT EXIST AND SHOULD")
		te.Log(spew.Sdump(etcd))
	}
	v, _ = fEtcd.k[*vulcanKey+"/backends/web.oneTwoThree.test/backend"]
	b, _ := NewBackend("web.oneTwoThree.test").Val()
	if !is.Equal(b, v) {
		debugf("BACKEND web.oneTwoThree.test NOT CONFIGURED CORRECTLY")
		te.Log("BACKEND web.oneTwoThree.test NOT CONFIGURED CORRECTLY")
		te.Log(spew.Sdump(etcd))
	}

	o = fakeKubeClient("")
	addObject(o, definitions["messy"][2])
	obj = fakeObject(o, endpointsType, "fourFiveSix")
	w = newEvent(watch.Added, obj)
	is.NoError(process(w))
	v, ok = fEtcd.k[*vulcanKey+"/backends/api.fourFiveSix.test/backend"]
	if !is.False(ok) || !is.Empty(v) {
		debugf("BACKEND api.fourFiveSix.test EXISTS AND SHOULD NOT")
		te.Log("BACKEND api.fourFiveSix.test EXISTS AND SHOULD NOT")
		te.Log(spew.Sdump(etcd))
	}

	fEtcd.k[*vulcanKey+"/backends/web.fourFiveSix.test/servers/old.svc.test-1234"] = `{"URL":"http://2.2.2.2:80"}`
	fEtcd.k[*vulcanKey+"/backends/web.fourFiveSix.test/servers/old.svc.test-5678"] = `{"URL":"http://5.5.5.5:80"}`

	addObject(o, definitions["messy"][3])
	obj = fakeObject(o, serviceType, "fourFiveSix")
	w = newEvent(watch.Added, obj)
	is.NoError(process(w))
	v, _ = fEtcd.k[*vulcanKey+"/backends/api.fourFiveSix.test/backend"]
	b, _ = NewBackend("api.fourFiveSix.test").Val()
	if !is.Equal(b, v) {
		debugf("BACKEND api.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log("BACKEND api.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log(spew.Sdump(etcd))
	}
	v, _ = fEtcd.k[*vulcanKey+"/frontends/web.fourFiveSix.test/frontend"]
	f, _ = NewFrontend("web.fourFiveSix.test", "web.fourFiveSix.test", "Host(`www.example.com`)", "Path(`/blog`)").Val()
	if !is.Equal(f, v) {
		debugf("FRONTEND web.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log("FRONTEND web.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log(spew.Sdump(etcd))
	}
	v, ok = fEtcd.k[*vulcanKey+"/backends/web.fourFiveSix.test/servers/old.svc.test-1234"]
	if !is.False(ok) || !is.Empty(v) {
		debugf("SERVER old.svc.test-1234 EXISTS AND SHOULD NOT")
		te.Log("SERVER old.svc.test-1234 EXISTS AND SHOULD NOT")
		te.Log(spew.Sdump(etcd))
	}

	addObject(o, definitions["messy"][4])
	obj = fakeObject(o, serviceType, "fourFiveSix")
	w = newEvent(watch.Modified, obj)
	is.NoError(process(w))
	v, _ = fEtcd.k[*vulcanKey+"/frontends/api.fourFiveSix.test/frontend"]
	fr := NewFrontend("api.fourFiveSix.test", "api.fourFiveSix.test", "Host(`www.something.else`)", "Path(`/api/v2`)")
	fr.Settings = NewFrontendSettings([]byte(`{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}`))
	f, _ = fr.Val()
	if !is.Equal(f, v) {
		debugf("FRONTEND api.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log("FRONTEND api.fourFiveSix.test NOT CONFIGURED CORRECTLY")
		te.Log(spew.Sdump(etcd))
	}
}

func TestBadRetry(te *testing.T) {
	is, _ := setup(te)
	var (
		obj     runtime.Object
		wOld, w *event
		v, f    string
	)

	fEtcd := newFakeEtcdClient(*vulcanKey)
	o := fakeKubeClient("")

	addObject(o, definitions["resource-ver"][2])
	obj = fakeObject(o, serviceType, "resource")
	wOld = newEvent(watch.Added, obj)
	addObject(o, definitions["resource-ver"][0])
	obj = fakeObject(o, serviceType, "resource")
	w = newEvent(watch.Modified, obj)
	is.NoError(process(w))
	addObject(o, definitions["resource-ver"][1])
	obj = fakeObject(o, endpointsType, "resource")
	w = newEvent(watch.Modified, obj)
	is.NoError(process(w))
	addObject(o, definitions["resource-ver"][2])

	is.NoError(process(wOld))
	v, _ = fEtcd.k[*vulcanKey+"/frontends/web.resource.test/frontend"]
	f, _ = resourceVer[1].Val()
	if !is.Equal(f, v) {
		debugf("FRONTEND web.resource.test NOT CONFIGURED CORRECTLY")
		te.Log("FRONTEND web.resource.test NOT CONFIGURED CORRECTLY")
		te.Log(spew.Sdump(etcd))
	}
}
