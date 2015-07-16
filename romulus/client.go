package romulus

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mgutz/logxi/v1"
)

type EtcdPeerList []string
type KubeClientConfig client.Config
type ResourceVersion string
type ServiceSelector map[string]string
type Endpoints []string

func (e Endpoints) isEmpty() bool { return len(([]string)(e)) == 0 }

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
	l log.Logger
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
		l: log.New("client"),
	}, nil
}

func (c *Client) setWatch() (watch.Interface, error) {
	return c.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), "")
}

func doEvent(c *Client, e watch.Event) error {
	c.l.Debug("Got an event", "event", e.Type)
	switch e.Type {
	default:
		return Error{fmt.Sprintf("Unrecognized event: %s", e.Type), nil}
	case watch.Added, watch.Modified:
		return register(c, e.Object)
	case watch.Deleted:
		return deregister(c, e.Object)
	case watch.Error:
		if a, ok := e.Object.(*api.Status); ok {
			e := fmt.Errorf("[%d] %v", a.Code, a.Reason)
			return Error{fmt.Sprintf("Kubernetes API failure: %s", a.Message), e}
		}
		return Error{"Unknown kubernetes api error", nil}
	}
}

func getBackendID(s *api.Service) string {
	r := fmt.Sprintf("%s.%s", s.Name, s.Namespace)
	if an, ok := s.Annotations[bckndIDAnnotation]; ok {
		r = an
	}
	return r
}

func getBackendConfig(s *api.Service) string {
	if b, ok := s.Annotations["vulcanBackendConfig"]; ok {
		return b
	}
	return ""
}

func getEndpoints(s *api.Service) Endpoints {
	en := []string{}
	ip := s.Spec.ClusterIP
	for _, sp := range s.Spec.Ports {
		if sp.Protocol == api.ProtocolTCP {
			en = append(en, fmt.Sprintf("http://%s:%d", ip, sp.Port))
		}
	}
	return Endpoints(en)
}

func registerable(s *api.Service, sl ServiceSelector) bool {
	for k, v := range sl {
		if sv, ok := s.Labels[k]; !ok || sv != v {
			return false
		}
	}
	return s.Spec.ClusterIP != "None" && s.Spec.ClusterIP != ""
}
