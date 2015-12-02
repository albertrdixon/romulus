/*
Package store is a thread-safe key/value store facade.

The key syntax is assumed to be "path-like" hierarchy such as /howdy/doody.
The error strategy for the package is to pass all underlying backend errors
out as the error for the io.Reader.Read function.

The returned io.Readers' Read function will lazy load the backend's data.
*/
package store

import (
	"io"
	"time"
)

// Store is a key/value interface with "path-like" key hierarchies.
type Store interface {
	// Get returns an io.Reader for key
	Get(key string) io.Reader
	// GetMulti retrieves all keys with prefix.  The io.Reader returns all values
	// concatenated back to back.  See the Example in the tests to show how to
	// use this with JSON data.  Lazy loaded and if any errors occur they are
	// propagated through the io.Readers Read().
	GetMulti(prefix string) io.Reader
	// GetMultiMap retrieves all keys with prefix.  The return is a map of keys to
	// io.Reader values. Error is propagated from underlying backend.
	GetMultiMap(prefix string) (map[string]io.Reader, error)
	// Set will store a key-value pair into the store.  If the ttl value is 0,
	// then the value will never timeout. The returned error will be nil on
	// success or come from the underlying implementation.
	Set(key string, value []byte, ttl time.Duration) error
	// Keys returns all keys prefixed by prefix. error returned is specific to
	// underlying store implementation.
	Keys(prefix string) ([]string, error)
}
