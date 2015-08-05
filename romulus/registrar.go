package romulus

import (
	"fmt"
	"net/url"
	"strings"

	"code.google.com/p/go-uuid/uuid"

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
	rk string
	v  string
	s  ServiceSelector
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
		s:  c.Selector,
		vk: formatEtcdNamespace(c.VulcanEtcdNamespace),
		rk: formatEtcdNamespace(c.RomulusEtcdNamespace),
	}, nil
}

func (c *Registrar) initEndpoints() (watch.Interface, error) {
	kf := "%s/backends"
	ids, err := c.e.Keys(fmt.Sprintf(kf, c.rk))
	if err != nil {
		return nil, NewErr(err, "etcd error")
	}
	for _, id := range ids {
		c.k.Endpoints(api.NamespaceAll)
	}

	return c.k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (c *Registrar) serviceEventsChannel() (watch.Interface, error) {
	return c.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (c *Registrar) getService(name, ns string) (*api.Service, error) {
	s, e := c.k.Services(ns).Get(name)
	if e != nil || s == nil {
		return nil, Error{fmt.Sprintf("Unable to get service %q", name), e}
	}
	return s, nil
}

func (c *Registrar) getEndpoint(name, ns string) (*api.Endpoints, error) {
	en, e := c.k.Endpoints(ns).Get(name)
	if e != nil || en == nil {
		return nil, Error{fmt.Sprintf("Unable to get endpoint %q", name), e}
	}
	return en, nil
}

func (c *Registrar) pruneServers(bid uuid.UUID, sm ServerMap) error {
	k := fmt.Sprintf(srvrDirFmt, bid.String())
	ips, e := c.e.Keys(k)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return NewErr(e, "etcd error")
	}

	logf(fi{"servers": ips, "bcknd-id": bid.String()}).Debug("Gathered servers from etcd")
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

func (reg *Registrar) handleDelete(r runtime.Object) error {
	switch o := r.(type) {
	case *api.Endpoints:
		return deregister(reg, o.ObjectMeta, false)
	case *api.Service:
		return deregister(reg, o.ObjectMeta, true)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
}

func (reg *Registrar) handleUpdate(r runtime.Object) error {
	switch o := r.(type) {
	case *api.Service:
		return nil
	case *api.Endpoints:
		return register(reg, o)
	default:
		return NewErr(nil, "Unsupported api object: %v", r)
	}
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
		return r.handleDelete(e.Object)
	case watch.Added, watch.Modified:
		return r.handleUpdate(e.Object)
	}
}

func expandEndpoints(bid uuid.UUID, e *api.Endpoints) ServerMap {
	sm := ServerMap{}
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			if port.Protocol != api.ProtocolTCP {
				logf(fi{"bcknd-id": bid.String()}).Warnf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			// TODO: Do we want to force ports to have a name?
			// if port.Name != "vulcan" {
			// 	log().Debugf("Not registering port %d", port.Port)
			// 	continue
			// }

			for _, ip := range es.Addresses {
				ur := fmt.Sprintf("http://%s:%d", ip.IP, port.Port)
				u, err := url.Parse(ur)
				if err != nil {
					logf(fi{"bcknd-id": bid.String()}).Warnf("Bad URL: %s", ur)
					continue
				}
				uu := (*URL)(u)
				sm[uu.GetHost()] = Server{
					Backend: bid,
					URL:     uu,
				}
			}
		}
	}
	return sm
}

func getUUID(o api.ObjectMeta) uuid.UUID {
	return uuid.Parse((string)(o.UID))
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
