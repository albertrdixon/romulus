package kubernetes

import (
	"fmt"
	"strings"

	"github.com/albertrdixon/gearbox/url"
	"github.com/bradfitz/slice"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

type watcher interface {
	Add(obj interface{})
	Delete(obj interface{})
	Update(old, next interface{})
}

type Service struct {
	*Route
	ID          string
	Annotations map[string]string
	Backends    []*Server
	UID         string
}

func NewService(id string, meta api.ObjectMeta) *Service {
	return &Service{
		ID:          id,
		Route:       newRoute(),
		Annotations: meta.Annotations,
		UID:         string(meta.UID),
		Backends:    make([]*Server, 0, 1),
	}
}

type Server struct {
	ID, Scheme, IP string
	Port           int
}

func (s *Server) URL() *url.URL {
	ur, _ := url.Parse(fmt.Sprintf("%s://%s:%d", s.Scheme, s.IP, s.Port))
	return ur
}

type Route struct {
	Header map[string]string
	Parts  map[string]string
}

func newRoute() *Route {
	return &Route{Parts: make(map[string]string), Header: make(map[string]string)}
}

func (r *Route) AddHost(host string)            { r.Parts["host"] = host }
func (r *Route) AddPath(path string)            { r.Parts["path"] = path }
func (r *Route) AddMethod(method string)        { r.Parts["method"] = strings.ToUpper(method) }
func (r *Route) AddHeader(header, value string) { r.Header[header] = value }
func (r *Route) GetParts() map[string]string    { return r.Parts }
func (r *Route) GetHeader() map[string]string   { return r.Header }

type serviceSorter struct {
	services []*Service
	sorter   func(s1, s2 *Service) bool
}

type Client struct {
	*unversioned.Client
	*unversioned.ExtensionsClient
}

type Selector map[string]string

type KubeCache struct {
	Ingress, Service cache.Store
}

type KubeIngress extensions.Ingress
type KubeService api.Service

func (i KubeIngress) String() string {
	return fmt.Sprintf("Ingress(Name=%q, Namespace=%q)", i.ObjectMeta.Name, i.ObjectMeta.Namespace)
}

func (s KubeService) String() string {
	return fmt.Sprintf(`Service(Name=%q, Namespace=%q)`, s.ObjectMeta.Name, s.ObjectMeta.Namespace)
}

func (s Service) String() string {
	return fmt.Sprintf("Service(backends=%v, route=%v, meta=%v)", s.Backends, s.Route, s.Annotations)
}

func (r Route) String() string {
	rt := []string{}
	for k, v := range r.Parts {
		rt = append(rt, fmt.Sprintf("%s(`%s`)", strings.Title(k), v))
	}
	for k, v := range r.Header {
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
