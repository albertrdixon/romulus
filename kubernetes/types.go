package kubernetes

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/bradfitz/slice"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

type Updater interface {
	Add(obj interface{})
	Delete(obj interface{})
	Update(old, next interface{})
}

type SuperClient interface {
	unversioned.Interface
	unversioned.ExtensionsInterface
}

type Client struct {
	*unversioned.Client
}

type Cache struct {
	ingress, service, endpoints cache.Store
	ingMap                      map[cache.ExplicitKey]cache.ExplicitKey
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
	parts []*routePart
}

type routePart struct {
	kind, header, value string
	regex               bool
}

type resourceListSorter struct {
	resources ResourceList
	sorter    func(a, b *Resource) bool
}

type Selector map[string]string

type Ingress extensions.Ingress
type Service api.Service
type Endpoints api.Endpoints
type ingressBackend extensions.IngressBackend

func (i Ingress) String() string {
	var (
		f = "Ingress(Name=%q, Namespace=%q, DefBackend=%v, Rules=%d)"
	)

	if i.Spec.Backend == nil {
		return fmt.Sprintf(f, i.ObjectMeta.Name, i.ObjectMeta.Namespace, "", len(i.Spec.Rules))
	}
	return fmt.Sprintf(f, i.ObjectMeta.Name, i.ObjectMeta.Namespace, ingressBackend(*i.Spec.Backend), len(i.Spec.Rules))
}

func (s Service) String() string {
	return fmt.Sprintf(`Service(Name=%q, Namespace=%q)`, s.ObjectMeta.Name, s.ObjectMeta.Namespace)
}

func (e Endpoints) String() string {
	return fmt.Sprintf(`Endpoints(Name=%q, Namespace=%q, Subsets=%d)`, e.ObjectMeta.Name, e.ObjectMeta.Namespace, len(e.Subsets))
}

func (i ingressBackend) String() string {
	return fmt.Sprintf("%s:%v", i.ServiceName, i.ServicePort.String())
}

func (s Service) IsFrontend() bool {
	key := path.Join(Keyspace, "frontend")
	val := s.ObjectMeta.Annotations[key]
	ok, _ := strconv.ParseBool(val)
	return ok
}

func (r Resource) String() string {
	return fmt.Sprintf("Resource(ID=%q, UID=%q, Route=%v, Servers=%v, Annotations=%v)",
		r.id, r.uid, r.Route, r.servers, r.annotations)
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
	for _, part := range r.parts {
		rt = append(rt, part.String())
	}
	slice.Sort(rt, func(i, j int) bool {
		return rt[i] < rt[j]
	})
	return fmt.Sprintf("Route(%s)", strings.Join(rt, " && "))
}

func (r routePart) String() string {
	var (
		kind, val string
	)

	if r.regex {
		kind = fmt.Sprintf("%sRegexp", r.kind)
	} else {
		kind = r.kind
	}

	if r.header != "" {
		val = fmt.Sprintf("`%s`, `%s`", r.header, r.value)
	} else {
		val = fmt.Sprintf("`%s`", r.value)
	}

	return fmt.Sprintf("%s(%s)", kind, val)
}

func (s Server) String() string {
	return fmt.Sprintf(`Server(url="%v")`, s.URL())
}
