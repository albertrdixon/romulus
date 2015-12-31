package store

import (
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
)

// EnableEtcdDebug turns on cURL debug logging for the etcd client
func EnableEtcdDebug() {
	client.EnablecURLDebug()
}

// NewEtcdStore constructs an EtcdStore using the given machine list
func NewEtcdStore(machines []string, timeout time.Duration) (*EtcdStore, error) {
	etcd, er := client.New(client.Config{Endpoints: machines})
	if er != nil {
		return nil, er
	}
	return &EtcdStore{new(sync.Mutex), client.NewKeysAPI(etcd), timeout}, nil
}

var _ Store = (*EtcdStore)(nil)

// EtcdStore implements the Store interface and can use directory like
// paths as keys.
type EtcdStore struct {
	// mutex is used to synchronize client access
	*sync.Mutex
	// client is the Etcd client connection
	client.KeysAPI
	// timeout is the etcd client request timeout
	Timeout time.Duration
}

// Get returns an io.Reader for a single existing Etcd key.
// The key's value is not loaded until Read is called in the io.Reader
func (e *EtcdStore) Get(key string) io.Reader {
	return &etcdStoreReader{store: e, key: key, recurse: false, extractFunc: extractValue}
}

// GetMulti returns all Etcd values whose keys are prefixed with prefix.
// The returned io.Reader has all values concatenated as a single output
// using io.MultiReader.
// Conceptually, this will return all values for a "directory"-like key.
// Each io.Reader is lazy loaded at .Read() call.
func (e *EtcdStore) GetMulti(prefix string) io.Reader {
	return &etcdStoreReader{store: e, key: prefix, recurse: true, extractFunc: extractValues}
}

// GetMultiMap returns a map of all keys prefixed with prefix whose values are
// io.Readers.  Note that all key-value pairs are loaded and cached during this
// call.
func (e *EtcdStore) GetMultiMap(prefix string) (map[string]io.Reader, error) {
	kv := make(map[string]io.Reader)
	recursive := true
	res, err := get(e, prefix, recursive)
	if err != nil {
		return kv, err
	}
	extractKeyValues(res.Node, kv)
	return kv, nil
}

// Set will set a single Etcd key to value with a ttl.  If the ttl is zero then
// no ttl will be set.  Ttls in etcd are in seconds and must be at least 1
func (e *EtcdStore) Set(key string, value []byte, ttl time.Duration) error {
	e.Lock()
	defer e.Unlock()
	c, q := context.WithTimeout(context.Background(), e.Timeout)
	defer q()

	o := &client.SetOptions{TTL: ttl}
	if _, err := e.KeysAPI.Set(c, key, string(value[:]), o); err != nil {
		return err
	}

	return nil
}

// Delete removes the specified key. Set recurse to true to delete recursively
func (e *EtcdStore) Delete(key string, recurse bool) error {
	e.Lock()
	defer e.Unlock()
	c, q := context.WithTimeout(context.Background(), e.Timeout)
	defer q()

	_, er := e.KeysAPI.Delete(c, key, &client.DeleteOptions{Recursive: recurse})
	return er
}

// Keys returns all keys prefixed with string as a slice of strings.
// Internally, this will recursively get all Etcd keys from a directory.
func (e *EtcdStore) Keys(prefix string) ([]string, error) {
	kv, err := e.GetMultiMap(prefix)
	if err != nil {
		return []string{}, err
	}
	mk := make([]string, len(kv))
	i := 0
	for key := range kv {
		mk[i] = key
		i++
	}
	return mk, nil
}

type etcdStoreReader struct {
	store       *EtcdStore
	key         string
	err         error
	reader      io.Reader
	recurse     bool
	extractFunc func(*client.Node) io.Reader
}

func (r *etcdStoreReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.reader == nil {
		var res *client.Response
		if res, r.err = get(r.store, r.key, r.recurse); r.err != nil {
			return 0, r.err
		}
		r.reader = r.extractFunc(res.Node)
	}
	return r.reader.Read(p)
}

func get(store *EtcdStore, key string, recurse bool) (res *client.Response, err error) {
	store.Lock()
	defer store.Unlock()
	c, q := context.WithTimeout(context.Background(), store.Timeout)
	defer q()
	res, err = store.KeysAPI.Get(c, key,
		&client.GetOptions{Recursive: recurse, Quorum: true})
	return
}

func extractKeyValues(node *client.Node, kv map[string]io.Reader) {
	if node.Dir {
		for _, childNode := range node.Nodes {
			extractKeyValues(childNode, kv)
		}
	} else {
		kv[node.Key] = strings.NewReader(node.Value)
	}
}

func extractValue(node *client.Node) io.Reader {
	if node.Dir {
		return strings.NewReader("")
	}
	return strings.NewReader(node.Value)
}

func extractValues(node *client.Node) io.Reader {
	readers := []io.Reader{}
	extractAllValues(node, &readers)
	if len(readers) > 0 {
		return io.MultiReader(readers...)
	}
	return strings.NewReader("")
}

func extractAllValues(node *client.Node, readers *[]io.Reader) {
	if node.Dir {
		for _, childNode := range node.Nodes {
			extractAllValues(childNode, readers)
		}
	} else {
		*readers = append(*readers, strings.NewReader(node.Value))
	}
}
