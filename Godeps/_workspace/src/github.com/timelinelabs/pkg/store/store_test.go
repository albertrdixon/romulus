package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Howdy struct {
	Yall int `json:"yall"`
}

func ExampleGetMulti() {
	b := NewMemStore()
	b.Set("/howdy/1", []byte(`{"yall": 1}`), 0)
	b.Set("/howdy/2", []byte(`{"yall": 2}`), 0)

	readers := b.GetMulti("/howdy")
	for {
		var h Howdy
		err := json.NewDecoder(readers).Decode(&h)
		if err != nil {
			break
		}
		fmt.Printf("%v\n", h.Yall)
	}
}

func TestMemBackend(t *testing.T) {
	s := NewMemStore()
	assert.Implements(t, (*Store)(nil), new(MemStore), "MemStore")
	testImplementation(t, s)
}

func testImplementation(t *testing.T, s Store) {
	testGet(t, s)
	testGetMulti(t, s)
	testGetMultiMap(t, s)
	testSet(t, s)
	testKeys(t, s)
	testEmptyKeys(t, s)
}

func testGet(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	key := "/howdy"
	value := "doody"
	store.Set(key, []byte(value), 0)
	reader := store.Get(key)
	actual := streamToString(reader)
	expected := value
	assert.Equal(t, expected, actual)
}

func testGetMulti(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	deleteKeys(store, "/doody")
	data := [3][2]string{{"/howdy/1", `{"yall": 1}`}, {"/howdy/2", `{"yall": 2}`}, {"/doody/1", `{"yall": 1}`}}

	for _, d := range data {
		key := d[0]
		value := d[1]
		store.Set(key, []byte(value), 0)
	}
	readers := store.GetMulti("/howdy")
	for i := 0; i < 2; i++ {
		var h Howdy
		err := json.NewDecoder(readers).Decode(&h)
		assert.NoError(t, err)
		assert.InDelta(t, h.Yall, 1, 1)
	}
	var h Howdy
	err := json.NewDecoder(readers).Decode(&h)
	assert.Error(t, err, "EOF")
}

func testGetMultiMap(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	deleteKeys(store, "/doody")
	data := [3][2]string{{"/howdy/1", `{"yall": 1}`}, {"/howdy/2", `{"yall": 2}`}, {"/doody/1", `{"yall": 1}`}}

	for _, d := range data {
		key := d[0]
		value := d[1]
		store.Set(key, []byte(value), 0)
	}
	res, err := store.GetMultiMap("/howdy")
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	val, ok := res["/howdy/1"]
	assert.True(t, ok)
	assert.Equal(t, streamToString(val), `{"yall": 1}`)

	val, ok = res["/howdy/2"]
	assert.True(t, ok)
	assert.Equal(t, streamToString(val), `{"yall": 2}`)
}

func testSet(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	value := "value"
	err := store.Set("/howdy", []byte(value), 0)
	assert.NoError(t, err)
	expected := value
	actual := streamToString(store.Get("/howdy"))
	assert.Equal(t, expected, actual)
}

func testEmptyKeys(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	actual, err := store.Keys("/howdy")
	assert.Error(t, err)
	assert.Empty(t, actual)
}

func testKeys(t *testing.T, store Store) {
	deleteKeys(store, "/howdy")
	deleteKeys(store, "/foo")
	store.Set("/howdy/doody", []byte("value"), 0)
	store.Set("/howdy/yall", []byte("value"), 0)
	store.Set("/foo", []byte("value"), 0)

	actual, err := store.Keys("/howdy")
	assert.NoError(t, err)
	var expected = []string{"/howdy/doody", "/howdy/yall"}
	assert.Len(t, expected, 2)
	for _, e := range expected {
		assert.Contains(t, actual, e)
	}
}

func deleteKeys(s Store, key string) {
	es, ok := s.(*EtcdStore)
	if ok {
		es.client.Delete(key, true)
	}
	ms, ok := s.(*MemStore)
	if ok {
		ms.store = make(map[string]string)
	}
}

func streamToString(stream io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.String()
}
