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
	parts []*routePart
}

type routePart struct {
	kind, header, value string
	regex               bool
}

type Sorter struct {
	resources ResourceList
	sorter    func(a, b *Resource) bool
}

type Client struct {
	*unversioned.Client
}

type Cache struct {
	ingress, service, endpoints cache.Store
}

type Selector map[string]string

// type Cache map[string]cache.Store

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
