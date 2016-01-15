package kubernetes

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bradfitz/slice"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
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

	HTTP  = "http"
	HTTPS = "https"
	TCP   = "tcp"
)

type watcher interface {
	Add(obj interface{})
	Delete(obj interface{})
	Update(old, next interface{})
}

type Resource struct {
	*Route
	port        api.ServicePort
	id, uid     string
	annotations map[string]string
	servers     ServerList
	websocket   bool
}

type ResourceList []*Resource

type Server struct {
	id, scheme, ip string
	port           int
	websocket      bool
}

type ServerList []*Server

type Route struct {
	header map[string]string
	parts  map[string]string
	regex  map[string]*regexp.Regexp
}

type Sorter struct {
	resources ResourceList
	sorter    func(a, b *Resource) bool
}

type Client struct {
	*unversioned.Client
}

type Selector map[string]string

type Cache map[string]cache.Store

type Ingress extensions.Ingress
type Service api.Service
type Endpoints api.Endpoints

func (i Ingress) String() string {
	return fmt.Sprintf("Ingress(Name=%q, Namespace=%q)", i.ObjectMeta.Name, i.ObjectMeta.Namespace)
}

func (s Service) String() string {
	return fmt.Sprintf(`Service(Name=%q, Namespace=%q)`, s.ObjectMeta.Name, s.ObjectMeta.Namespace)
}

func (e Endpoints) String() string {
	return fmt.Sprintf(`Endpoints(Name=%q, Namespace=%q, Subsets=%d)`, e.ObjectMeta.Name, e.ObjectMeta.Namespace, len(e.Subsets))
}

func (s Service) IsFrontend() bool {
	key := path.Join(Keyspace, "frontend")
	val := s.ObjectMeta.Annotations[key]
	ok, _ := strconv.ParseBool(val)
	return ok
}

func (r Resource) String() string {
	return fmt.Sprintf("Resource(ID=%q, Route=%v, Servers=%v, Annotations=%v)", r.id, r.Route, r.servers, r.annotations)
}

func (r ResourceList) String() string {
	list := make([]string, 0, 1)
	for i := range r {
		list = append(list, r[i].String())
	}
	return fmt.Sprintf("Resources[ %s ]", strings.Join(list, ",  "))
}

func (s ServerList) String() string {
	list := make([]string, 0, len(s))
	for i := range s {
		list = append(list, s[i].String())
	}
	return fmt.Sprintf("[%s]", strings.Join(list, ", "))
}

func (r Route) String() string {
	rt := []string{}
	for k, v := range r.parts {
		rt = append(rt, fmt.Sprintf("%s(`%s`)", strings.Title(k), v))
	}
	for k, v := range r.regex {
		rt = append(rt, fmt.Sprintf("%sRegex(`%v`)", strings.Title(k), v))
	}
	for k, v := range r.header {
		rt = append(rt, fmt.Sprintf("Header(`%s`, `%s`)", k, v))
	}
	slice.Sort(rt, func(i, j int) bool {
		return rt[i] < rt[j]
	})
	return fmt.Sprintf("Route(%s)", strings.Join(rt, " && "))
}

func (s Server) String() string {
	return fmt.Sprintf(`Server(url="%v")`, s.URL())
}
