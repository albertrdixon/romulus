package kubernetes

import (
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoute(te *testing.T) {
	is := assert.New(te)

	var tests = []struct {
		expected    string
		annotations map[string]string
	}{
		{"Route()", map[string]string{}},
		{"Route(host(`abc`) && prefix(`/f`))", map[string]string{"host": "abc", "prefix": "/f"}},
		{"Route(method(`GET`) && method(`POST`))", map[string]string{"methods": "get; post"}},
		{
			"Route(header(`X-Foo`, `Bar`) && headerRegexp(`X-Bif`, `Baz.*`))",
			map[string]string{"headers": "X-Foo=Bar; X-Bif=|Baz.*|"},
		},
		{
			"Route(hostRegexp(`.*local`) && pathRegexp(`/f/b.*`))",
			map[string]string{"host": "|.*local|", "path": "|/f/b.*|"},
		},
	}

	for _, t := range tests {
		r := NewRoute("foo", t.annotations)
		is.Equal(t.expected, r.String())
	}
}

func TestGenResources(te *testing.T) {
	var (
		is    = assert.New(te)
		must  = require.New(te)
		tests = []struct {
			category         string
			ingName, svcName string
			route            string
			numSrvs          int
		}{
			{"default", "foo", "bar", "Route()", 2},
			{"route-ingress", "bif", "baz", "Route(host(`www.example.net`) && path(`/foo`))", 3},
		}
	)

	if testing.Verbose() {
		logger.Configure("debug", "[romulus-test] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	for _, test := range tests {
		var (
			m   = NewCache()
			ing = new(extensions.Ingress)
			svc = new(api.Service)
			end = new(api.Endpoints)
		)
		objectFromFile(te, test.category, "ingress", ing)
		objectFromFile(te, test.category, "svc", svc)
		objectFromFile(te, test.category, "endpoints", end)

		m.SetServiceStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
		m.SetEndpointsStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
		m.SetIngressStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
		m.endpoints.Add(end)
		m.service.Add(svc)
		m.ingress.Add(ing)
		m.MapServiceToIngress("test", test.svcName, test.ingName)

		c := newFakeClient()
		c.updateFake(svc, end)
		c.updateFakeExp(ing)

		fromIng, ingEr := GenResources(m, c, ing)
		fromSvc, svcEr := GenResources(m, c, svc)
		fromEnd, endEr := GenResources(m, c, end)
		must.NotEmpty(fromIng, "[%s] ResourceList should be non-zero: %v", test.category, fromIng)
		must.NoError(ingEr, "[%s] GenResources(Ingress): %v", test.category, ingEr)
		must.NotEmpty(fromSvc, "ResourceList should be non-zero: %v", fromSvc)
		must.NoError(svcEr, "[%s] GenResources(Service): %v", test.category, svcEr)
		must.NotEmpty(fromEnd, "ResourceList should be non-zero: %v", fromEnd)
		must.NoError(endEr, "[%s] GenResources(Endpoints): %v", test.category, endEr)

		is.EqualValues(fromIng, fromSvc, "[%s]\nfrom_ingress: %v\nfrom_service: %v", test.category, fromIng, fromSvc)
		is.EqualValues(fromIng, fromEnd, "[%s]\nfrom_ingress  : %v\nfrom_endpoints: %v", test.category, fromIng, fromEnd)

		obj := map[string]map[string]*Resource{"from_ingress": fromIng.Map(), "from_service": fromSvc.Map(), "from_endpoints": fromEnd.Map()}
		for cat, ma := range obj {
			id := "test." + test.svcName + ".web"
			r, ok := ma[id]
			if is.True(ok, "[%s] [%s] Resource(%q) not created: %v", test.category, cat, id, fromIng) {
				is.Equal(test.numSrvs, len(r.Servers()), "[%s] [%s] Resource should have %d Servers: %v", test.category, cat, test.numSrvs, r)
				is.Equal(test.route, r.Route.String(), "[%s] [%s] Resource route should be %q: %v", test.category, cat, test.route, r)
			}
		}
	}
}
