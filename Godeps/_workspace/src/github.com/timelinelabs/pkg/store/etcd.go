package store

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

// NewEtcdStore constructs an EtcdStore using the given machine list
func NewEtcdStore(machines []string) *EtcdStore {
	client := etcd.NewClient(machines)
	return &EtcdStore{client, &sync.Mutex{}}
}

var _ Store = (*EtcdStore)(nil)

// EtcdStore implements the Store interface and can use directory like
// paths as keys.
type EtcdStore struct {
	// client is the Etcd client connection
	client *etcd.Client
	// mutex is used to synchronize client access
	mutex *sync.Mutex
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
	var etcdTTL = uint64(ttl.Seconds())
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, err := e.client.Set(key, string(value[:]), etcdTTL); err != nil {
		return err
	}

	return nil
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
	extractFunc func(*etcd.Node) io.Reader
}

func (r *etcdStoreReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.reader == nil {
		var res *etcd.Response
		if res, r.err = get(r.store, r.key, r.recurse); r.err != nil {
			return 0, r.err
		}
		r.reader = r.extractFunc(res.Node)
	}
	return r.reader.Read(p)
}

func get(store *EtcdStore, key string, recurse bool) (res *etcd.Response, err error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	res, err = store.client.Get(key, false, recurse)
	return
}

func extractKeyValues(node *etcd.Node, kv map[string]io.Reader) {
	if node.Dir {
		for _, childNode := range node.Nodes {
			extractKeyValues(childNode, kv)
		}
	} else {
		kv[node.Key] = strings.NewReader(node.Value)
	}
}

func extractValue(node *etcd.Node) io.Reader {
	if node.Dir {
		return strings.NewReader("")
	}
	return strings.NewReader(node.Value)
}

func extractValues(node *etcd.Node) io.Reader {
	readers := []io.Reader{}
	extractAllValues(node, &readers)
	if len(readers) > 0 {
		return io.MultiReader(readers...)
	}
	return strings.NewReader("")
}

func extractAllValues(node *etcd.Node, readers *[]io.Reader) {
	if node.Dir {
		for _, childNode := range node.Nodes {
			extractAllValues(childNode, readers)
		}
	} else {
		*readers = append(*readers, strings.NewReader(node.Value))
	}
}
