package kubernetes

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	uapi "k8s.io/kubernetes/pkg/api/unversioned"
)

func GetMetadata(obj Object) (*Metadata, error) {
	o, er := api.ObjectMetaFor(obj)
	if er != nil {
		return nil, er
	}
	md := &Metadata{*o, "Unknown"}
	a, er := meta.Accessor(obj)
	if er != nil {
		return md, er
	}
	md.Kind = getKind(a, obj)
	return md, nil
}

func getKind(m meta.Interface, r Object) string {
	k := m.Kind()
	if k != "" {
		return k
	}
	switch r.(type) {
	default:
		return "Unknown"
	case *api.Service:
		return ServiceKind
	case *api.Endpoints:
		return EndpointsKind
	case *uapi.Status:
		return StatusKind
	}
}
