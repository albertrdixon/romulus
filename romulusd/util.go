package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"k8s.io/kubernetes/pkg/api/meta"
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
		kind:        a.Kind(),
		version:     a.ResourceVersion(),
		uid:         a.UID(),
		labels:      a.Labels(),
		annotations: a.Annotations(),
	}, nil
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
