package romulus

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

var (
	bckndSettingsAnnotation  = "backendSettings"
	frntndSettingsAnnotation = "frontendSettings"

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
	ee, e := c.endpointsEventChannel()
	if e != nil {
		return e
	}
	log().Debug("Setting watch on Services")
	se, e := c.serviceEventsChannel()
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

func register(c *Registrar, e *api.Endpoints) error {
	s, err := c.getService(e.Name, e.Namespace)
	if err != nil {
		logf(fi{"msg": err, "endpoint": e.Name}).Warn("Could not get service to match endpoint")
		return nil
	}

	if !registerable(s, c.s) {
		logf(fi{"service": s.Name, "namespace": s.Namespace}).Debug("Service not registerable")
		return nil
	}

	eid, sid := getUUID(e.ObjectMeta), getUUID(s.ObjectMeta)
	if eid.String() == "" {
		return fmt.Errorf("Endpoint %q has no uuid", e.Name)
	}
	if sid.String() == "" {
		return fmt.Errorf("Service %q has no uuid", s.Name)
	}

	logf(fi{"service": s.Name, "namespace": s.Namespace,
		"bcknd-id": eid.String(), "frntnd-id": sid.String()}).
		Info("Registering service")
	bnd := NewBackend(eid)
	bnd.Type = "http"

	if st, ok := s.Annotations[bckndSettingsAnnotation]; ok {
		bnd.Settings = NewBackendSettings([]byte(st))
	}
	logf(fi{"bcknd-id": bnd.ID.String(), "type": bnd.Type, "settings": bnd.Settings.String()}).Debug("Backend settings")

	val, err := bnd.Val()
	if err != nil {
		return NewErr(err, "Could not encode backend for %q", e.Name)
	}
	if err := c.e.Add(bnd.Key(c.vk), val); err != nil {
		return NewErr(err, "etcd error")
	}

	sm := expandEndpoints(eid, e)
	logf(fi{"servers": sm.IPs(), "bcknd-id": eid.String()}).Debug("Expanded endpoints")
	if err := c.pruneServers(eid, sm); err != nil {
		return NewErr(err, "Unable to prune servers for backend %q", e.Name)
	}

	for _, srv := range sm {
		val, err = srv.Val()
		if err != nil {
			logf(fi{"service": s.Name, "namespace": s.Namespace,
				"server": srv.URL.String(), "error": err}).
				Warn("Unable to encode server")
			continue
		}
		if err := c.e.Add(srv.Key(c.vk), val); err != nil {
			return NewErr(err, "etcd error")
		}
	}

	fnd := NewFrontend(sid, eid)
	fnd.Type = "http"
	fnd.Route = buildRoute(s.Annotations)

	if st, ok := s.Annotations[frntndSettingsAnnotation]; ok {
		fnd.Settings = NewFrontendSettings([]byte(st))
	}
	logf(fi{"frntnd-id": fnd.ID.String(), "type": fnd.Type, "route": fnd.Route, "settings": fnd.Settings.String()}).Debug("Frontend settings")

	val, err = fnd.Val()
	if err != nil {
		return NewErr(err, "Could not encode frontend for %q", s.Name)
	}
	if err := c.e.Add(fnd.Key(c.vk), val); err != nil {
		return NewErr(err, "etcd error")
	}

	return nil
}

func deregister(c *Registrar, o api.ObjectMeta, frontend bool) error {
	id := getUUID(o)
	if id.String() == "" {
		return fmt.Errorf("Unable to get uuid for %q", o.Name)
	}

	if frontend {
		logf(fi{"service": o.Name, "namespace": o.Namespace, "frntnd-id": id.String()}).Info("Deregistering frontend")
		f := Frontend{ID: id}
		if err := c.e.Del(f.DirKey()); err != nil {
			return NewErr(err, "etcd error")
		}
	} else {
		logf(fi{"service": o.Name, "namespace": o.Namespace, "bcknd-id": id.String()}).Info("Deregistering backend")
		b := Backend{ID: id}
		if err := c.e.Del(b.DirKey()); err != nil {
			return NewErr(err, "etcd error")
		}
	}
	return nil
}
