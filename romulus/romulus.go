package romulus

import (
	"fmt"
	"strings"

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

func Version() string {
	return version
}

func Start(c *Client) error {
	stop = make(chan struct{}, 1)
	c.l.Debug("Setting watch on Endpoints")
	ee, e := c.endpointsEventChannel()
	if e != nil {
		return e
	}
	c.l.Debug("Setting watch on Services")
	se, e := c.serviceEventsChannel()
	go func() {
		for {
			select {
			case e := <-ee.ResultChan():
				go func() {
					if er := doEndpointsEvent(c, e); er != nil {
						c.l.Error("Error!", "error", er)
					}
				}()
			case e := <-se.ResultChan():
				go func() {
					if er := doServiceEvent(c, e); er != nil {
						c.l.Error("Error!", "error", er)
					}
				}()
			case <-stop:
				c.l.Info("Received stop, closing watch channels")
				ee.Stop()
				se.Stop()
				return
			}
		}
	}()
	return nil
}

func Stop() { stop <- struct{}{} }

func register(c *Client, e *api.Endpoints) error {
	s, err := c.getService(e.Name, e.Namespace)
	if err != nil {
		c.l.Warn(fmt.Sprintf("Could not get service to match endpoint %q", e.Name), "msg", err.Error())
		return nil
	}

	if !registerable(s, c.s) {
		c.l.Debug("Service not registerable", "service", s.Name, "namespace", s.Namespace)
		return nil
	}

	eid, sid := getUUID(e.ObjectMeta), getUUID(s.ObjectMeta)
	if eid.String() == "" {
		return fmt.Errorf("Endpoint %q has no uuid", e.Name)
	}
	if sid.String() == "" {
		return fmt.Errorf("Service %q has no uuid", s.Name)
	}

	c.l.Info("Registering service", "service", s.Name, "namespace", s.Namespace)
	c.l.Debug(fmt.Sprintf("Backend uuid: %s", eid.String()))
	bnd := Backend{
		ID:   eid,
		Type: "http",
	}
	if st, ok := s.Annotations[bckndSettingsAnnotation]; ok {
		bnd.Settings = NewBackendSettings([]byte(st))
	}

	val, err := bnd.Val()
	if err != nil {
		return Error{fmt.Sprintf("Could not encode backend for %q", e.Name), err}
	}
	if _, err := c.e.Set(bnd.Key(), strings.TrimSpace(val), 0); err != nil {
		return Error{"etcd error", err}
	}

	sm := expandEndpoints(eid, e)
	c.l.Debug(fmt.Sprintf("Expanded ednpoints: %v", sm))
	if err := c.pruneServers(eid, sm); err != nil {
		return Error{fmt.Sprintf("Unable to prune servers for backend %q", e.Name), err}
	}

	for _, srv := range sm {
		val, err = srv.Val()
		if err != nil {
			c.l.Warn("Unable to encode server", "service", s.Name, "namespace", s.Namespace, "server", srv.URL.String(), "error", err)
			continue
		}
		if _, err := c.e.Set(srv.Key(), strings.TrimSpace(val), 0); err != nil {
			return Error{"etcd error", err}
		}
	}

	c.l.Debug(fmt.Sprintf("Frontend uuid: %s", sid.String()))
	fnd := Frontend{
		ID:        sid,
		Type:      "http",
		BackendID: eid,
		Route:     buildRoute(s.Annotations),
	}
	if st, ok := s.Annotations[frntndSettingsAnnotation]; ok {
		fnd.Settings = NewFrontendSettings([]byte(st))
	}

	val, err = fnd.Val()
	if err != nil {
		return Error{fmt.Sprintf("Could not encode frontend for %q", s.Name), err}
	}
	if _, err := c.e.Set(fnd.Key(), strings.TrimSpace(val), 0); err != nil {
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
		c.l.Info("Deregistering frontend", "service", o.Name, "namespace", o.Namespace)
		c.l.Debug(fmt.Sprintf("Frontend uuid: %s", id.String()))
		f := Frontend{ID: id}
		if _, err := c.e.Delete(f.DirKey(), true); err != nil {
			return Error{"etcd error", err}
		}
		if _, err := c.e.DeleteDir(f.Key()); err != nil {
			if isKeyNotFound(err) {
				return nil
			}
			return Error{"etcd error", err}
		}
	} else {
		c.l.Info("Deregistering backend", "service", o.Name, "namespace", o.Namespace)
		c.l.Debug(fmt.Sprintf("Backend uuid: %s", id.String()))
		b := Backend{ID: id}
		if _, err := c.e.Delete(b.DirKey(), true); err != nil {
			return Error{"etcd error", err}
		}
		if _, err := c.e.DeleteDir(b.Key()); err != nil {
			if isKeyNotFound(err) {
				return nil
			}
			return Error{"etcd error", err}
		}
	}
	return nil
}
