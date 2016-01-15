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

	rt := NewRoute()
	for key, val := range annotations {
		switch key {
		case "headers":
			vals := strings.Fields(strings.Replace(val, ";", "", -1))
			for _, v := range vals {
				bits := strings.SplitN(v, "=", 2)
				if len(bits) < 2 {
					continue
				}
				rt.AddHeader(bits[0], bits[1])
			}
		case "host":
			rt.AddHost(val)
		case "host_regex":
			if er := rt.AddHostRegex(val); er != nil {
				logger.Warnf("[%v] Failed to parse host regex: %v", id, er)
			}
		case "hostRegex":
			if er := rt.AddHostRegex(val); er != nil {
				logger.Warnf("[%v] Failed to parse host regex: %v", id, er)
			}
		case "path":
			rt.AddPath(val)
		case "path_regex":
			if er := rt.AddPathRegex(val); er != nil {
				logger.Warnf("[%v] Failed to parse path regex: %v", id, er)
			}
		case "pathRegex":
			if er := rt.AddPathRegex(val); er != nil {
				logger.Warnf("[%v] Failed to parse path regex: %v", id, er)
			}
		case "method":
			rt.AddMethod(val)
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
		Route:       rt,
		port:        port,
		annotations: annotations,
		uid:         string(meta.UID),
		servers:     make([]*Server, 0, 1),
		websocket:   websocket,
	}
}

func NewRoute() *Route {
	return &Route{
		parts:  make(map[string]string),
		header: make(map[string]string),
		regex:  make(map[string]*regexp.Regexp),
	}
}

func GenResources(store Cache, obj interface{}) (ResourceList, error) {
	var (
		list ResourceList = make([]*Resource, 0, 1)
	)

	switch t := obj.(type) {
	default:
		return list, errors.New("Unsupported type")
	case *extensions.Ingress:
		list = ResourcesFromIngress(store, t)
	case *api.Service:
		list = ResourcesFromService(store, t)
	case *api.Endpoints:
		list = ResourcesFromEndpoints(store, t)
	}
	logger.Debugf(list.String())
	return list, nil
}

func ResourcesFromIngress(store Cache, in *extensions.Ingress) ResourceList {
	var (
		list ResourceList = make([]*Resource, 0, 1)
	)

	namespace := in.GetNamespace()
	if in.Spec.Backend != nil {
		name := in.Spec.Backend.ServiceName
		svc, er := store.GetService(namespace, name)
		if er != nil {
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

			r.Route.AddHost(rule.Host)
			r.Route.AddPath(path.Path)
			list = append(list, r)
		}
	}
	Sort(list, nil)
	return list
}

func ResourcesFromService(store Cache, svc *api.Service) ResourceList {
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

func ResourcesFromEndpoints(store Cache, en *api.Endpoints) ResourceList {
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
	return len(r.parts) < 1 && len(r.header) < 1
}

func (r *Route) AddHost(host string)            { r.parts["host"] = host }
func (r *Route) AddPath(path string)            { r.parts["path"] = path }
func (r *Route) AddMethod(method string)        { r.parts["method"] = strings.ToUpper(method) }
func (r *Route) AddHeader(header, value string) { r.header[header] = value }

func (r *Route) GetParts() map[string]string {
	var dst = make(map[string]string, len(r.parts))
	for k, v := range r.parts {
		dst[k] = v
	}
	return dst
}

func (r *Route) GetHeader() map[string]string {
	var dst = make(map[string]string, len(r.header))
	for k, v := range r.header {
		dst[k] = v
	}
	return dst
}

func (r *Route) GetRegex() map[string]string {
	var dst = make(map[string]string, len(r.regex))
	for k, v := range r.regex {
		dst[k] = v.String()
	}
	return dst
}

func (r *Route) AddHostRegex(expr string) error {
	rg, er := regexp.Compile(expr)
	if er != nil {
		return er
	}
	r.regex["host"] = rg
	return nil
}

func (r *Route) AddPathRegex(expr string) error {
	rg, er := regexp.Compile(expr)
	if er != nil {
		return er
	}
	r.regex["path"] = rg
	return nil
}

func (r ResourceList) Map() map[string]*Resource {
	m := make(map[string]*Resource, len(r))
	for i := range r {
		m[r[i].id] = r[i]
	}
	return m
}
