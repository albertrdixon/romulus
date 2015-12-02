package store

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

var _ Store = (*MemStore)(nil)

// MemStore is an in-memory implementation of the Store interface using a map.
type MemStore struct {
	store map[string]string
	mutex *sync.RWMutex
}

// KeyNotFound is the error returned when the MemStore is not able to find
// the key.
type KeyNotFound struct {
	msg string
	Key string
}

// Error returns the key string that was not able to be found
func (e *KeyNotFound) Error() string { return e.msg }

// NewKeyNotFound constructs and formats KeyNotFound
func NewKeyNotFound(key string) error {
	return &KeyNotFound{fmt.Sprintf("Key %v not found", key), key}
}

// NewMemStore constructs a thread-safe in memory key-value store implemented
// with a map[string][string].
func NewMemStore() *MemStore {
	return &MemStore{map[string]string{}, &sync.RWMutex{}}
}

type memStoreReader struct {
	store  *MemStore
	key    string
	err    error
	reader io.Reader
}

func (r *memStoreReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.reader != nil {
		return r.reader.Read(p)
	}

	r.store.mutex.Lock()
	defer r.store.mutex.Unlock()
	v, ok := r.store.store[r.key]
	if !ok {
		r.err = NewKeyNotFound(r.key)
		return 0, r.err
	}
	r.reader = strings.NewReader(v)
	return r.reader.Read(p)
}

// Get returns an io.Reader associated with a single key in the store.
func (m *MemStore) Get(key string) io.Reader {
	return &memStoreReader{store: m, key: key}
}

// GetMulti returns an io.MultiReader of all keys prefixed by prefix.
func (m *MemStore) GetMulti(prefix string) io.Reader {
	readers := make([]io.Reader, len(m.store))
	i := 0
	for key := range m.store {
		if strings.HasPrefix(key, prefix) {
			readers[i] = m.Get(key)
			i++
		}
	}
	return io.MultiReader(readers[0:i]...)
}

// GetMultiMap returns a key-value map of all keys prefixed by prefix
func (m *MemStore) GetMultiMap(prefix string) (map[string]io.Reader, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	readers := make(map[string]io.Reader)
	for key := range m.store {
		if strings.HasPrefix(key, prefix) {
			readers[key] = m.Get(key)
		}
	}
	err := error(nil)
	if len(readers) == 0 {
		err = NewKeyNotFound(prefix)
	}
	return readers, err
}

// Set will add an entry to the internal map and set a time-to-live using a
// time.Timer if the duration is greater than 0.
func (m *MemStore) Set(key string, value []byte, ttl time.Duration) error {
	m.mutex.Lock()
	m.store[key] = string(value[:])
	m.mutex.Unlock()
	if ttl > 0 {
		timer := time.NewTimer(ttl)
		go func(key string) {
			<-timer.C
			m.mutex.Lock()
			delete(m.store, key)
			m.mutex.Unlock()
		}(key)
	}

	return nil
}

// Keys returns all keys in the internal map that are prefixed with prefix
func (m *MemStore) Keys(prefix string) ([]string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	mk := make([]string, len(m.store))
	i := 0
	for key := range m.store {
		if strings.HasPrefix(key, prefix) {
			mk[i] = key
			i++
		}
	}
	if i == 0 {
		return []string{}, NewKeyNotFound(prefix)
	}
	return mk[0:i], nil
}
