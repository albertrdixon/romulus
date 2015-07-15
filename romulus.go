package main

import "fmt"

type StopChan chan struct{}

var (
	bckndDirFmt  = "/vulcan/backends/%s"
	frntndDirFmt = "/vulcan/frontends/%s"
	bckndFmt     = "/vulcan/backends/%s/backend"
	srvrFmt      = "/vulcan/backends/%s/servers/%d"
	frntndFmt    = "/vulcan/frontends/%s/frontend"
)

func RegisterWithVulcan(c *Client, service string, endpoints Endpoints) {
	bk := fmt.Sprintf(bckndFmt, service)
	c.e.Set(bk, `{"Type": "http"}`, 0)

	for i, e := range endpoints {
		sk := fmt.Sprintf(srvrFmt, service, i)
		sv := fmt.Sprintf(`{"URL": "%s"}`, e)
		c.e.Set(sk, sv, 0)
	}

	fk := fmt.Sprintf(frntndFmt, service)
	fv := fmt.Sprintf(`{"Type": "http", "BackendId": "%s", "Route": "Path('/')"}`, service)
	c.e.Set(fk, fv, 0)
}

func DeregisterWithVulcan(c *Client, service string) {
	bk, fk := fmt.Sprintf(bckndDirFmt, service), fmt.Sprintf(frntndDirFmt, service)
	c.e.Delete(bk, true)
	c.e.Delete(fk, true)
}
