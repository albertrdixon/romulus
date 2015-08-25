package romulus

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
)

var (
	etcdDebug = false
)

type EtcdClient interface {
	Add(key, val string) error
	Keys(pre string) ([]string, error)
	Del(key string) error
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

func DebugEtcd() {
	etcdDebug = true
}

func NewEtcdClient(peers []string, prefix string, timeout time.Duration) (EtcdClient, error) {
	if etcdDebug {
		client.EnablecURLDebug()
	}
	ec, er := client.New(client.Config{Endpoints: peers})
	if er != nil {
		return nil, er
	}
	return &realEtcdClient{client.NewKeysAPI(ec), new(sync.RWMutex),
		prefix, timeout}, nil
}

func NewFakeEtcdClient(prefix string) EtcdClient {
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
		k = append(k, strings.TrimLeft(strings.TrimPrefix(n.Key, p), "/"))
	}
	return k, nil
}

func (f *fakeEtcdClient) SetPrefix(pre string) { f.p = pre }
func (f fakeEtcdClient) Add(k, v string) (e error) {
	f.k[prefix(f.p, k)] = v
	return
}
func (f fakeEtcdClient) Del(k string) error {
	ke := prefix(f.p, k)
	delete(f.k, ke)
	for key := range f.k {
		if strings.HasPrefix(key, ke) {
			delete(f.k, key)
		}
	}
	return nil
}
func (f fakeEtcdClient) Keys(k string) ([]string, error) {
	key := prefix(f.p, k)
	r := []string{}
	for ke := range f.k {
		if key == filepath.Dir(ke) {
			r = append(r, filepath.Base(ke))
		}
	}
	return r, nil
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
	// s := k
	// if strings.HasPrefix(s, r.p) {
	// 	s = strings.TrimLeft(s, r.p)
	// }
	// s = strings.TrimLeft(s, "/")
	return strings.Join([]string{p, s}, "/")
}
