package romulus

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

var (
	bckndSettingsAnnotation  = "romulus/backendSettings"
	frntndSettingsAnnotation = "romulus/frontendSettings"

	stop chan struct{}
)

// Version returns the current software version
func Version() string {
	if SHA != "" {
		return fmt.Sprintf("%s-%s", version, SHA)
	}
	return version
}

// Start boots up the daemon
func Start(c *Registrar) error {
	stop = make(chan struct{}, 1)
	log().Debugf("Selecting objects that match: %s", c.s.String())
	log().Debug("Setting watch on Endpoints")
	ee, e := c.initEndpoints()
	if e != nil {
		return e
	}
	log().Debug("Setting watch on Services")
	se, e := c.initServices()
	if e != nil {
		return e
	}
	go func() {
		for {
			select {
			case e := <-ee.ResultChan():
				go func() {
					if er := do(c, e); er != nil {
						logf(fi{"error": er}).Error("Error!")
					}
				}()
			case e := <-se.ResultChan():
				go func() {
					if er := do(c, e); er != nil {
						logf(fi{"error": er}).Error("Error!")
					}
				}()
			case <-stop:
				log().Info("Received stop, closing watch channels")
				ee.Stop()
				se.Stop()
				return
			}
		}
	}()
	return nil
}

// Stop shuts down the daemon threads
func Stop() { stop <- struct{}{} }

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
		if err := r.e.Del(f.DirKey(r.vk)); err != nil {
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
			if err := r.e.Del(b.DirKey(r.vk)); err != nil {
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
