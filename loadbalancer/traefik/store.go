package traefik

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/coreos/etcd/client"
)

var KeyReadErr = errors.New("Key read error")

type Store interface {
	Exists(key string) error
	Keys(pre string) ([]string, error)
	Mkdir(path string) error

	Set(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
}

type etcd struct {
	client.KeysAPI
	*sync.Mutex
	timeout time.Duration
}

func NewEtcdStore(endpoints []string, timeout time.Duration) (Store, error) {
	cl, er := client.New(client.Config{Endpoints: endpoints})
	if er != nil {
		return nil, er
	}
	return &etcd{
		KeysAPI: client.NewKeysAPI(cl),
		Mutex:   new(sync.Mutex),
		timeout: timeout,
	}, nil
}

func (s *etcd) Exists(key string) (e error) {
	_, e = s.Get(key)
	return
}

func (e *etcd) Keys(prefix string) ([]string, error) {
	c, q := context.WithTimeout(context.Background(), e.timeout)
	o := &client.GetOptions{Sort: true, Quorum: true}
	node, er := e.get(c, q, o, prefix)
	if er != nil {
		return nil, er
	}
	logger.Debugf("From etcd: %v", node)
	return extractKeys(node)
}

func (e *etcd) Get(key string) (string, error) {
	c, q := context.WithTimeout(context.Background(), e.timeout)
	o := &client.GetOptions{Quorum: true}
	node, er := e.get(c, q, o, key)
	if er != nil {
		return "", er
	}
	logger.Debugf("From etcd: %v", node)
	return extractNodeValue(node)
}

func (e *etcd) Set(key, value string) error {
	c, q := context.WithTimeout(context.Background(), e.timeout)
	o := &client.SetOptions{PrevExist: client.PrevIgnore, TTL: 0}
	return e.set(c, q, o, key, value)
}

func (e *etcd) Mkdir(path string) error {
	c, q := context.WithTimeout(context.Background(), e.timeout)
	o := &client.SetOptions{PrevExist: client.PrevNoExist, Dir: true}
	return e.set(c, q, o, path, "")
}

func (e *etcd) Delete(key string) error {
	c, q := context.WithTimeout(context.Background(), e.timeout)
	o := &client.DeleteOptions{Recursive: true}
	return e.del(c, q, o, key)
}

func (e *etcd) get(c context.Context, fn context.CancelFunc, o *client.GetOptions, k string) (*client.Node, error) {
	e.Lock()
	defer e.Unlock()
	defer fn()

	resp, er := e.KeysAPI.Get(c, k, o)
	return resp.Node, er
}

func (e *etcd) set(c context.Context, fn context.CancelFunc, o *client.SetOptions, k, v string) (er error) {
	e.Lock()
	defer e.Unlock()
	defer fn()

	_, er = e.KeysAPI.Set(c, k, v, o)
	return
}

func (e *etcd) del(c context.Context, fn context.CancelFunc, o *client.DeleteOptions, k string) (er error) {
	e.Lock()
	defer e.Unlock()
	defer fn()

	_, er = e.KeysAPI.Delete(c, k, o)
	return
}

func extractNodeValue(node *client.Node) (string, error) {
	if node == nil {
		return "", KeyReadErr
	}
	return node.Value, nil
}

func extractKeys(node *client.Node) ([]string, error) {
	if node == nil {
		return nil, KeyReadErr
	}
	if !node.Dir {
		return []string{}, nil
	}
	keys := make([]string, 0, len(node.Nodes))
	for _, n := range node.Nodes {
		keys = append(keys, n.Value)
	}
	return keys, nil
}
