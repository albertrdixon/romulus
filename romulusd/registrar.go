package main

import (
	"fmt"
	"net/url"

	jURL "github.com/albertrdixon/gearbox/url"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/endpoints"
	uApi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

func pruneServers(bid string, sm ServerMap) error {
	k := serverDirf(bid)
	debugf("Looking up servers in etcd %q", k)
	srvs, e := etcd.Keys(k)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return NewErr(e, "etcd error")
	}

	debugf("Gathered known servers from kubernetes: %v", sm)
	debugf("Gathered known servers from etcd: %v", srvs)
	for _, id := range srvs {
		key := fmt.Sprintf("%s/%s", k, id)
		s, e := etcd.Val(key)
		if e != nil {
			warnf("Error getting server from etcd: %v", e)
			continue
		}

		srv := &Server{ID: id, Backend: bid}
		if e := decode(srv, []byte(s)); e != nil {
			errorf("Unable to unmarshall Server: %v", e)
			debugf("Data: %s", s)
			if e := etcd.Del(key); e != nil {
				errorf("Error removing server: %v", e)
			}
			continue
		}

		sTag := md5Hash(bid, srv.URL.String())[:serverTagLen]
		if nSrv, ok := sm[sTag]; ok {
			debugf("Server %q exists", srv.ID)
			nSrv.ID = srv.ID
		} else {
			infof("Removing Server %q", srv.ID)
			if e := etcd.Del(key); e != nil {
				errorf("Error removing server: %v", e)
				continue
			}
		}
	}
	return nil
}

func pruneBackends() error {
	ids, err := etcd.Keys("backends")
	if err != nil {
		if isKeyNotFound(err) {
			return nil
		}
		return NewErr(err, "etcd error")
	}

	debugf("Found current backends: %v", ids)
	for _, id := range ids {
		name, ns, e := parseVulcanID(id)
		if e != nil {
			errorf("Invalid ID: %s", id)
			key := backendDirf(id)
			if e := etcd.Del(key); e != nil {
				warnf("etcd error: %v", e)
			}
		} else if _, ok := getEndpoints(name, ns); !ok {
			warnf("Did not find '%s-%s' Endpoints on API server", name, ns)
			b := NewBackend(id)
			if err := etcd.Del(b.DirKey()); err != nil {
				warnf("etcd error")
			}
		}
	}
	return nil
}

func pruneFrontends() error {
	ids, err := etcd.Keys("frontends")
	if err != nil {
		if isKeyNotFound(err) {
			return nil
		}
		return NewErr(err, "etcd error")
	}

	debugf("Found current frontends: %v", ids)
	for _, id := range ids {
		name, ns, e := parseVulcanID(id)
		if e != nil {
			errorf("Invalid ID: %s", id)
			key := frontendDirf(id)
			if e := etcd.Del(key); e != nil {
				warnf("etcd error: %v", e)
			}
		} else if _, ok := getService(name, ns); !ok {
			warnf("Did not find '%s-%s' Service on API server", name, ns)
			f := NewFrontend(id, "")
			if err := etcd.Del(f.DirKey()); err != nil {
				warnf("etcd error: %v", err)
			}
		}
	}
	return nil
}

func registerBackends(s *api.Service, e *api.Endpoints) (*BackendList, error) {
	bnds := NewBackendList()
	infof("Registering backend '%s-%s'", e.Name, e.Namespace)
	// pruneBackends()
	subsets := endpoints.RepackSubsets(e.Subsets)
	for _, es := range subsets {
		for _, port := range es.Ports {
			if port.Protocol != api.ProtocolTCP {
				warnf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			sm := ServerMap{}
			bid := getVulcanID(e.Name, e.Namespace, port.Name)
			bnd := NewBackend(bid)

			if st, ok := s.Annotations[labelf("backendSettings")]; ok {
				bnd.Settings = NewBackendSettings([]byte(st))
			}
			debugf("Backend settings: %q", bnd)

			debugf("Gathering kubernetes endpoints: %v", es.Addresses)
			for _, ip := range es.Addresses {
				ur := fmt.Sprintf("http://%s:%d", ip.IP, port.Port)
				u, err := url.Parse(ur)
				if err != nil {
					warnf("Bad URL: %s", ur)
					continue
				}
				uu := (*jURL.URL)(u)
				sTag := md5Hash(bid, uu.String())[:serverTagLen]
				sm[sTag] = &Server{
					ID:      fmt.Sprintf("%s-%s", bid, sTag),
					Backend: bid,
					URL:     uu,
				}
			}
			if err := pruneServers(bid, sm); err != nil {
				warnf("Failed to remove servers for %q: %v", bnd.ID, err)
			}

			val, err := bnd.Val()
			if err != nil {
				return bnds, NewErr(err, "Could not encode backend for %q", e.Name)
			}
			eVal, _ := etcd.Val(bnd.Key())
			if val != eVal {
				debugf("Upserting backend %q", bnd)
				if err := etcd.Add(bnd.Key(), val); err != nil {
					return bnds, NewErr(err, "etcd error")
				}
			} else {
				debugf("No changes, not upserting Backend %q", bnd)
			}
			bnds.Add(port.Port, port.Name, bnd)

			for _, srv := range sm {
				val, err := srv.Val()
				if err != nil {
					warnf("Unable to encode server %q: %v", srv.ID, err)
					continue
				}
				eVal, _ := etcd.Val(srv.Key())
				if val != eVal {
					debugf("Upserting server %q", srv)
					if err := etcd.Add(srv.Key(), val); err != nil {
						return bnds, NewErr(err, "etcd error")
					}
				} else {
					debugf("No changes, not upserting Server %q", srv)
				}
			}
		}
	}
	return bnds, nil
}

func registerFrontends(s *api.Service, bnds *BackendList) error {
	infof("Registering frontend '%s-%s'", s.Name, s.Namespace)
	// pruneFrontends()
	debugf("Backend List: %+v", bnds)
	for _, port := range s.Spec.Ports {
		bnd, ok := bnds.Lookup(port.TargetPort.IntVal, port.TargetPort.StrVal)
		if !ok {
			warnf("No backend for service port %d (target: %d)", port.Port, port.TargetPort.IntVal)
			continue
		}

		fid := getVulcanID(s.Name, s.Namespace, port.Name)
		fnd := NewFrontend(fid, bnd.ID)
		fnd.Route = buildRoute(port.Name, s.Annotations)
		if st, ok := s.Annotations[labelf("frontendSettings")]; ok {
			fnd.Settings = NewFrontendSettings([]byte(st))
		}
		debugf("Frontend settings: %q", fnd)

		val, err := fnd.Val()
		if err != nil {
			return NewErr(err, "Could not encode frontend for %q", s.Name)
		}
		eVal, _ := etcd.Val(fnd.Key())
		if val != eVal {
			debugf("Upserting frontend %q", fnd)
			if err := etcd.Add(fnd.Key(), val); err != nil {
				return NewErr(err, "etcd error")
			}
		} else {
			debugf("No changes, not upserting Frontend %q", fnd)
		}
	}
	return nil
}

func registerService(s *api.Service) error {
	e, ok := getEndpoints(s.Name, s.Namespace)
	if !ok {
		return NewErr(nil, "kubernetes error")
	}

	return register(s, e)
}

func registerEndpoints(e *api.Endpoints) error {
	s, ok := getService(e.Name, e.Namespace)
	if !ok {
		return NewErr(nil, "kubernetes error")
	}

	return register(s, e)
}

func register(s *api.Service, e *api.Endpoints) error {
	if !registerable(s) {
		debugf("Service '%s-%s' not registerable", s.Name, s.Namespace)
		return nil
	}

	bnds, er := registerBackends(s, e)
	if er != nil {
		return NewErr(er, "Backend Error")
	}

	if er := registerFrontends(s, bnds); er != nil {
		return NewErr(er, "Frontend Error")
	}

	return nil
}

func deregisterService(s *api.Service) error {
	etcd.SetPrefix(getVulcanKey(s))
	defer etcd.SetPrefix(*vulcanKey)
	for _, port := range s.Spec.Ports {
		f := NewFrontend(getVulcanID(s.Name, s.Namespace, port.Name), "")
		infof("Deregistering frontend %v", f)
		if er := etcd.Del(f.DirKey()); er != nil {
			if isKeyNotFound(er) {
				warnf("%s frontend key not found in etcd", f.ID)
				continue
			}
			return NewErr(er, "etcd error")
		}
	}

	cache.del(cKey{s.Name, s.Namespace, s.Kind})
	return nil
}

func deregisterEndpoints(e *api.Endpoints) error {
	etcd.SetPrefix(getVulcanKey(e))
	defer etcd.SetPrefix(*vulcanKey)
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			b := NewBackend(getVulcanID(e.Name, e.Namespace, port.Name))
			infof("Deregistering backend %v", b)
			if er := etcd.Del(b.DirKey()); er != nil {
				if isKeyNotFound(er) {
					warnf("%s backend key not found in etcd", b.ID)
					continue
				}
				return NewErr(er, "etcd error")
			}
		}
	}

	cache.del(cKey{e.Name, e.Namespace, e.Kind})
	return nil
}

func registerable(o runtime.Object) bool {
	if _, ok := o.(*uApi.Status); ok {
		return true
	}
	m, er := getMeta(o)
	if er != nil {
		debugf("Failed to access labels: %v", er)
		return false
	}

	for k, v := range *svcSel {
		if val, ok := m.labels[labelf(k)]; !ok || val != v {
			return false
		}
	}
	return true
}
