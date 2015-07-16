package romulus

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

var (
	bckndIDAnnotation = "domain"

	bckndDirFmt  = "/vulcan/backends/%s"
	frntndDirFmt = "/vulcan/frontends/%s"
	bckndFmt     = "/vulcan/backends/%s/backend"
	srvrDirFmt   = "/vulcan/backends/%s/servers"
	srvrFmt      = "/vulcan/backends/%s/servers/%d"
	frntndFmt    = "/vulcan/frontends/%s/frontend"

	bckndCfg     = `{"Type": "http"}`
	srvrCfgFmt   = `{"URL": "%s"}`
	frntndCfgFmt = `{"Type": "http", "BackendId": "%s", "Route": "Path('/')"}`

	stop chan struct{}
)

func Version() string {
	return version
}

func Start(c *Client) error {
	stop = make(chan struct{}, 1)
	c.l.Debug("Setting watch on services")
	w, e := c.setWatch()
	if e != nil {
		return e
	}
	go func() {
		for {
			select {
			case e := <-w.ResultChan():
				if er := doEvent(c, e); er != nil {
					c.l.Error("Error!", "error", er)
				}
			case <-stop:
				c.l.Info("Received stop, ending watch")
				w.Stop()
				return
			}
		}
	}()
	return nil
}

func Stop() { stop <- struct{}{} }

func register(c *Client, r runtime.Object) error {
	s, ok := svc(r)
	if !ok {
		return Error{fmt.Sprintf("Unrecognized api object: %v", r), nil}
	}
	if !registerable(s, c.s) {
		c.l.Debug("Service not registerable", "service", s.Name, "namespace", s.Namespace)
		return nil
	}

	id, en, bkc := getBackendID(s), getEndpoints(s), getBackendConfig(s)
	c.l.Info("Registering service", "service", s.Name, "namespace", s.Namespace, "domain", id)

	bk := fmt.Sprintf(bckndFmt, id)
	bc := bckndCfg
	if bkc != "" {
		bc = bkc
	}
	if _, e := c.e.Set(bk, bc, 0); e != nil {
		return Error{"etcd error", e}
	}

	srs := fmt.Sprintf(srvrDirFmt, id)
	if r, _ := c.e.Get(srs, false, false); r != nil {
		if _, e := c.e.Delete(srs, true); e != nil {
			return Error{"etcd error", e}
		}
	}
	for i, e := range en {
		sk := fmt.Sprintf(srvrFmt, id, i)
		sv := fmt.Sprintf(srvrCfgFmt, e)
		if _, e := c.e.Set(sk, sv, 0); e != nil {
			return Error{"etcd error", e}
		}
	}

	fk := fmt.Sprintf(frntndFmt, id)
	fv := fmt.Sprintf(frntndCfgFmt, id)
	if _, e := c.e.Set(fk, fv, 0); e != nil {
		return Error{"etcd error", e}
	}
	return nil
}

func deregister(c *Client, r runtime.Object) error {
	s, ok := svc(r)
	if !ok {
		return Error{fmt.Sprintf("Unrecognized api object: %v", r), nil}
	}

	id := getBackendID(s)
	c.l.Info("Deregistering service", "service", s.Name, "namespace", s.Namespace, "domain", id)

	bk, fk := fmt.Sprintf(bckndDirFmt, id), fmt.Sprintf(frntndDirFmt, id)
	if _, e := c.e.Delete(bk, true); e != nil {
		return Error{"etcd error", e}
	}
	if _, e := c.e.Delete(fk, true); e != nil {
		return Error{"etcd error", e}
	}
	return nil
}

func svc(r runtime.Object) (a *api.Service, b bool) {
	a, b = r.(*api.Service)
	return
}
