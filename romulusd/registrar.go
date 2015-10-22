package main

import (
	"fmt"
	"net/url"

	jURL "github.com/albertrdixon/gearbox/url"
	"k8s.io/kubernetes/pkg/api"
	eps "k8s.io/kubernetes/pkg/api/endpoints"
	uApi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// Remove Servers that are misconfigured or exist in etcd,
// but NOT in the api.Endpoints object from kubernetes.
func pruneServers(bid string, sm ServerMap) error {
	k := serverDirf(bid)
	srvs, e := etcd.Keys(k)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return NewErr(e, "etcd error")
	}

	if isDebug() {
		debugf("Gathered known servers from kubernetes: %v", sm)
		debugf("Gathered known servers from etcd: %v", ppSlice(srvs))
	}
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

		if nSrv, ok := sm[srv.URL.String()]; ok {
			debugf("Exists: %v", srv)
			nSrv.ID = srv.ID
		} else {
			infof("Removing %v", srv)
			if e := etcd.Del(key); e != nil {
				errorf("Error removing server: %v", e)
				continue
			}
		}
	}
	return nil
}

// Gathers information from given api.Endpoints object and parses into a Backend object for each
// IP set / port combination. Will attempt to upsert backends into etcd.
func registerBackends(e *api.Endpoints, s *api.Service) (*BackendList, error) {
	bnds := NewBackendList()
	subsets := eps.RepackSubsets(e.Subsets)
	debugf("Processing %v, %v", endpoints{e}, epSubsets(subsets))
	for _, es := range subsets {
		for _, port := range es.Ports {
			bid := getVulcanID(e.Name, e.Namespace, port.Name)
			bnd := NewBackend(bid)
			debugf("Working on %v", bnd)

			if port.Protocol != api.ProtocolTCP {
				warnf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			if st, ok := s.Annotations[labelf("backendSettings", port.Name)]; ok {
				bnd.Settings = NewBackendSettings([]byte(st))
				debugf("Backend settings: %q", bnd.Settings)
			}

			sm := expandEndpointSubset(bid, es, port)
			if len(sm) < 1 {
				warnf("No ready addresses for port {%s:%d}: %v", port.Name, port.Port, epSubset(es))
				continue
			}
			if err := pruneServers(bid, sm); err != nil {
				warnf("Failed to remove servers for %v: %v", bnd, err)
			}

			val, err := bnd.Val()
			if err != nil {
				return bnds, NewErr(err, "Could not encode backend for %v", endpoints{e})
			}
			eVal, _ := etcd.Val(bnd.Key())
			if val != eVal {
				infof("Registering %v", bnd)
				if err := etcd.Add(bnd.Key(), val); err != nil {
					return bnds, NewErr(err, "etcd error")
				}
			} else {
				debugf("No updates %v", bnd)
			}
			bnds.Add(port.Port, port.Name, bnd.ID)

			for _, srv := range sm {
				val, err := srv.Val()
				if err != nil {
					warnf("Unable to encode server %v: %v", srv, err)
					continue
				}
				eVal, _ := etcd.Val(srv.Key())
				if val != eVal {
					infof("Registering %v", srv)
					if err := etcd.Add(srv.Key(), val); err != nil {
						return bnds, NewErr(err, "etcd error")
					}
				} else {
					debugf("No updates %v", srv)
				}
			}
		}
	}

	if len(subsets) < 1 {
		debugf("No subsets for %v, deregistering Servers", endpoints{e})
		for _, port := range s.Spec.Ports {
			bnd := NewBackend(getVulcanID(e.Name, e.Namespace, port.Name))
			bnds.Add(port.Port, port.Name, bnd.ID)
			if err := pruneServers(bnd.ID, ServerMap{}); err != nil {
				warnf("Failed to remove servers for %v: %v", bnd, err)
			}
		}
	}
	return bnds, nil
}

// Gathers information from given api.Service object and parses into a Frontend object.
// Attempts to match api.Service.Spec.Ports with given Backend ports in order to match Frontend and Backend.
// Will attempt to upsert frontend into etcd.
func registerFrontends(s *api.Service, bnds *BackendList) error {
	debugf("Processing %v", service{s})
	debugf("%v", bnds)
	for _, port := range s.Spec.Ports {
		fid := getVulcanID(s.Name, s.Namespace, port.Name)
		fnd := NewFrontend(fid, "")
		debugf("Working on %v", fnd)

		var ok bool
		fnd.BackendID, ok = bnds.Lookup(port.TargetPort.IntVal, port.TargetPort.StrVal)
		if !ok {
			warnf("No Backend for service port %d (target: %d)", port.Port, port.TargetPort.IntVal)
			continue
		}

		fnd.Route = buildRoute(port.Name, s.Annotations)
		if st, ok := s.Annotations[labelf("frontendSettings", port.Name)]; ok {
			fnd.Settings = NewFrontendSettings([]byte(st))
			debugf("Frontend settings: %v", fnd.Settings)
		}

		val, err := fnd.Val()
		if err != nil {
			return NewErr(err, "Could not encode frontend for %v", service{s})
		}
		eVal, _ := etcd.Val(fnd.Key())
		if val != eVal {
			infof("Registering %v", fnd)
			if err := etcd.Add(fnd.Key(), val); err != nil {
				return NewErr(err, "etcd error")
			}
		} else {
			debugf("No updates %v", fnd)
		}
	}
	return nil
}

func register(s *api.Service, e *api.Endpoints) error {
	if !registerable(s) {
		debugf("%v not registerable", service{s})
		return nil
	}

	bnds, er := registerBackends(e, s)
	if er != nil {
		return NewErr(er, "Backend Error")
	}

	if er := registerFrontends(s, bnds); er != nil {
		return NewErr(er, "Frontend Error")
	}

	return nil
}

func deregisterService(s *api.Service) error {
	debugf("Attempting to deregister %v", service{s})
	for _, port := range s.Spec.Ports {
		f := NewFrontend(getVulcanID(s.Name, s.Namespace, port.Name), "")
		infof("Deregistering %v", f)
		if er := etcd.Del(f.DirKey()); er != nil {
			if isKeyNotFound(er) {
				warnf("Not found in etcd: %v", f)
				continue
			}
			return NewErr(er, "etcd error")
		}
	}
	return nil
}

func deregisterEndpoints(e *api.Endpoints) error {
	subsets := eps.RepackSubsets(e.Subsets)
	debugf("Attempting to deregister %v %v", endpoints{e}, epSubsets(subsets))
	for _, es := range subsets {
		for _, port := range es.Ports {
			b := NewBackend(getVulcanID(e.Name, e.Namespace, port.Name))
			infof("Deregistering %v", b)
			if er := etcd.Del(b.DirKey()); er != nil {
				if isKeyNotFound(er) {
					warnf("Not found in etcd: %v", b)
					continue
				}
				return NewErr(er, "etcd error")
			}
		}
	}
	return nil
}

// registerable returns true if the Object is configured to be registered with Romulus
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

func expandEndpointSubset(bid string, es api.EndpointSubset, p api.EndpointPort) ServerMap {
	debugf("Expanding kubernetes Endpoints: %v", epSubset(es))
	sm := ServerMap{}
	for _, ip := range es.Addresses {
		ur := fmt.Sprintf("http://%s:%d", ip.IP, p.Port)
		u, err := url.Parse(ur)
		if err != nil {
			warnf("Bad URL: %s", ur)
			continue
		}
		uu := (*jURL.URL)(u)
		sTag := md5Hash(bid, uu.String())[:serverTagLen]
		sm[uu.String()] = &Server{
			ID:      fmt.Sprintf("%s.%s", sTag, bid),
			Backend: bid,
			URL:     uu,
		}
	}
	return sm
}
