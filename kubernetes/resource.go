package kubernetes

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/endpoints"
	"k8s.io/kubernetes/pkg/apis/extensions"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/url"
)

func NewResource(id, namespace string, port api.ServicePort, meta api.ObjectMeta) *Resource {
	annotations := make(map[string]string)
	for key, value := range meta.Annotations {
		if strings.HasPrefix(key, Keyspace) {
			bits := strings.SplitN(path.Base(key), ".", 2)
			if len(bits) == 2 && namespace == "" {
				continue
			}
			switch len(bits) {
			case 2:
				if bits[0] == namespace {
					annotations[bits[1]] = value
				}
			case 1:
				if _, ok := annotations[bits[0]]; !ok {
					annotations[bits[0]] = value
				}
			}
		}
	}

	websocket := false
	if val, ok := annotations["websocket"]; ok {
		if b, er := strconv.ParseBool(val); er == nil {
			websocket = b
		}
	}

	return &Resource{
		id:          id,
		Route:       NewRoute(id, annotations),
		port:        port,
		annotations: annotations,
		uid:         string(meta.UID),
		servers:     make([]*Server, 0, 1),
		websocket:   websocket,
	}
}

func NewRoute(id string, annotations map[string]string) *Route {
	var (
		rt = &Route{parts: make([]*routePart, 0, 1)}
	)

	for key, val := range annotations {
		switch key {
		case HeadersKey:
			vals := strings.Fields(strings.Replace(val, ";", "", -1))
			for _, v := range vals {
				bits := strings.SplitN(v, "=", 2)
				if len(bits) < 2 {
					continue
				}
				if er := rt.AddHeader(bits[0], bits[1]); er != nil {
					logger.Warnf("[%v] Failed to add header(%q) matcher: %v", id, bits[0], er)
				}
			}
		case MethodsKey:
			vals := strings.Fields(strings.Replace(val, ";", "", -1))
			for _, v := range vals {
				if er := rt.AddMethod(strings.ToUpper(v)); er != nil {
					logger.Warnf("[%v] Failed to add method matcher: %v", id, er)
				}
			}
		case HostKey:
			if er := rt.AddHost(val); er != nil {
				logger.Warnf("[%v] Failed to add host matcher: %v", id, er)
			}
		case PathKey:
			if er := rt.AddPath(val); er != nil {
				logger.Warnf("[%v] Failed to add patch matcher: %v", id, er)
			}
		case PrefixKey:
			if er := rt.AddPrefix(val); er != nil {
				logger.Warnf("[%v] Failed to app prefix matcher: %v", id, er)
			}
		}
	}

	return rt
}

func GenResources(store *Cache, obj interface{}) (ResourceList, error) {
	var (
		list ResourceList = make([]*Resource, 0, 1)
	)

	switch t := obj.(type) {
	default:
		return list, errors.New("Unsupported type")
	case *extensions.Ingress:
		list = resourcesFromIngress(store, t)
	case *api.Service:
		list = resourcesFromService(store, t)
	case *api.Endpoints:
		list = resourcesFromEndpoints(store, t)
	}
	logger.Debugf(list.String())
	return list, nil
}

func resourcesFromIngress(store *Cache, in *extensions.Ingress) ResourceList {
	var (
		list ResourceList = make([]*Resource, 0, 1)
	)

	namespace := in.GetNamespace()
	if in.Spec.Backend != nil {
		name := in.Spec.Backend.ServiceName
		svc, er := store.GetService(namespace, name)
		if er != nil {
			logger.Warnf(er.Error())
			goto Rules
		}

		port, ok := GetServicePort(svc, in.Spec.Backend.ServicePort)
		if !ok {
			goto Rules
		}

		id := GenResourceID(namespace, name, intstrFromPort(port.Name, port.Port))
		r := NewResource(id, port.Name, port, svc.ObjectMeta)
		en, _ := store.GetEndpoints(namespace, name)
		AddServers(r, svc, en, port)

		list = append(list, r)
	}

Rules:
	for _, rule := range in.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			name := path.Backend.ServiceName
			svc, er := store.GetService(namespace, name)
			if er != nil {
				continue
			}
			port, ok := GetServicePort(svc, path.Backend.ServicePort)
			if !ok {
				continue
			}

			id := GenResourceID(namespace, name, intstrFromPort(port.Name, port.Port))
			r := NewResource(id, port.Name, port, svc.ObjectMeta)
			en, _ := store.GetEndpoints(namespace, name)
			AddServers(r, svc, en, port)

			if rule.Host != "" {
				r.Route.delete(HostPart)
				r.Route.AddHost(rule.Host)
			}
			if path.Path != "" {
				r.Route.delete(PathPart)
				r.Route.AddPath(path.Path)
			}
			list = append(list, r)
		}
	}
	Sort(list, nil)
	return list
}

func resourcesFromService(store *Cache, svc *api.Service) ResourceList {
	var (
		list ResourceList = make([]*Resource, 0, 1)
		s    Service      = Service(*svc)

		namespace = svc.GetNamespace()
		name      = svc.GetName()
	)

	en, er := store.GetEndpoints(namespace, name)
	if er != nil {
		logger.Warnf("No Endpoints for %v", s)
	}

	for _, port := range svc.Spec.Ports {
		id := GenResourceID(namespace, name, intstrFromPort(port.Name, port.Port))
		r := NewResource(id, port.Name, port, svc.ObjectMeta)
		AddServers(r, svc, en, port)

		list = append(list, r)
	}
	Sort(list, nil)
	return list
}

func resourcesFromEndpoints(store *Cache, en *api.Endpoints) ResourceList {
	var (
		list ResourceList = make([]*Resource, 0, 1)
		e    Endpoints    = Endpoints(*en)

		namespace = en.GetNamespace()
		name      = en.GetName()
	)

	svc, er := store.GetService(namespace, name)
	if er != nil {
		logger.Errorf("Unable to find Service for %v", e)
		return list
	}

	for _, port := range svc.Spec.Ports {
		id := GenResourceID(namespace, name, intstrFromPort(port.Name, port.Port))
		r := NewResource(id, port.Name, port, svc.ObjectMeta)
		AddServers(r, svc, en, port)

		list = append(list, r)
	}
	Sort(list, nil)
	return list
}

func AddServers(rsc *Resource, svc *api.Service, en *api.Endpoints, port api.ServicePort) {
	if en != nil {
		AddServersFromEndpoints(rsc, en, port)
	}
	if rsc.NoServers() {
		logger.Warnf("[%v] No servers added from Endpoints, falling back to Service", rsc.id)
		AddServersFromService(rsc, svc, port)
	}
}

func AddServersFromService(r *Resource, svc *api.Service, p api.ServicePort) {
	var (
		namespace = svc.GetNamespace()
		name      = svc.GetName()
		ips       = make([]string, 0, 1)
		s         = Service(*svc)
	)

	logger.Debugf("[%v] Adding Servers from %v", r.id, s)
	if HasServiceIP(svc) {
		ips = append(ips, svc.Spec.ClusterIP)
	} else if len(svc.Spec.ExternalIPs) > 0 {
		ips = svc.Spec.ExternalIPs
	}

	for _, ip := range ips {
		id := GenServerID(namespace, name, ip, p.Port)

		scheme := HTTP
		if sc, ok := r.GetAnnotation("scheme"); ok && validScheme.MatchString(sc) {
			scheme = sc
		}

		r.AddServer(id, scheme, ip, p.Port)
	}
}

func AddServersFromEndpoints(r *Resource, en *api.Endpoints, p api.ServicePort) {
	// Quick path, no subsets
	if len(en.Subsets) < 1 {
		return
	}

	var (
		namespace = en.GetNamespace()
		name      = en.GetName()
		subs      = endpoints.RepackSubsets(en.Subsets)
		end       = Endpoints(*en)
	)

	logger.Debugf("[%v] Adding Servers from %v", r.id, end)
	for _, sub := range subs {
		logger.Debugf("[%v] Subset(Ports=%+v, Addrs=%+v)", r.id, sub.Ports, sub.Addresses)
		for _, port := range sub.Ports {
			if !matchPort(p, port) {
				continue
			}

			logger.Debugf(`[%v] Found Port("%d") in %v`, r.id, p.Port, end)
			for _, addr := range sub.Addresses {
				id := GenServerID(namespace, name, addr.IP, port.Port)
				// scheme := string(port.Protocol)
				scheme := HTTP
				if sc, ok := r.GetAnnotation("scheme"); ok {
					scheme = sc
				}
				r.AddServer(id, scheme, addr.IP, port.Port)
			}
		}
	}
}

func (r *Resource) AddServer(id, scheme, ip string, port int) {
	if r.servers == nil {
		r.servers = make([]*Server, 0, 1)
	}

	server := &Server{
		id:        id,
		scheme:    scheme,
		ip:        ip,
		port:      port,
		websocket: (scheme == "ws" || scheme == "wss"),
	}
	logger.Debugf("[%v] Adding %v", r.id, server)
	r.servers = append(r.servers, server)
}

func (r *Resource) ID() string          { return r.id }
func (r *Resource) UID() string         { return r.uid }
func (r *Resource) Servers() ServerList { return r.servers }
func (r *Resource) IsWebsocket() bool   { return r.websocket }

func (r *Resource) GetAnnotations(expr string) (map[string]string, error) {
	var matches = make(map[string]string)
	rgx, er := regexp.Compile(expr)
	if er != nil {
		return matches, er
	}
	for key, value := range r.annotations {
		if rgx.MatchString(key) {
			matches[key] = value
		}
	}
	return matches, nil
}

func (r *Resource) GetAnnotation(key string) (val string, ok bool) {
	logger.Debugf("[%v] Looking up annotation key=%q", r.id, key)
	val, ok = r.annotations[key]
	return
}

func (r *Resource) NoServers() bool {
	return r.servers == nil || len(r.servers) < 1
}

func (s *Server) URL() *url.URL {
	ur, er := url.Parse(fmt.Sprintf("%s://%s:%d", s.scheme, s.ip, s.port))
	if er != nil {
		logger.Warnf("Failed to create URL for Server(%s): %v", s.id, er)
	}
	return ur
}

func (s *Server) ID() string        { return s.id }
func (s *Server) IsWebsocket() bool { return s.websocket }

func (r *Route) Empty() bool {
	return len(r.parts) < 1
}

func (r *Route) AddHost(host string) error {
	part := &routePart{kind: HostPart, value: host}
	return r.add(part, host)
}

func (r *Route) AddPath(path string) error {
	part := &routePart{kind: PathPart, value: path}
	return r.add(part, path)
}

func (r *Route) AddHeader(header, value string) error {
	part := &routePart{kind: HeaderPart, header: header, value: value}
	return r.add(part, value)
}

func (r *Route) AddMethod(method string) error {
	part := &routePart{kind: MethodPart, value: method}
	return r.add(part, method)
}

func (r *Route) AddPrefix(pre string) error {
	part := &routePart{kind: PrefixPart, value: pre}
	return r.add(part, pre)
}

func (r *Route) add(part *routePart, val string) error {
	if isRegexp(val) {
		expr := strings.Trim(val, "|")
		rg, er := regexp.Compile(expr)
		if er != nil {
			return er
		}
		part.value = rg.String()
		part.regex = true
	}
	r.parts = append(r.parts, part)
	return nil
}

func (r *Route) delete(kind string) {
	cp := make([]*routePart, 0, 1)
	for i := 0; i < len(r.parts); i++ {
		if r.parts[i].kind != kind {
			cp = append(cp, r.parts[i])
		}
	}

	r.parts = cp
	return
}

func (r *Route) Parts() []*routePart {
	p := make([]*routePart, len(r.parts))
	copy(p, r.parts)
	return p
}

func (r *routePart) Type() string   { return r.kind }
func (r *routePart) Value() string  { return r.value }
func (r *routePart) Header() string { return r.header }
func (r *routePart) IsRegex() bool  { return r.regex }

func (r ResourceList) Map() map[string]*Resource {
	m := make(map[string]*Resource, len(r))
	for i := range r {
		m[r[i].id] = r[i]
	}
	return m
}
