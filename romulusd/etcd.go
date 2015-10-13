package main

import (
	"path/filepath"
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

type realEtcdClient struct {
	client.KeysAPI
	*sync.RWMutex
	p string
	t time.Duration
}

type fakeEtcdClient struct {
	k map[string]string
	p string
}

func NewEtcdClient(peers []string, prefix string, timeout time.Duration) (etcdInterface, error) {
	if *etcdDebug {
		client.EnablecURLDebug()
	}
	ec, er := client.New(client.Config{Endpoints: peers})
	if er != nil {
		return nil, er
	}
	return &realEtcdClient{client.NewKeysAPI(ec), new(sync.RWMutex),
		prefix, timeout}, nil
}

func NewFakeEtcdClient(prefix string) etcdInterface {
	return &fakeEtcdClient{map[string]string{"/": ""}, prefix}
}

func (r *realEtcdClient) SetPrefix(pre string) {
	r.p = pre
}

func (r *realEtcdClient) Add(k, v string) error {
	r.Lock()
	defer r.Unlock()
	return r.add(prefix(r.p, k), v)
}

func (r *realEtcdClient) add(k, v string) error {
	c, q := context.WithTimeout(context.Background(), r.t)
	defer q()

	_, e := r.Set(c, k, v, nil)
	return e
}

func (r *realEtcdClient) Del(k string) error {
	r.Lock()
	defer r.Unlock()
	return r.del(prefix(r.p, k))
}

func (r *realEtcdClient) del(k string) error {
	c, q := context.WithTimeout(context.Background(), r.t)
	defer q()

	_, e := r.Delete(c, k, &client.DeleteOptions{Recursive: true})
	if isKeyNotFound(e) {
		return nil
	}
	return e
}

func (r *realEtcdClient) Keys(k string) ([]string, error) {
	r.RLock()
	defer r.RUnlock()
	return r.keys(prefix(r.p, k))
}

func (re *realEtcdClient) keys(p string) ([]string, error) {
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

func (r *realEtcdClient) Val(key string) (string, error) {
	r.RLock()
	defer r.RUnlock()
	return r.get(prefix(r.p, key))
}

func (r *realEtcdClient) get(key string) (string, error) {
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

func (f *fakeEtcdClient) SetPrefix(pre string) { f.p = pre }
func (f *fakeEtcdClient) Add(k, v string) (e error) {
	f.k[prefix(f.p, k)] = v
	return
}
func (f *fakeEtcdClient) Del(k string) error {
	ke := prefix(f.p, k)
	delete(f.k, ke)
	for key := range f.k {
		if strings.HasPrefix(key, ke) {
			delete(f.k, key)
		}
	}
	return nil
}
func (f *fakeEtcdClient) Keys(k string) ([]string, error) {
	key := prefix(f.p, k)
	r := []string{}
	for ke := range f.k {
		if key == filepath.Dir(ke) {
			r = append(r, filepath.Base(ke))
		}
	}
	return r, nil
}
func (f *fakeEtcdClient) get(k string) (v string, b bool) {
	v, b = f.k[k]
	return
}

func (f *fakeEtcdClient) Val(k string) (string, error) {
	key := prefix(f.p, k)
	v, ok := f.k[key]
	if !ok {
		return "", NewErr(nil, "%q not found", key)
	}
	return v, nil
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
