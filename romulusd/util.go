package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	uapi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
)

type metadata struct {
	name, ns, kind, version string
	labels, annotations     map[string]string
	uid                     types.UID
}

func getMeta(obj runtime.Object) (*metadata, error) {
	a, e := meta.Accessor(obj)
	if e != nil {
		return nil, e
	}
	return &metadata{
		name:        a.Name(),
		ns:          a.Namespace(),
		kind:        getKind(a, obj),
		version:     a.ResourceVersion(),
		uid:         a.UID(),
		labels:      a.Labels(),
		annotations: a.Annotations(),
	}, nil
}

func getKind(m meta.Interface, r runtime.Object) string {
	k := m.Kind()
	if k != "" {
		return k
	}
	switch r.(type) {
	default:
		return "Unknown"
	case *api.Service:
		return serviceType
	case *api.Endpoints:
		return endpointsType
	case *uapi.Status:
		return "Status"
	}
}

func md5Hash(ss ...interface{}) string {
	if len(ss) < 1 {
		return ""
	}

	h := md5.New()
	for i := range ss {
		io.WriteString(h, fmt.Sprintf("%v", ss[i]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func kubeClient() (unversioned.Interface, error) {
	if test {
		return tKubeClient, nil
	}

	cfg := &unversioned.Config{
		Host:     (*kubeAddr).String(),
		Username: *kubeUser,
		Password: *kubePass,
		Insecure: true,
	}
	if useTLS() {
		cfg.Insecure = false
		cfg.CertFile = *kubeCert
		cfg.KeyFile = *kubeKey
		cfg.CAFile = *kubeCA
	}
	if *kubeUseClust {
		if cc, er := unversioned.InClusterConfig(); er == nil {
			cfg = cc
		}
	}
	return unversioned.New(cfg)
}

func useTLS() bool {
	return *kubeCert != "" && (*kubeKey != "" || *kubeCA != "")
}

type endpoints struct{ *api.Endpoints }

func (e endpoints) String() string {
	return fmt.Sprintf("Endpoints '%s-%s'", e.ObjectMeta.Name, e.ObjectMeta.Namespace)
}

type service struct{ *api.Service }

func (s service) String() string {
	return fmt.Sprintf("Service '%s-%s'", s.ObjectMeta.Name, s.ObjectMeta.Namespace)
}

type epSubsets []api.EndpointSubset
type epSubset api.EndpointSubset

func (s epSubset) String() string {
	ports := make([]string, 0, len(s.Ports))
	addrs := make([]string, 0, len(s.Addresses))

	for _, p := range s.Ports {
		ports = append(ports, fmt.Sprintf("%s:%d", p.Name, p.Port))
	}
	for _, a := range s.Addresses {
		addrs = append(addrs, a.IP)
	}
	return fmt.Sprintf("{ips=[%s], ports=[%s]}",
		strings.Join(addrs, ", "), strings.Join(ports, ", "))
}

func (ss epSubsets) String() string {
	sl := []string{}
	for _, s := range ss {
		sl = append(sl, epSubset(s).String())
	}
	return fmt.Sprintf("[%s]", strings.Join(sl, ", "))
}

func annotationf(p, n string) string { return fmt.Sprintf("romulus/%s%s", p, n) }
func labelf(l string, s ...string) string {
	la := strings.Join(append([]string{l}, s...), ".")
	if !strings.HasPrefix(la, "romulus/") {
		return strings.Join([]string{"romulus", la}, "/")
	}
	return l
}

func backendf(id string) string     { return fmt.Sprintf("backends/%s/backend", id) }
func frontendf(id string) string    { return fmt.Sprintf("frontends/%s/frontend", id) }
func backendDirf(id string) string  { return fmt.Sprintf("backends/%s", id) }
func frontendDirf(id string) string { return fmt.Sprintf("frontends/%s", id) }
func serverf(b, id string) string   { return fmt.Sprintf("backends/%s/servers/%s", b, id) }
func serverDirf(id string) string   { return fmt.Sprintf("backends/%s/servers", id) }
