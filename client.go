package main

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/coreos/go-etcd/etcd"
)

type EtcdPeerList []string
type KubeClientConfig client.Config
type ResourceVersion string
type Endpoints []string

func (e Endpoints) isEmpty() bool { return len(([]string)(e)) == 0 }

type Config struct {
	p EtcdPeerList
	k KubeClientConfig
	v ResourceVersion
}

func (c *Config) kc() client.Config { return (client.Config)(c.k) }
func (c *Config) ps() []string      { return ([]string)(c.p) }

type Client struct {
	k *client.Client
	e *etcd.Client
	v string
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
		v: (string)(c.v),
	}, nil
}

func StartWatch(c *Client, s StopChan) error {
	w, e := c.k.Services(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), c.v)
	if e != nil {
		return e
	}
	go func() {
		for {
			select {
			case e := <-w.ResultChan():
				DoEvent(c, e)
			case <-s:
				w.Stop()
				return
			}
		}
	}()
	return nil
}

func DoEvent(c *Client, e watch.Event) {

	switch e.Type {
	case watch.Added, watch.Modified:
		s := e.Object.(*api.Service)
		if s.Spec.ClusterIP != "None" && s.Spec.ClusterIP != "" {
			RegisterWithVulcan(c, getServiceName(s), expandEndpoints(s))
		}
	case watch.Deleted:
		s := e.Object.(*api.Service)
		DeregisterWithVulcan(c, getServiceName(s))
	case watch.Error:
	}
}

func getServiceName(s *api.Service) string {
	r := fmt.Sprintf("%s.%s", s.Name, s.Namespace)
	if an, ok := s.Annotations["domain"]; ok {
		r = an
	}
	return r
}

func expandEndpoints(s *api.Service) Endpoints {
	en := []string{}
	ip := s.Spec.ClusterIP
	for _, sp := range s.Spec.Ports {
		if sp.Protocol == api.ProtocolTCP {
			en = append(en, fmt.Sprintf("http://%s:%d", ip, sp.Port))
		}
	}
	return Endpoints(en)
}
