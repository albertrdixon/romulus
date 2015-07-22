package romulus

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

var (
	bckndSettingsAnnotation  = "backendSettings"
	frntndSettingsAnnotation = "frontendSettings"

	routeAnnotations = map[string]string{
		"host":   `Host('%s')`,
		"method": `Method('%s')`,
		"path":   `Path('%s')`,
		"header": `Header('%s')`,
	}

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
func Start(c *Client) error {
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
					if er := doEndpointsEvent(c, e); er != nil {
						logf(F{"error": er}).Error("Error!")
					}
				}()
			case e := <-se.ResultChan():
				go func() {
					if er := doServiceEvent(c, e); er != nil {
						logf(F{"error": er}).Error("Error!")
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

func register(c *Client, e *api.Endpoints) error {
	s, err := c.getService(e.Name, e.Namespace)
	if err != nil {
		logf(F{"msg": err, "endpoint": e.Name}).Warn("Could not get service to match endpoint")
		return nil
	}

	if !registerable(s, c.s) {
		logf(F{"service": s.Name, "namespace": s.Namespace}).Debug("Service not registerable")
		return nil
	}

	eid, sid := getUUID(e.ObjectMeta), getUUID(s.ObjectMeta)
	if eid.String() == "" {
		return fmt.Errorf("Endpoint %q has no uuid", e.Name)
	}
	if sid.String() == "" {
		return fmt.Errorf("Service %q has no uuid", s.Name)
	}

	logf(F{"service": s.Name, "namespace": s.Namespace,
		"bcknd-id": eid.String(), "frntnd-id": sid.String()}).
		Info("Registering service")
	bnd := Backend{
		ID:   eid,
		Type: "http",
	}
	if st, ok := s.Annotations[bckndSettingsAnnotation]; ok {
		bnd.Settings = NewBackendSettings([]byte(st))
	}
	logf(F{"bcknd-id": bnd.ID.String(), "type": bnd.Type, "settings": bnd.Settings.String()}).Debug("Backend settings")

	val, err := bnd.Val()
	if err != nil {
		return Error{fmt.Sprintf("Could not encode backend for %q", e.Name), err}
	}
	if _, err := c.e.Set(bnd.Key(), val, 0); err != nil {
		return Error{"etcd error", err}
	}

	sm := expandEndpoints(eid, e)
	logf(F{"servers": sm.IPs(), "bcknd-id": eid.String()}).Debug("Expanded endpoints")
	if err := c.pruneServers(eid, sm); err != nil {
		return Error{fmt.Sprintf("Unable to prune servers for backend %q", e.Name), err}
	}

	for _, srv := range sm {
		val, err = srv.Val()
		if err != nil {
			logf(F{"service": s.Name, "namespace": s.Namespace,
				"server": srv.URL.String(), "error": err}).
				Warn("Unable to encode server")
			continue
		}
		if _, err := c.e.Set(srv.Key(), val, 0); err != nil {
			return Error{"etcd error", err}
		}
	}

	fnd := Frontend{
		ID:        sid,
		Type:      "http",
		BackendID: eid,
		Route:     buildRoute(s.Annotations),
	}
	if st, ok := s.Annotations[frntndSettingsAnnotation]; ok {
		fnd.Settings = NewFrontendSettings([]byte(st))
	}
	logf(F{"frntnd-id": fnd.ID.String(), "type": fnd.Type, "route": fnd.Route, "settings": fnd.Settings.String()}).Debug("Frontend settings")

	val, err = fnd.Val()
	if err != nil {
		return Error{fmt.Sprintf("Could not encode frontend for %q", s.Name), err}
	}
	if _, err := c.e.Set(fnd.Key(), val, 0); err != nil {
		return Error{"etcd error", err}
	}

	return nil
}

func deregister(c *Client, o api.ObjectMeta, frontend bool) error {
	id := getUUID(o)
	if id.String() == "" {
		return fmt.Errorf("Unable to get uuid for %q", o.Name)
	}

	if frontend {
		logf(F{"service": o.Name, "namespace": o.Namespace, "frntnd-id": id.String()}).Info("Deregistering frontend")
		f := Frontend{ID: id}
		if _, err := c.e.Delete(f.DirKey(), true); err != nil {
			return Error{"etcd error", err}
		}
		if _, err := c.e.DeleteDir(f.DirKey()); err != nil {
			if isKeyNotFound(err) {
				return nil
			}
			return Error{"etcd error", err}
		}
	} else {
		logf(F{"service": o.Name, "namespace": o.Namespace, "bcknd-id": id.String()}).Info("Deregistering backend")
		b := Backend{ID: id}
		if _, err := c.e.Delete(b.DirKey(), true); err != nil {
			return Error{"etcd error", err}
		}
		if _, err := c.e.DeleteDir(b.DirKey()); err != nil {
			if isKeyNotFound(err) {
				return nil
			}
			return Error{"etcd error", err}
		}
	}
	return nil
}
