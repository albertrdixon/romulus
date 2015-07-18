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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
)

type EtcdPeerList []string
type KubeClientConfig client.Config
type ResourceVersion string
type ServiceSelector map[string]string

type Config struct {
	PeerList   EtcdPeerList
	KubeConfig KubeClientConfig
	Version    ResourceVersion
	Selector   ServiceSelector
}

func (c *Config) kc() client.Config { return (client.Config)(c.KubeConfig) }
func (c *Config) ps() []string      { return ([]string)(c.PeerList) }

type Client struct {
	k *client.Client
	e *etcd.Client
	v string
	s ServiceSelector
}

func NewClient(c *Config) (*Client, error) {
	cf := c.kc()
	cl, err := client.New(&cf)
	if err != nil {
		return nil, err
	}
	return &Client{
		e: etcd.NewClient(c.ps()),
		k: cl,
		v: (string)(c.Version),
		s: c.Selector,
	}, nil
}

func (c *Client) endpointsEventChannel() (watch.Interface, error) {
	return c.k.Endpoints(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (c *Client) serviceEventsChannel() (watch.Interface, error) {
	return c.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func (c *Client) getService(name, ns string) (*api.Service, error) {
	s, e := c.k.Services(ns).Get(name)
	if e != nil || s == nil {
		return nil, Error{fmt.Sprintf("Unable to get service %q", name), e}
	}
	return s, nil
}

func (c *Client) getEndpoint(name, ns string) (*api.Endpoints, error) {
	en, e := c.k.Endpoints(ns).Get(name)
	if e != nil || en == nil {
		return nil, Error{fmt.Sprintf("Unable to get endpoint %q", name), e}
	}
	return en, nil
}

func (c *Client) pruneServers(bid uuid.UUID, sm ServerMap) error {
	k := fmt.Sprintf(srvrDirFmt, bid.String())
	r, e := c.e.Get(k, true, false)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return Error{"etcd error", e}
	}

	ips := []string{}
	for _, n := range r.Node.Nodes {
		ips = append(ips, strings.TrimLeft(strings.TrimPrefix(n.Key, k), "/"))
	}
	log().Debugf("Found %v ips in etcd", ips)

	for _, ip := range ips {
		if _, ok := sm[ip]; !ok {
			log().Debugf("Removing %s from etcd", ip)
			key := fmt.Sprintf("%s/%s", k, ip)
			if _, e := c.e.Delete(key, true); e != nil {
				return Error{"etcd error", e}
			}
		}
	}
	return nil
}

func doEndpointsEvent(c *Client, e watch.Event) error {
	logf(F{"event": e.Type}).Debug("Got an Endpoints event")
	switch e.Type {
	default:
		return nil
	case watch.Deleted:
		en, ok := e.Object.(*api.Endpoints)
		if !ok {
			return fmt.Errorf("Unrecognized api object: %v", e.Object)
		}
		return deregister(c, en.ObjectMeta, false)
	case watch.Added, watch.Modified:
		en, ok := e.Object.(*api.Endpoints)
		if !ok {
			return fmt.Errorf("Unrecognized api object: %v", e.Object)
		}
		return register(c, en)
	case watch.Error:
		if a, ok := e.Object.(*api.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return Error{fmt.Sprintf("Kubernetes API failure: %s", a.Message), e}
		}
		return Error{"Unknown kubernetes api error", nil}
	}
}

func doServiceEvent(c *Client, e watch.Event) error {
	if e.Type == watch.Added || e.Type == watch.Modified {
		return nil
	}
	logf(F{"event": e.Type}).Debug("Got a Service event")
	switch e.Type {
	default:
		return nil
	case watch.Deleted:
		s, ok := e.Object.(*api.Service)
		if !ok {
			return fmt.Errorf("Unrecognized api object: %v", e.Object)
		}
		return deregister(c, s.ObjectMeta, true)
	case watch.Error:
		if a, ok := e.Object.(*api.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return Error{fmt.Sprintf("Kubernetes API failure: %s", a.Message), e}
		}
		return Error{"Unknown kubernetes api error", nil}
	}
}

func expandEndpoints(bid uuid.UUID, e *api.Endpoints) ServerMap {
	sm := ServerMap{}
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			if port.Protocol != api.ProtocolTCP {
				log().Debugf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			// TODO: Do we want to force ports to have a name?
			// if port.Name != "vulcan" {
			// 	log().Debugf("Not registering port %d", port.Port)
			// 	continue
			// }

			for _, ip := range es.Addresses {
				u, err := url.Parse(fmt.Sprintf("http://%s:%d", ip.IP, port.Port))
				if err != nil {
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
	for k, v := range sl {
		if sv, ok := s.Labels[k]; !ok || sv != v {
			return false
		}
	}
	return api.IsServiceIPSet(s)
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*etcd.EtcdError)
	return ok && e.ErrorCode == etcdErr.EcodeKeyNotFound
}
