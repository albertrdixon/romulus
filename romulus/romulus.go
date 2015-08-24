package romulus

import (
	"fmt"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
)

var (
	bckndSettingsAnnotation  = "romulus/backendSettings"
	frntndSettingsAnnotation = "romulus/frontendSettings"
)

// Version returns the current software version
func Version() string {
	if SHA != "" {
		return fmt.Sprintf("%s-%s", version, SHA)
	}
	return version
}

// Start boots up the daemon
func Start(r *Registrar, c context.Context) error {
	log().Debugf("Selecting objects that match: %s", r.s.String())
	w, er := initEvents(r, c)
	if er != nil {
		return er
	}
	go start(r, w, c)
	return nil
}

func start(r *Registrar, w <-chan Event, c context.Context) {
	for {
		select {
		case <-c.Done():
			return
		case e := <-w:
			if registerable(e.Object, r.s) {
				go func() {
					if er := event(r, e); er != nil {
						log().Error(er.Error())
					}
				}()
			}
		}
	}
}

func registerService(r *Registrar, s *api.Service) error {
	e, err := r.getEndpoint(s.Name, s.Namespace)
	if err != nil {
		if kubeIsNotFound(err) {
			logf(fi{"msg": err, "service": s.Name, "namespace": s.Namespace}).Warn("Service has no associated endpoint")
			return nil
		}
		return NewErr(err, "kubernetes error")
	}

	return register(r, s, e)
}

func registerEndpoint(r *Registrar, e *api.Endpoints) error {
	s, err := r.getService(e.Name, e.Namespace)
	if err != nil {
		if kubeIsNotFound(err) {
			logf(fi{"msg": err, "endpoint": e.Name, "namespace": e.Namespace}).Warn("Could not get service to match endpoint")
			return nil
		}
		return NewErr(err, "kubernetes error")
	}

	return register(r, s, e)
}

func register(r *Registrar, s *api.Service, e *api.Endpoints) error {
	if !registerable(s, r.s) {
		logf(fi{"service": s.Name, "namespace": s.Namespace}).Debug("Service not registerable")
		return nil
	}

	bnds, err := r.registerBackends(s, e)
	if err != nil {
		return NewErr(err, "Backend Error")
	}

	if err := r.registerFrontends(s, bnds); err != nil {
		return NewErr(err, "Frontend Error")
	}

	return nil
}

func deregisterService(r *Registrar, s *api.Service) error {
	for _, port := range s.Spec.Ports {
		fid := getVulcanID(s.Name, s.Namespace, port.Name)
		logf(fi{"service": s.Name, "namespace": s.Namespace, "id": fid}).Info("Deregistering frontend")
		f := NewFrontend(fid, "")
		if err := r.e.Del(f.DirKey()); err != nil {
			if isKeyNotFound(err) {
				logf(fi{"service": s.Name, "namespace": s.Namespace, "id": fid}).Warn("Frontend key not found in etcd")
				continue
			}
			return NewErr(err, "etcd error")
		}
	}
	return nil
}

func deregisterEndpoints(r *Registrar, e *api.Endpoints) error {
	for _, es := range e.Subsets {
		for _, port := range es.Ports {
			bid := getVulcanID(e.Name, e.Namespace, port.Name)
			logf(fi{"service": e.Name, "namespace": e.Namespace, "id": bid}).Info("Deregistering backend")
			b := NewBackend(bid)
			if err := r.e.Del(b.DirKey()); err != nil {
				if isKeyNotFound(err) {
					logf(fi{"service": e.Name, "namespace": e.Namespace, "id": bid}).Warn("Backend key not found in etcd")
					continue
				}
				return NewErr(err, "etcd error")
			}
		}
	}
	return nil
}
