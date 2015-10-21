package main

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
)

type etcdInterface interface {
	Add(key, val string) error
	Keys(pre string) ([]string, error)
	Del(key string) error
	Val(key string) (string, error)
	SetPrefix(pre string)
}

type etcdClient struct {
	client.KeysAPI
	*sync.RWMutex
	p string
	t time.Duration
}

func NewEtcdClient(peers []*url.URL, prefix string, timeout time.Duration) (etcdInterface, error) {
	if *etcdDebug {
		client.EnablecURLDebug()
	}
	sp := []string{}
	for _, p := range peers {
		sp = append(sp, p.String())
	}
	ec, er := client.New(client.Config{Endpoints: sp})
	if er != nil {
		return nil, er
	}
	return &etcdClient{client.NewKeysAPI(ec), new(sync.RWMutex),
		prefix, timeout}, nil
}

func (r *etcdClient) SetPrefix(pre string) {
	r.p = pre
}

func (r *etcdClient) Add(k, v string) error {
	r.Lock()
	defer r.Unlock()
	return r.add(prefix(r.p, k), v)
}

func (r *etcdClient) add(k, v string) error {
	c, q := context.WithTimeout(context.Background(), r.t)
	defer q()

	_, e := r.Set(c, k, v, nil)
	return e
}

func (r *etcdClient) Del(k string) error {
	r.Lock()
	defer r.Unlock()
	return r.del(prefix(r.p, k))
}

func (r *etcdClient) del(k string) error {
	c, q := context.WithTimeout(context.Background(), r.t)
	defer q()

	_, e := r.Delete(c, k, &client.DeleteOptions{Recursive: true})
	if isKeyNotFound(e) {
		return nil
	}
	return e
}

func (r *etcdClient) Keys(k string) ([]string, error) {
	r.RLock()
	defer r.RUnlock()
	return r.keys(prefix(r.p, k))
}

func (re *etcdClient) keys(p string) ([]string, error) {
	c, q := context.WithTimeout(context.Background(), re.t)
	defer q()

	r, e := re.Get(c, p, &client.GetOptions{Recursive: true, Sort: true, Quorum: true})
	if e != nil {
		if isKeyNotFound(e) {
			return []string{}, nil
		}
		return []string{}, e
	}
	if r.Node == nil {
		return []string{}, nil
	}

	k := make([]string, 0, len(r.Node.Nodes))
	for _, n := range r.Node.Nodes {
		str := strings.TrimLeft(strings.TrimPrefix(n.Key, p), "/")
		k = append(k, str)
	}
	return k, nil
}

func (r *etcdClient) Val(key string) (string, error) {
	r.RLock()
	defer r.RUnlock()
	return r.get(prefix(r.p, key))
}

func (r *etcdClient) get(key string) (string, error) {
	c, q := context.WithTimeout(context.Background(), r.t)
	defer q()

	resp, e := r.Get(c, key, &client.GetOptions{Recursive: false, Quorum: true})
	if e != nil {
		if isKeyNotFound(e) {
			return "", nil
		}
		return "", e
	}
	if resp.Node == nil || resp.Node.Dir {
		return "", nil
	}

	return resp.Node.Value, nil
}

func isKeyNotFound(e error) bool {
	if e == nil {
		return false
	}
	switch e := e.(type) {
	default:
		return false
	case client.Error:
		if e.Code == client.ErrorCodeKeyNotFound {
			return true
		}
		return false
	}
}

func prefix(p, s string) string {
	return strings.Join([]string{p, s}, "/")
}
