package kubernetes

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/util"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
)

var (
	// FakeKubeClient = &testclient.Fake{}
	Keyspace string

	EverythingSelector = map[string]string{}

	resources = map[string]runtime.Object{
		"services":  &api.Service{},
		"endpoints": &api.Endpoints{},
		"ingresses": &extensions.Ingress{},
	}
)

func NewClient(url, user, pass string, insecure bool) (*unversioned.Client, error) {
	config, er := getKubeConfig(url, user, pass, insecure)
	if er != nil {
		return nil, er
	}
	return unversioned.New(config)
}

func NewExtensionsClient(url, user, pass string, insecure bool) (*unversioned.ExtensionsClient, error) {
	config, er := getKubeConfig(url, user, pass, insecure)
	if er != nil {
		return nil, er
	}
	return unversioned.NewExtensions(config)
}

func getKubeConfig(url, user, pass string, insecure bool) (*unversioned.Config, error) {
	config, er := unversioned.InClusterConfig()
	if er != nil {
		config, er = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if er != nil {
			return nil, er
		}
		config.Host = url
	}

	config.Insecure = insecure
	config.Username = user
	config.Password = pass
	return config, nil
}

// func ResetFakeClient() {
// 	FakeKubeClient = &testclient.Fake{}
// }

func Status(client *Client) error {
	if _, er := client.Client.ServerVersion(); er != nil {
		return er
	}
	_, er := client.ExtensionsClient.ServerVersion()
	return er
}

// func GetID(port intstr.IntOrString, m api.ObjectMeta) string {
// 	id := []string{m.Namespace, m.Name}
// 	if port.Type == intstr.String {
// 		id = append(id, port.String())
// 	}
// 	return strings.Join(id, ".")
// }

func GetSrvID(u *url.URL, m api.ObjectMeta) string {
	id := []string{m.Namespace, m.Name, util.Hashf(md5.New(), u, m.UID)[:hashLen]}
	return strings.Join(id, ".")
}

func CreateStore(kind string, c cache.Getter, sel Selector, resync time.Duration, ctx context.Context) (cache.Store, error) {
	obj, ok := resources[kind]
	if !ok {
		return nil, fmt.Errorf("Object type %q not supported", kind)
	}

	store := cache.NewTTLStore(framework.DeletionHandlingMetaNamespaceKeyFunc, cacheTTL)
	selector := selectorFromMap(sel)
	lw := getListWatch(kind, c, selector)
	cache.NewReflector(lw, obj, store, resync).RunUntil(ctx.Done())
	return store, nil
}

func CreateUpdateController(kind string, w watcher, c cache.Getter, sel Selector, resync time.Duration) (cache.Store, *framework.Controller) {
	obj, ok := resources[kind]
	if !ok {
		return nil, nil
	}

	sl := selectorFromMap(sel)
	handler := framework.ResourceEventHandlerFuncs{
		DeleteFunc: w.Delete,
		UpdateFunc: w.Update,
	}
	return framework.NewInformer(getListWatch(kind, c, sl), obj, resync, handler)
}

func CreateFullController(kind string, w watcher, c cache.Getter, sel Selector, resync time.Duration) (cache.Store, *framework.Controller) {
	obj, ok := resources[kind]
	if !ok {
		return nil, nil
	}

	sl := selectorFromMap(sel)
	handler := framework.ResourceEventHandlerFuncs{
		AddFunc:    w.Add,
		DeleteFunc: w.Delete,
		UpdateFunc: w.Update,
	}
	return framework.NewInformer(getListWatch(kind, c, sl), obj, resync, handler)
}

func getListWatch(kind string, get cache.Getter, selector labels.Selector) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			logger.Debugf("Running ListFunc for %q", kind)
			req := get.Get().Namespace(api.NamespaceAll).Resource(kind).
				LabelsSelectorParam(selector).FieldsSelectorParam(fields.Everything())
			logger.Debugf("Request URL: %v", req.URL())
			obj, er := req.Do().Get()
			if er != nil {
				logger.Debugf("Got error: %v", er)
			}
			return obj, er
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			logger.Debugf("Running WatchFunc for %q", kind)
			req := get.Get().Prefix("watch").Namespace(api.NamespaceAll).Resource(kind).
				LabelsSelectorParam(selector).FieldsSelectorParam(fields.Everything()).
				Param("resourceVersion", options.ResourceVersion)
			logger.Debugf("Request URL: %v", req.URL())
			w, er := req.Watch()
			if er != nil {
				logger.Debugf("Got error: %v", er)
			} else {
				logger.Debugf("Set watch for %q", kind)
			}
			return w, er
		},
	}
}

func selectorFromMap(m Selector) labels.Selector {
	s := labels.Everything()
	for k, val := range m {
		key := k
		if !strings.HasPrefix(k, Keyspace) {
			key = strings.Join([]string{Keyspace, k}, "")
		}
		if req, er := labels.NewRequirement(key, labels.DoubleEqualsOperator, sets.NewString(val)); er != nil {
			s = s.Add(*req)
		}
	}
	return s
}

func ServicesFromIngress(store *KubeCache, in *extensions.Ingress) []*Service {
	var (
		list []*Service
		s    *api.Service
		er   error
	)

	list = []*Service{}
	namespace := in.Namespace
	if in.Spec.Backend != nil {
		name := in.Spec.Backend.ServiceName
		if s, er = store.GetService(namespace, name); er != nil {
			logger.Errorf("Unable to find default service '%s/%s': %v", namespace, name, er)
		} else {
			id := ServiceID(s.ObjectMeta, in.Spec.Backend.ServicePort)
			svc := NewService(id, s.ObjectMeta)
			if er := AddBackendsFromService(in.Spec.Backend.ServicePort, s, svc); er == nil {
				svc.Annotations = mergeAnnotations(in.Annotations, s.Annotations)
				list = append(list, svc)
			} else {
				logger.Warnf(er.Error())
			}
		}
	}

	for _, rule := range in.Spec.Rules {
		for _, node := range rule.HTTP.Paths {
			name := node.Backend.ServiceName
			if s, er = store.GetService(namespace, name); er != nil {
				logger.Errorf("Unable to find service '%s/%s': %v", namespace, name, er)
			} else {
				id := ServiceID(s.ObjectMeta, node.Backend.ServicePort)
				svc := NewService(id, s.ObjectMeta)
				if er := AddBackendsFromService(node.Backend.ServicePort, s, svc); er == nil {
					svc.Annotations = mergeAnnotations(in.Annotations, s.Annotations)
					svc.Route.AddHost(rule.Host)
					svc.Route.AddPath(node.Path)
					list = append(list, svc)
				} else {
					logger.Warnf(er.Error())
				}
			}
		}
	}
	Sort(list, nil)
	return list
}

func (k *KubeCache) GetService(namespace, name string) (*api.Service, error) {
	key := cacheLookupKey(namespace, name)
	obj, ok, er := k.Service.Get(key)
	if er != nil {
		return nil, er
	}
	if !ok {
		return nil, fmt.Errorf("Could not find Service %q", key)
	}
	s, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("Service cache returned non-Service object")
	}
	return s, nil
}

func (s *Service) AddBackend(id, scheme, ip string, port int) {
	if s.Backends == nil {
		s.Backends = make([]*Server, 0, 1)
	}

	server := &Server{id, scheme, ip, port}
	logger.Debugf("[%v] Adding %v", s.ID, server)
	s.Backends = append(s.Backends, server)
}

func (s *Service) GetAnnotation(key string) (val string, ok bool) {
	if !strings.HasPrefix(key, Keyspace) {
		n, k := strings.TrimRight(Keyspace, "/"), strings.TrimLeft(key, "/")
		key = strings.Join([]string{n, k}, "/")
	}
	logger.Debugf("[%v] Looking up annotation key=%q", s.ID, key)
	val, ok = s.Annotations[key]
	return
}

func AddBackendsFromService(port intstr.IntOrString, backend *api.Service, service *Service) error {
	logger.Debugf(`[%v] Look up Port("%v") in %v`, service.ID, port.String(), KubeService(*backend))
	for _, sp := range backend.Spec.Ports {
		logger.Debugf(`[%v] Checking Port(name="%s", port=%d)`, service.ID, sp.Name, sp.Port)
		if sp.Name == port.String() || sp.Port == port.IntValue() {
			id := ServerID(backend.Spec.ClusterIP, sp.Port, backend.ObjectMeta)
			service.AddBackend(id, HTTP, backend.Spec.ClusterIP, sp.Port)
			return nil
		}
	}
	return fmt.Errorf(`[%v] Port("%v") matches no ports in %v`, service.ID, port.String(), KubeService(*backend))
}

const (
	hashLen  = 8
	cacheTTL = 48 * time.Hour

	ServiceKind  = "service"
	ServicesKind = "services"
	IngressKind  = "ingresses"

	HTTP  = "http"
	HTTPS = "https"
)

func Sort(services []*Service, fn func(s1, s2 *Service) bool) {
	sortFn := fn
	if sortFn == nil {
		sortFn = func(s1, s2 *Service) bool {
			return s1.ID < s2.ID
		}
	}
	sort.Sort(&serviceSorter{
		services: services,
		sorter:   sortFn,
	})
}

func (s *serviceSorter) Len() int {
	return len(s.services)
}
func (s *serviceSorter) Swap(i, j int) {
	s.services[i], s.services[j] = s.services[j], s.services[i]
}
func (s *serviceSorter) Less(i, j int) bool {
	return s.sorter(s.services[i], s.services[j])
}

func ServiceReady(s *api.Service) bool {
	return s.Spec.Type == api.ServiceTypeClusterIP && api.IsServiceIPSet(s)
}

func (r *Route) Empty() bool {
	return len(r.Parts) < 1 && len(r.Header) < 1
}

func ServiceID(m api.ObjectMeta, port ...intstr.IntOrString) string {
	id := []string{m.Namespace, m.Name}
	if len(port) > 0 && port[0].Type == intstr.String {
		id = append(id, port[0].String())
	}
	return strings.Join(id, ".")
}

func ServerID(ip string, port int, m api.ObjectMeta) string {
	id := []string{m.Namespace, m.Name, util.Hashf(md5.New(), ip, port, m.UID)[:hashLen]}
	return strings.Join(id, ".")
}

func mergeAnnotations(defaultMap, overrideMap map[string]string) map[string]string {
	out := make(map[string]string)
	for key, val := range defaultMap {
		if strings.HasPrefix(key, Keyspace) {
			if override, ok := overrideMap[key]; ok {
				out[key] = override
			} else {
				out[key] = val
			}
		}
	}
	return out
}

func cacheLookupKey(namespace, name string) cache.ExplicitKey {
	if namespace == "" {
		return cache.ExplicitKey(name)
	}
	k := fmt.Sprintf("%s/%s", namespace, name)
	return cache.ExplicitKey(k)
}
