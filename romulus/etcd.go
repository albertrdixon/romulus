package romulus

import (
	"path/filepath"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
)

type EtcdClient interface {
	Add(key, val string) error
	Keys(pre string) ([]string, error)
	Del(key string) error
}

type realEtcdClient struct {
	*etcd.Client
}

type fakeEtcdClient map[string]string

func NewEtcdClient(peers []string) EtcdClient {
	return &realEtcdClient{etcd.NewClient(peers)}
}

func NewFakeEtcdClient() EtcdClient {
	return &fakeEtcdClient{"/": ""}
}

func (r *realEtcdClient) Add(k, v string) error {
	_, e := r.Set(k, v, 0)
	return e
}

func (r *realEtcdClient) Del(k string) error {
	_, e := r.Delete(k, true)
	return e
}

func (re *realEtcdClient) Keys(p string) ([]string, error) {
	var k []string
	r, e := re.Get(p, true, false)
	if e != nil {
		return k, e
	}

	k = make([]string, 0, len(r.Node.Nodes))
	for _, n := range r.Node.Nodes {
		k = append(k, strings.TrimLeft(strings.TrimPrefix(n.Key, p), "/"))
	}
	return k, nil
}

func (f fakeEtcdClient) Add(k, v string) (e error) { f[k] = v; return }
func (f fakeEtcdClient) Del(k string) error {
	delete(f, k)
	for key := range f {
		if strings.HasPrefix(key, k) {
			delete(f, key)
		}
	}
	return nil
}
func (f fakeEtcdClient) Keys(p string) ([]string, error) {
	r := []string{}
	for k := range f {
		if p == filepath.Dir(k) {
			r = append(r, filepath.Base(k))
		}
	}
	return r, nil
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*etcd.EtcdError)
	return ok && e.ErrorCode == etcdErr.EcodeKeyNotFound
}
