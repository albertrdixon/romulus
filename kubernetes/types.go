package kubernetes

import (
	"fmt"
	"strings"

	"github.com/albertrdixon/gearbox/url"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const (
	ServiceKind   = "Service"
	EndpointsKind = "Endpoints"
	StatusKind    = "Status"
)

type Object interface {
	runtime.Object
}

type Service struct {
	api.Service
}

type Endpoints struct {
	api.Endpoints
}

type EndpointSubset struct {
	api.EndpointSubset
}

type EndpointSubsets []api.EndpointSubset

type Metadata struct {
	api.ObjectMeta
	Kind string
}

type Addresses map[int]*url.URL

type ClientConfig struct {
	unversioned.Config
	InCluster, Testing bool
}

func (e Endpoints) String() string {
	return fmt.Sprintf(`Endpoints(Name=%q, Namespace=%q)`, e.ObjectMeta.Name, e.ObjectMeta.Namespace)
}

func (s Service) String() string {
	return fmt.Sprintf(`Service(Name=%q, Namespace=%q)`, s.ObjectMeta.Name, s.ObjectMeta.Namespace)
}

func (e EndpointSubset) String() string {
	ports := make([]string, 0, len(e.Ports))
	addrs := make([]string, 0, len(e.Addresses))

	for _, p := range e.Ports {
		ports = append(ports, fmt.Sprintf("%s:%d", p.Name, p.Port))
	}
	for _, a := range e.Addresses {
		addrs = append(addrs, a.IP)
	}
	return fmt.Sprintf("{ips=[%s], ports=[%s]}",
		strings.Join(addrs, ", "), strings.Join(ports, ", "))
}

func (eps EndpointSubsets) String() string {
	sl := []string{}
	for _, s := range eps {
		sl = append(sl, EndpointSubset{s}.String())
	}
	return fmt.Sprintf("Subsets(%s)", strings.Join(sl, ", "))
}
