package kubernetes

import (
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/intstr"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/stretchr/testify/assert"
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

func TestDefaultResourceFromIngress(te *testing.T) {
	var (
		is  = assert.New(te)
		m   = NewCache()
		ing = &extensions.Ingress{
			ObjectMeta: api.ObjectMeta{Name: "ingress", Namespace: "test", UID: types.UID("one")},
			Spec: extensions.IngressSpec{
				Backend: &extensions.IngressBackend{
					ServiceName: "service",
					ServicePort: intstr.FromString("web"),
				},
			},
		}
		svc = &api.Service{
			ObjectMeta: api.ObjectMeta{Name: "service", Namespace: "test", UID: types.UID("two")},
			Spec: api.ServiceSpec{
				Type:      api.ServiceTypeClusterIP,
				ClusterIP: "1.2.3.4",
				Ports: []api.ServicePort{
					api.ServicePort{Name: "web", Port: 80, TargetPort: intstr.FromString("http")},
				},
			},
		}
		end = &api.Endpoints{
			ObjectMeta: api.ObjectMeta{Name: "service", Namespace: "test", UID: types.UID("three")},
			Subsets: []api.EndpointSubset{
				api.EndpointSubset{
					Addresses: []api.EndpointAddress{
						api.EndpointAddress{IP: "10.11.12.13"},
						api.EndpointAddress{IP: "10.20.21.23"},
					},
					Ports: []api.EndpointPort{
						api.EndpointPort{Name: "web", Port: 8080, Protocol: api.ProtocolTCP},
					},
				},
			},
		}
	)

	if testing.Verbose() {
		logger.Configure("debug", "[romulus-test] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	m.SetServiceStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
	m.SetEndpointsStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
	m.endpoints.Add(end)
	m.service.Add(svc)

	list := resourcesFromIngress(m, ing)
	te.Logf("Default ResourceList: %v", list)
	is.True(len(list) > 0, "ResourceList should be non-zero")
	ma := list.Map()
	rsc, ok := ma["test.service.web"]
	if is.True(ok, "'test.service.web' not created: %v", list) {
		is.False(rsc.NoServers(), "%v should have servers", rsc)
	}
}

func TestRoutedResourceFromIngress(te *testing.T) {
	var (
		is  = assert.New(te)
		m   = NewCache()
		ing = &extensions.Ingress{
			ObjectMeta: api.ObjectMeta{Name: "ingress", Namespace: "test", UID: types.UID("one")},
			Spec: extensions.IngressSpec{
				Rules: []extensions.IngressRule{
					extensions.IngressRule{
						Host: "www.example.net",
						IngressRuleValue: extensions.IngressRuleValue{
							HTTP: &extensions.HTTPIngressRuleValue{
								Paths: []extensions.HTTPIngressPath{
									extensions.HTTPIngressPath{
										Path: "/foo",
										Backend: extensions.IngressBackend{
											ServiceName: "service",
											ServicePort: intstr.FromString("web"),
										},
									},
								},
							},
						},
					},
				},
			},
		}
		svc = &api.Service{
			ObjectMeta: api.ObjectMeta{
				Name:        "service",
				Namespace:   "test",
				UID:         types.UID("two"),
				Annotations: map[string]string{"romulus/path": "/bar"},
			},
			Spec: api.ServiceSpec{
				Type: api.ServiceTypeClusterIP,
				Ports: []api.ServicePort{
					api.ServicePort{Name: "web", Port: 80, TargetPort: intstr.FromString("http")},
				},
			},
		}
		end = &api.Endpoints{
			ObjectMeta: api.ObjectMeta{Name: "service", Namespace: "test", UID: types.UID("three")},
			Subsets: []api.EndpointSubset{
				api.EndpointSubset{
					Addresses: []api.EndpointAddress{
						api.EndpointAddress{IP: "10.11.12.13"},
						api.EndpointAddress{IP: "10.20.21.23"},
					},
					Ports: []api.EndpointPort{
						api.EndpointPort{Name: "web", Port: 8080, Protocol: api.ProtocolTCP},
					},
				},
			},
		}
	)

	if testing.Verbose() {
		logger.Configure("debug", "[romulus-test] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	m.SetServiceStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
	m.SetEndpointsStore(cache.NewStore(cache.MetaNamespaceKeyFunc))
	m.service.Add(svc)
	m.endpoints.Add(end)

	list := resourcesFromIngress(m, ing)
	te.Logf("Routed ResourceList: %v", list)
	is.True(len(list) > 0, "ResourceList should be non-zero")
	ma := list.Map()
	rsc, ok := ma["test.service.web"]
	if is.True(ok, "'test.service.web' not created: %v", list) {
		is.False(rsc.NoServers(), "%v should have servers", rsc)
		rt := rsc.Route.String()
		is.Equal("Route(host(`www.example.net`) && path(`/foo`))", rt)
	}
}
