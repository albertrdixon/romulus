package romulus

import (
	"fmt"
	"net/url"
	"strings"

	jURL "github.com/albertrdixon/gearbox/url"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/runtime"
)

type serviceSelector map[string]string

func (s serviceSelector) format() ServiceSelector {
	ss := make(map[string]string, len(s))
	for k := range s {
		key := k
		if !strings.HasPrefix(k, "romulus/") {
			key = fmt.Sprintf("romulus/%s", key)
		}
		ss[key] = s[k]
	}
	return serviceSelector(ss)
}

func (sl serviceSelector) String() string {
	s := []string{}
	for k, v := range sl {
		s = append(s, strings.Join([]string{k, v}, "="))
	}
	return strings.Join(s, ", ")
}

func pruneServers(bid string, sm ServerMap) error {
	k := fmt.Sprintf(srvrDirFmt, bid)
	srvs, e := etcd.Keys(k)
	if e != nil {
		if isKeyNotFound(e) {
			return nil
		}
		return NewErr(e, "etcd error")
	}

	log.Debugf("Gathered known servers from kubernetes: %v", sm)
	log.Debugf("Gathered known servers from etcd: %v", srvs)
	for _, id := range srvs {
		key := fmt.Sprintf("%s/%s", k, id)
		s, e := etcd.Val(key)
		if e != nil {
			log.Warnf("Error getting server from etcd: %v", e)
			continue
		}

		srv := &Server{ID: id, Backend: bid}
		if e := decode(srv, []byte(s)); e != nil {
			log.Errorf("Unable to unmarshall Server: %v", e)
			log.Debugf("Data: %s", s)
			if e := etcd.Del(key); e != nil {
				log.Errorf("Error removing server: %v", e)
				continue
			}
		}

		sTag := md5Hash(bid, srv.URL.String())[:serverTagLen]
		if nSrv, ok := sm[sTag]; ok {
			log.Debugf("Server %q exists", srv.ID)
			nSrv.ID = srv.ID
		} else {
			log.Infof("Removing Server %q", srv.ID)
			if e := etcd.Del(key); e != nil {
				log.Errorf("Error removing server: %v", e)
				continue
			}
		}
	}
	return nil
}

func pruneBackends() error {
	ids, err := etcd.Keys(bcknds)
	if err != nil {
		if isKeyNotFound(err) {
			return nil
		}
		return NewErr(err, "etcd error")
	}

	log.Debugf("Found current backends: %v", ids)
	for _, id := range ids {
		name, ns, e := parseVulcanID(id)
		if e != nil {
			log.Errorf("Invalid ID: %s", id)
			key := fmt.Sprintf(bckndDirFmt, id)
			if e := etcd.Del(key); e != nil {
				log.Warningf("etcd error: %v", e)
			}
		} else if _, ok := getEndpoint(name, ns); err != nil && kubeIsNotFound(err) {
			log.Warningf("Did not find backend on API server: %v", err)
			b := NewBackend(id)
			if err := etcd.Del(b.DirKey()); err != nil {
				log.Warn("etcd error")
			}
		}
	}
	return nil
}

func pruneFrontends() error {
	ids, err := etcd.Keys(frntnds)
	if err != nil {
		if isKeyNotFound(err) {
			return nil
		}
		return NewErr(err, "etcd error")
	}

	log.Debugf("Found current frontends: %v", ids)
	for _, id := range ids {
		name, ns, e := parseVulcanID(id)
		if e != nil {
			log.Errorf("Invalid ID: %s", id)
			key := fmt.Sprintf(frntndDirFmt, id)
			if e := etcd.Del(key); e != nil {
				log.Warningf("etcd error: %v", e)
			}
		} else if _, ok := getService(name, ns); !ok {
			log.Warningf("Did not find '%s-%s' frontend on API server: %v", name, ns, err)
			f := NewFrontend(id, "")
			if err := etcd.Del(f.DirKey()); err != nil {
				log.Warningf("etcd error: %v", err)
			}
		}
	}
	return nil
}

func registerBackends(s *api.Service, e *api.Endpoints) (*BackendList, error) {
	bnds := NewBackendList()
	log.Infof("Registering backend '%s-%s'", e.Name, e.Namespace)
	pruneBackends()
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			if port.Protocol != api.ProtocolTCP {
				log.Warningf("Unsupported protocol: %s", port.Protocol)
				continue
			}

			sm := ServerMap{}
			bid := getVulcanID(e.Name, e.Namespace, port.Name)
			bnd := NewBackend(bid)

			if st, ok := s.Annotations[bckndSettingsAnnotation]; ok {
				bnd.Settings = NewBackendSettings([]byte(st))
			}
			log.Debugf("Backend settings: %v", bnd)

			log.Debug("Gathering kubernetes endpoints: %v", es.Addresses)
			for _, ip := range es.Addresses {
				ur := fmt.Sprintf("http://%s:%d", ip.IP, port.Port)
				u, err := url.Parse(ur)
				if err != nil {
					log.Warningf("Bad URL: %s", ur)
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
				log.Warningf("Failed to remove servers for %q: %v", bnd.ID, err)
			}

			val, err := bnd.Val()
			if err != nil {
				return bnds, NewErr(err, "Could not encode backend for %q", e.Name)
			}
			eVal, _ := etcd.Val(bnd.Key())
			if val != eVal {
				log.Debugf("Upserting backend %s", bnd.ID)
				if err := etcd.Add(bnd.Key(), val); err != nil {
					return bnds, NewErr(err, "etcd error")
				}
			} else {
				log.Debugf("No changes, not upserting Backend %q", bnd.ID)
			}
			bnds.Add(port.Port, port.Name, bnd)

			for _, srv := range sm {
				val, err := srv.Val()
				if err != nil {
					log.Warningf("Unable to encode server %q: %v", srv.ID, err)
					continue
				}
				eVal, _ := etcd.Val(srv.Key())
				if val != eVal {
					log.Debugf("Upserting server %q", srv)
					if err := etcd.Add(srv.Key(), val); err != nil {
						return bnds, NewErr(err, "etcd error")
					}
				} else {
					log.Debugf("No changes, not upserting Server %q", srv.ID)
				}
			}
		}
	}
	return bnds, nil
}

func registerFrontends(s *api.Service, bnds *BackendList) error {
	log.Infof("Registering frontend '%s-%s'", s.Name, s.Namespace)
	pruneFrontends()
	log.Debugf("Backend List: %+v", bnds)
	for _, port := range s.Spec.Ports {
		bnd, ok := bnds.Lookup(port.TargetPort.IntVal, port.TargetPort.StrVal)
		if !ok {
			log.Warningf("No backend for service port %d (target: %d)", port.Port, port.TargetPort.IntVal)
			continue
		}

		fid := getVulcanID(s.Name, s.Namespace, port.Name)
		fnd := NewFrontend(fid, bnd.ID)
		fnd.Route = buildRoute(port.Name, s.Annotations)
		if st, ok := s.Annotations[frntndSettingsAnnotation]; ok {
			fnd.Settings = NewFrontendSettings([]byte(st))
		}
		log.Debugf("Frontend settings: %v", fnd)

		val, err := fnd.Val()
		if err != nil {
			return NewErr(err, "Could not encode frontend for %q", s.Name)
		}
		eVal, _ := etcd.Val(fnd.Key())
		if val != eVal {
			log.Debugf("Upserting frontend %q", fnd.ID)
			if err := etcd.Add(fnd.Key(), val); err != nil {
				return NewErr(err, "etcd error")
			}
		} else {
			log.Debugf("No changes, not upserting Frontend %q", fnd.ID)
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
		log.Debugf("Service '%s-%s' not registerable", s.Name, s.Namespace)
		return nil
	}

	bnds, err := registerBackends(s, e)
	if err != nil {
		return NewErr(err, "Backend Error")
	}

	if err := registerFrontends(s, bnds); err != nil {
		return NewErr(err, "Frontend Error")
	}

	return nil
}

func deregisterService(s *api.Service) error {
	etcd.SetPrefix(getVulcanKey(s))
	defer etcd.SetPrefix(*vulcanKey)
	for _, port := range s.Spec.Ports {
		f := NewFrontend(getVulcanID(s.Name, s.Namespace, port.Name), "")
		log.Infof("Deregistering frontend %v", f)
		if er := etcd.Del(f.DirKey()); err != nil {
			if isKeyNotFound(err) {
				log.Warningf("%s frontend key not found in etcd", f.ID)
				continue
			}
			return NewErr(err, "etcd error")
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
			log.Infof("Deregistering backend %v", b)
			if err := etcd.Del(b.DirKey()); err != nil {
				if isKeyNotFound(err) {
					log.Warningf("%s backend key not found in etcd", b.ID)
					continue
				}
				return NewErr(err, "etcd error")
			}
		}
	}

	cache.del(cKey{s.Name, s.Namespace, s.Kind})
	return nil
}

func registerable(o runtime.Object, sl ServiceSelector) bool {
	if _, ok := o.(*unvApi.Status); ok {
		return true
	}
	m := meta.NewAccessor()
	la, er := m.Labels(o)
	if er != nil {
		log().Debugf("Failed to access labels: %v", er)
		return false
	}

	for k, v := range sl.fixNamespace() {
		if val, ok := la[k]; !ok || val != v {
			return false
		}
	}
	return true
}
