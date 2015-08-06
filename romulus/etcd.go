package romulus

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
)

var EtcdRetryLimit = 10 * time.Second

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
	fn := func() error {
		_, e := r.Set(k, v, 0)
		return e
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = EtcdRetryLimit
	return backoff.Retry(fn, b)
}

func (r *realEtcdClient) Del(k string) error {
	fn := func() error {
		_, e := r.Delete(k, true)
		if isKeyNotFound(e) {
			return nil
		}
		return e
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = EtcdRetryLimit
	return backoff.Retry(fn, b)
}

func (re *realEtcdClient) Keys(p string) ([]string, error) {
	var k []string
	var r *etcd.Response
	var er error
	fn := func() error {
		r, er = re.Get(p, true, false)
		if er != nil && isKeyNotFound(er) {
			return nil
		}
		return er
	}
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = EtcdRetryLimit
	if e := backoff.Retry(fn, b); e != nil || er != nil {
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
