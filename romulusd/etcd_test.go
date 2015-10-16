package main

import "strings"

type fakeEtcdClient struct {
	k map[string]string
	p string
}

func newFakeEtcdClient(prefix string) *fakeEtcdClient {
	f := &fakeEtcdClient{map[string]string{"/": ""}, prefix}
	etcd = f
	return f
}

func (f *fakeEtcdClient) SetPrefix(pre string) { f.p = pre }
func (f *fakeEtcdClient) Add(k, v string) (e error) {
	if strings.HasSuffix(k, "frontend") {
		f.k[prefix(f.p, strings.TrimSuffix(k, "/frontend"))] = ""
	}
	if strings.HasSuffix(k, "backend") {
		f.k[prefix(f.p, strings.TrimSuffix(k, "/backend"))] = ""
	}
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
		if idx := strings.LastIndex(ke, "/"); idx > 0 && key == ke[:idx] {
			r = append(r, ke[idx+1:])
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
