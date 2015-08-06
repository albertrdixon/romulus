package romulus

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

// EtcdPeerList is just a slice of etcd peers
type EtcdPeerList []string

// KubeClientConfig is an alias for kubernetes/pkg/client.Config
type KubeClientConfig client.Config

// ServiceSelector is a map of labels for selecting services
type ServiceSelector map[string]string

func (s ServiceSelector) fixNamespace() ServiceSelector {
	ss := make(map[string]string, len(s))
	for k := range s {
		key := k
		if !strings.HasPrefix(k, "romulus/") {
			key = fmt.Sprintf("romulus/%s", key)
		}
		ss[key] = s[k]
	}
	return ServiceSelector(ss)
}

func formatEtcdNamespace(v string) string {
	return fmt.Sprintf("/%s", strings.Trim(v, "/"))
}

// Config is used to configure the Registrar
type Config struct {
	PeerList             EtcdPeerList
	KubeConfig           KubeClientConfig
	APIVersion           string
	Selector             ServiceSelector
	VulcanEtcdNamespace  string
	RomulusEtcdNamespace string
}

func (c *Config) kc() client.Config { return (client.Config)(c.KubeConfig) }
func (c *Config) ps() []string      { return ([]string)(c.PeerList) }

func (sl ServiceSelector) String() string {
	s := []string{}
	for k, v := range sl {
		s = append(s, strings.Join([]string{k, v}, "="))
	}
	return strings.Join(s, ", ")
}

// Registrar holds the kubernetes/pkg/client.Client and etcd.Client
type Registrar struct {
	k  *client.Client
	e  EtcdClient
	vk string
	v  string
	s  ServiceSelector
}

func (r *Registrar) serviceWatch() (watch.Interface, error) {
	return r.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (r *Registrar) endpointsWatch() (watch.Interface, error) {
	return r.k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (r *Registrar) getEndpoint(name, ns string) (*api.Endpoints, error) {
	if ns == "" {
		ns = api.NamespaceAll
	}
	return r.k.Endpoints(ns).Get(name)
}

func (r *Registrar) getService(name, ns string) (*api.Service, error) {
	if ns == "" {
		ns = api.NamespaceAll
	}
	return r.k.Services(ns).Get(name)
}

// NewRegistrar returns a ptr to a new Registrar from a Config
func NewRegistrar(c *Config) (*Registrar, error) {
	cf := c.kc()
	cl, err := client.New(&cf)
	if err != nil {
		return nil, err
	}
	return &Registrar{
		e:  NewEtcdClient(c.ps()),
		k:  cl,
		v:  c.APIVersion,
		s:  c.Selector.fixNamespace(),
		vk: formatEtcdNamespace(c.VulcanEtcdNamespace),
	}, nil
}

func (r *Registrar) initEndpoints() (watch.Interface, error) {
	kf := "%s/backends"
	ids, err := r.e.Keys(fmt.Sprintf(kf, r.vk))
	if err != nil {
		if isKeyNotFound(err) {
			return r.endpointsWatch()
		}
		return nil, NewErr(err, "etcd error")
	}

	log().Debugf("Found current backends: %v", ids)
	for _, id := range ids {
		name := strings.Split(id, ".")
		if len(name) < 2 {
			logf(fi{"bcknd-id": id}).Error("Invalid backend ID")
			continue
		}
		if _, err := r.getEndpoint(name[0], ""); err != nil && kubeIsNotFound(err) {
			logf(fi{"bcknd-id": id}).Warnf("Did not find backend on API server: %v", err)
			b := NewBackend(id)
			if err := r.e.Del(b.DirKey(r.vk)); err != nil {
				logf(fi{"bcknd-id": id}).Warn("etcd error")
			}
		}
	}

	return r.endpointsWatch()
}

func (r *Registrar) initServices() (watch.Interface, error) {
	kf := "%s/frontends"
	ids, err := r.e.Keys(fmt.Sprintf(kf, r.vk))
	if err != nil {
		if isKeyNotFound(err) {
			return r.serviceWatch()
		}
		return nil, NewErr(err, "etcd error")
	}

	log().Debugf("Found current frontends: %v", ids)
	for _, id := range ids {
		name := strings.Split(id, ".")
		if len(name) < 2 {
			logf(fi{"frntnd-id": id}).Error("Invalid frontend ID")
			continue
		}
		if _, err := r.getService(name[0], ""); err != nil {
			logf(fi{"frntnd-id": id}).Warnf("Did not find frontend on API server: %v", err)
			f := NewFrontend(id, "")
			if err := r.e.Del(f.DirKey(r.vk)); err != nil {
				logf(fi{"frntnd-id": id}).Warn("etcd error")
			}
		}
	}

	return r.serviceWatch()
}

func (c *Registrar) pruneServers(bid string, sm ServerMap) error {
	k := fmt.Sprintf(srvrDirFmt, bid)
	ips, e := c.e.Keys(k)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return NewErr(e, "etcd error")
	}

	logf(fi{"servers": ips, "bcknd-id": bid}).Debug("Gathered servers from etcd")
	for _, ip := range ips {
		if _, ok := sm[ip]; !ok {
			log().Debugf("Removing %s from etcd", ip)
			key := fmt.Sprintf("%s/%s", k, ip)
			if e := c.e.Del(key); e != nil {
				return Error{"etcd error", e}
			}
		}
	}
	return nil
}

func (reg *Registrar) delete(r runtime.Object) error {
	switch o := r.(type) {
	case *api.Endpoints:
		return deregisterEndpoints(reg, o)
	case *api.Service:
		return deregisterService(reg, o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}

func (reg *Registrar) update(r runtime.Object, s string) error {
	switch o := r.(type) {
	case *api.Service:
		if s == "mod" {
			return registerService(reg, o)
		}
		return nil
	case *api.Endpoints:
		return registerEndpoint(reg, o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}

func (r *Registrar) registerBackends(s *api.Service, e *api.Endpoints) (BackendList, error) {
	bnds := BackendList{}
	logf(fi{"service": e.Name, "namespace": e.Namespace}).Debug("Registering backend")
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			if port.Protocol != api.ProtocolTCP {
				logf(fi{"service": e.Name, "namespace": e.Namespace}).Warnf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			sm := ServerMap{}
			bid := getVulcanID(e.Name, e.Namespace, port.Name)
			bnd := NewBackend(bid)
			bnd.Type = "http"

			if st, ok := s.Annotations[bckndSettingsAnnotation]; ok {
				bnd.Settings = NewBackendSettings([]byte(st))
			}
			logf(fi{"bcknd-id": bnd.ID, "type": bnd.Type, "settings": bnd.Settings.String()}).Debug("Backend settings")

			val, err := bnd.Val()
			if err != nil {
				return bnds, NewErr(err, "Could not encode backend for %q", e.Name)
			}
			if err := r.e.Add(bnd.Key(r.vk), val); err != nil {
				return bnds, NewErr(err, "etcd error")
			}
			bnds[port.Port] = bnd

			for _, ip := range es.Addresses {
				ur := fmt.Sprintf("http://%s:%d", ip.IP, port.Port)
				u, err := url.Parse(ur)
				if err != nil {
					logf(fi{"service": e.Name, "namespace": e.Namespace, "bcknd-id": bnd.ID}).Warnf("Bad URL: %s", ur)
					continue
				}
				uu := (*URL)(u)
				sm[uu.GetHost()] = Server{
					Backend: bid,
					URL:     uu,
				}
			}
			if err := r.pruneServers(bid, sm); err != nil {
				return bnds, NewErr(err, "Unable to prune servers for backend %q", bid)
			}

			for _, srv := range sm {
				val, err := srv.Val()
				if err != nil {
					logf(fi{"service": e.Name, "namespace": e.Namespace,
						"server": srv.URL.String(), "error": err}).
						Warn("Unable to encode server")
					continue
				}
				if err := r.e.Add(srv.Key(r.vk), val); err != nil {
					return bnds, NewErr(err, "etcd error")
				}
			}
		}
	}
	return bnds, nil
}

func (r *Registrar) registerFrontends(s *api.Service, bnds BackendList) error {
	logf(fi{"service": s.Name, "namespace": s.Namespace}).Debug("Registering frontend")
	for _, port := range s.Spec.Ports {
		bnd, ok := bnds[port.Port]
		if !ok {
			logf(fi{"service": s.Name, "namespace": s.Namespace}).Warnf("No backend for service port %d", port.Port)
			continue
		}

		fid := getVulcanID(s.Name, s.Namespace, port.Name)
		fnd := NewFrontend(fid, bnd.ID)
		fnd.Type = "http"
		fnd.Route = buildRoute(s.Annotations)
		if st, ok := s.Annotations[frntndSettingsAnnotation]; ok {
			fnd.Settings = NewFrontendSettings([]byte(st))
		}
		logf(fi{"frntnd-id": fnd.ID, "type": fnd.Type, "route": fnd.Route,
			"settings": fnd.Settings.String()}).Debug("Frontend settings")

		val, err := fnd.Val()
		if err != nil {
			return NewErr(err, "Could not encode frontend for %q", s.Name)
		}
		if err := r.e.Add(fnd.Key(r.vk), val); err != nil {
			return NewErr(err, "etcd error")
		}
	}
	return nil
}

func do(r *Registrar, e watch.Event) error {
	logf(fi{"event": e.Type}).Debug("Got a Kubernetes API event")
	switch e.Type {
	default:
		log().Debugf("Unsupported event type %q", e.Type)
		return nil
	case watch.Error:
		if a, ok := e.Object.(*api.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return Error{fmt.Sprintf("Kubernetes API failure: %s", a.Message), e}
		}
		return Error{"Unknown kubernetes api error", nil}
	case watch.Deleted:
		return r.delete(e.Object)
	case watch.Added:
		return r.update(e.Object, "add")
	case watch.Modified:
		return r.update(e.Object, "mod")
	}
}

func getVulcanID(name, ns, port string) string {
	var id []string
	if port != "" {
		id = []string{name, port, ns}
	} else {
		id = []string{name, ns}
	}
	return strings.Join(id, ".")
}

func registerable(s *api.Service, sl ServiceSelector) bool {
	for k, v := range sl.fixNamespace() {
		if sv, ok := s.Labels[k]; !ok || sv != v {
			if sv, ok := s.Annotations[k]; !ok || sv != v {
				return false
			}
		}
	}
	return api.IsServiceIPSet(s)
}
