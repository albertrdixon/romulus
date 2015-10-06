package romulus

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"
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
		"messy": []string{
			"test/messy/test-endpoints1.yaml",
			"test/messy/test-svc1.yaml",
			"test/messy/test-endpoints2.yaml",
			"test/messy/test-svc2.yaml",
		},
	}

	apiVersion = "v1"
	vulcanKey  = "/vulcand-test"
	selector   = ServiceSelector{"type": "public"}

	singlePortID   = getVulcanID("singlePort", "test", "web")
	apiMultiPortID = getVulcanID("multiPort", "test", "api")
	webMultiPortID = getVulcanID("multiPort", "test", "web")
	singlePort     = []VulcanObject{
		NewBackend(singlePortID),
		NewFrontend(singlePortID, singlePortID, "Host(`www.example.com`)", "Path(`/web`)"),
	}
	multiPort = []VulcanObject{
		NewBackend(apiMultiPortID),
		NewBackend(webMultiPortID),
		NewFrontend(apiMultiPortID, apiMultiPortID, "Host(`www.example.com`)", "Path(`/api/v1`)"),
		NewFrontend(webMultiPortID, webMultiPortID, "Host(`www.example.com`)", "Path(`/blog`)"),
	}
)

func fakeKubeClient(defs string) (*testclient.Fake, testclient.ObjectRetriever) {
	c := &testclient.Fake{}
	o := testclient.NewObjects(api.Scheme, api.Scheme)
	c.AddReactor("*", "*", testclient.ObjectReaction(o, testapi.Default.RESTMapper()))
	for _, d := range definitions[defs] {
		e := testclient.AddObjectsFromPath(d, o, api.Scheme)
		if e != nil {
			panic(e)
		}
	}

	return c, o
}

func setup(t *testing.T) (*assert.Assertions, *require.Assertions, *Registrar) {
	if testing.Verbose() {
		LogLevel("debug")
	}
	is, must := assert.New(t), require.New(t)
	r := &Registrar{
		e:  NewFakeEtcdClient(vulcanKey),
		v:  apiVersion,
		vk: vulcanKey,
		s:  selector.fixNamespace(),
	}

	return is, must, r
}

func TestCleanRegister(te *testing.T) {
	defer LogLevel("info")
	is, must, reg := setup(te)

	var tests = []struct {
		defs, kind, name string
		event            watch.EventType
		valid            bool
		data             []VulcanObject
	}{
		{"single-port", "Endpoints", "singlePort", watch.Added, true, singlePort},
		{"multi-port", "Service", "multiPort", watch.Modified, true, multiPort},
	}

	for _, t := range tests {
		c, o := fakeKubeClient(t.defs)
		reg.k = c

		obj, er := o.Kind(t.kind, t.name)
		is.NoError(er, "Unable to get object '%s-%s'", t.name, t.kind)

		w := Event{watch.Event{t.event, obj}}
		er = event(reg, w)
		te.Logf("Fake Etcd: %v", spew.Sdump(reg.e))
		if t.valid {
			must.NoError(er, "Event handling failed event=%v", spew.Sdump(w))
		} else {
			must.Error(er, "Expected error event=%v", spew.Sdump(w))
			continue
		}

		for _, d := range t.data {
			expVal, _ := d.Val()
			key := prefix(reg.vk, d.Key())
			val, ok := reg.e.(*fakeEtcdClient).get(key)
			is.True(ok, "%q not written to etcd", key)
			is.Equal(expVal, val, "Encoding for '%s-%s' not expected", t.name, t.kind)
		}

		reg.e = NewFakeEtcdClient(vulcanKey)
	}
}

func TestMessyRegister(te *testing.T) {
	defer LogLevel("info")
	// is, _, r, o := setup(te)

}
