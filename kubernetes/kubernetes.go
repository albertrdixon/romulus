package kubernetes

import (
	"fmt"
	"regexp"
	"time"

	"golang.org/x/net/context"

	"github.com/albertrdixon/gearbox/logger"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
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

	extensionsObj = map[string]struct{}{
		"ingresses": struct{}{},
	}

	validScheme = regexp.MustCompile(`(?:wss?|https?)`)
)

const (
	hashLen  = 8
	cacheTTL = 48 * time.Hour

	ServiceKind   = "service"
	ServicesKind  = "services"
	IngressKind   = "ingress"
	IngressesKind = "ingresses"
	EndpointsKind = "endpoints"

	HostPart   = "host"
	PathPart   = "path"
	PrefixPart = "prefix"
	MethodPart = "method"
	HeaderPart = "header"

	HostKey    = "host"
	PathKey    = "path"
	PrefixKey  = "prefix"
	MethodsKey = "methods"
	HeadersKey = "headers"

	HTTP  = "http"
	HTTPS = "https"
	TCP   = "tcp"
)

func NewClient(kubeapi, user, pass string, insecure bool) (*Client, error) {
	config, er := getKubeConfig(kubeapi, user, pass, insecure)
	if er != nil {
		return nil, er
	}

	cl, er := unversioned.New(config)
	if er != nil {
		return nil, er
	}

	return &Client{Client: cl}, nil
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

func (c *Client) GetExtensionsClient() *unversioned.ExtensionsClient {
	return c.Client.ExtensionsClient
}

func (c *Client) GetDiscoveryClient() *unversioned.DiscoveryClient {
	return c.Client.DiscoveryClient
}

func (c *Client) GetUnversionedClient() *unversioned.Client {
	return c.Client
}

func Status(client *Client) error {
	_, er := client.ServerVersion()
	return er
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

func getListWatch(kind string, getter cache.Getter, selector labels.Selector) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			logger.Debugf("Running ListFunc for %q", kind)
			req := getter.Get().Namespace(api.NamespaceAll).Resource(kind).
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
			req := getter.Get().Prefix("watch").Namespace(api.NamespaceAll).Resource(kind).
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

func LogCallback(callback string, obj interface{}) {
	format := "%s %s"
	switch t := obj.(type) {
	default:
		logger.Warnf(format, callback, "<unknown>")
	case *extensions.Ingress:
		logger.Infof(format, callback, Ingress(*t))
	case *api.Service:
		logger.Infof(format, callback, Service(*t))
	case *api.Endpoints:
		logger.Infof(format, callback, Endpoints(*t))
	}
}
